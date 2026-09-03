package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CreateAsaasWalletRequest struct {
	Name        string  `json:"name"`
	CpfCnpj     string  `json:"cpf_cnpj"`
	Email       string  `json:"email"`
	Phone       string  `json:"phone"`
	PersonType  string  `json:"person_type"` // "JURIDICA" or "FISICA"
	IncomeValue float64 `json:"income_value,omitempty"`
}

// CreateAsaasWallet creates an Asaas sub-account (wallet).
// SECURITY: Admin-only — this operation uses the platform's Asaas API key
// and creates provider-side resources. Ordinary authenticated users must NOT
// be able to invoke this endpoint.
func CreateAsaasWallet(c *fiber.Ctx) error {
	// Verify admin role
	role, err := middlewares.GetUserRoleFromToken(c)
	if err != nil || role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	var req CreateAsaasWalletRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Name == "" || req.CpfCnpj == "" || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name, cpf_cnpj, and email are required"})
	}

	if req.PersonType == "" {
		req.PersonType = "JURIDICA"
	}

	client := services.NewAsaasClient()

	walletResp, err := client.CreateSubAccount(services.AsaasWalletRequest{
		Name:        req.Name,
		CpfCnpj:     req.CpfCnpj,
		Email:       req.Email,
		Phone:       req.Phone,
		PersonType:  req.PersonType,
		IncomeValue: req.IncomeValue,
	})
	if err != nil {
		log.Printf("[ASAAS] Failed to create sub-account for %s: %v", req.Email, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create Asaas wallet: " + err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":   "Asaas wallet created successfully",
		"wallet_id": walletResp.ID,
		"name":      walletResp.Name,
		"status":    walletResp.Status,
	})
}

func GetAsaasWalletStatus(c *fiber.Ctx) error {
	walletID := c.Params("walletId")
	if walletID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Wallet ID is required"})
	}

	client := services.NewAsaasClient()

	walletResp, err := client.GetSubAccount(walletID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get wallet status"})
	}

	return c.JSON(fiber.Map{
		"wallet_id": walletResp.ID,
		"name":      walletResp.Name,
		"status":    walletResp.Status,
	})
}

// OrderDocument represents the order structure from orders_api.
// We need this to validate orders and retrieve their details.
type OrderDocument struct {
	ID              int64  `gorm:"primaryKey"`
	LegacyID        string `gorm:"column:legacy_id"`
	EstablishmentID int64  `gorm:"column:establishment_id"`
	UserPhone       string `gorm:"column:user_phone"`
	Status          string
	Payload         []byte `gorm:"type:jsonb"`
}

func (OrderDocument) TableName() string { return "order_documents" }

// OrderPayload represents the nested structure within the order's JSONB payload.
type OrderPayload struct {
	OrderTotal      float64 `json:"order_total"`
	DeliveryValue   float64 `json:"deliveryValue"`
	EstablishmentId int64   `json:"establishmentId"`
	User            struct {
		Phone string `json:"phone"`
	} `json:"user"`
	DeliveryMan struct {
		Id int64 `json:"id"`
	} `json:"deliveryman"`
}

// CreateAsaasSplitPayment creates an Asaas PIX payment with split rules.
// SECURITY: This endpoint must validate the order, authorize the caller,
// derive amounts from the server-side order record, verify wallet ownership,
// and enforce the platform fee.
func CreateAsaasSplitPayment(c *fiber.Ctx) error {
	// Extract authenticated user information
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	role, _ := middlewares.GetUserRoleFromToken(c)

	var req struct {
		OrderID string `json:"order_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.OrderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "order_id is required"})
	}

	// Step 1: Retrieve and validate the order from the database
	var orderDoc OrderDocument
	if err := authModels.DB.Where("legacy_id = ?", req.OrderID).First(&orderDoc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
		}
		log.Printf("[ASAAS] Failed to retrieve order %s: %v", req.OrderID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve order"})
	}

	// Step 2: Parse the order payload to get amounts and participants
	var payload OrderPayload
	if err := json.Unmarshal(orderDoc.Payload, &payload); err != nil {
		log.Printf("[ASAAS] Failed to parse order payload for %s: %v", req.OrderID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse order data"})
	}

	// Step 3: Authorize the caller - only the customer, establishment owner, or admin can create payment
	authorized := false
	if role == "admin" {
		authorized = true
	} else {
		// Check if caller is the customer
		if payload.User.Phone != "" {
			var customer authModels.Client
			if err := authModels.DB.Where("phone = ?", payload.User.Phone).First(&customer).Error; err == nil {
				if int64(customer.ID) == tokenUserID {
					authorized = true
				}
			}
		}

		// Check if caller is the establishment owner
		if !authorized && payload.EstablishmentId > 0 {
			var establishment authModels.Establishment
			if err := authModels.DB.First(&establishment, payload.EstablishmentId).Error; err == nil {
				var user authModels.User
				if err := authModels.DB.Where("establishment_id = ? AND id = ?", establishment.ID, tokenUserID).First(&user).Error; err == nil {
					authorized = true
				}
			}
		}
	}

	if !authorized {
		log.Printf("[ASAAS] Unauthorized payment creation attempt for order %s by user %d", req.OrderID, tokenUserID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Not authorized to create payment for this order"})
	}

	// Step 4: Validate amounts from the order (server-side source of truth)
	if payload.OrderTotal <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid order total"})
	}

	// Step 5: Retrieve and validate establishment wallet
	var establishment authModels.Establishment
	if err := authModels.DB.First(&establishment, payload.EstablishmentId).Error; err != nil {
		log.Printf("[ASAAS] Establishment %d not found for order %s", payload.EstablishmentId, req.OrderID)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Establishment not found"})
	}

	if establishment.PaymentWalletID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Establishment does not have an Asaas wallet configured"})
	}

	// Step 6: Retrieve and validate delivery person wallet (if applicable)
	var deliveryWalletID string
	if payload.DeliveryMan.Id > 0 && payload.DeliveryValue > 0 {
		var deliveryMan authModels.DeliveryMan
		if err := authModels.DB.First(&deliveryMan, payload.DeliveryMan.Id).Error; err != nil {
			log.Printf("[ASAAS] Delivery person %d not found for order %s", payload.DeliveryMan.Id, req.OrderID)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Delivery person not found"})
		}
		deliveryWalletID = deliveryMan.PaymentWalletID
	}

	// Step 7: Calculate split percentages with enforced platform fee
	// Platform keeps 5%, establishment gets their share, delivery gets their share
	platformPct := 5.0
	deliveryPct := 0.0
	if payload.DeliveryValue > 0 && deliveryWalletID != "" {
		deliveryPct = (payload.DeliveryValue / payload.OrderTotal) * 100
	}
	
	// Establishment gets the remainder after platform and delivery fees
	establishmentPct := 100.0 - platformPct - deliveryPct

	if establishmentPct < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid split calculation: fees exceed order total"})
	}

	// Step 8: Build split rules with validated wallets and calculated percentages
	var splits []services.AsaasSplitRequest
	
	// Add establishment split
	splits = append(splits, services.AsaasSplitRequest{
		SubMerchantWalletId: establishment.PaymentWalletID,
		Percentual:          establishmentPct,
	})

	// Add delivery split if applicable
	if deliveryPct > 0 && deliveryWalletID != "" {
		splits = append(splits, services.AsaasSplitRequest{
			SubMerchantWalletId: deliveryWalletID,
			Percentual:          deliveryPct,
		})
	}

	// Platform fee is implicit (100% - sum of splits = platformPct)
	// Asaas automatically keeps the remainder for the platform account

	// Step 9: Create the payment request
	client := services.NewAsaasClient()
	dueDate := time.Now().Format("2006-01-02")

	paymentReq := services.AsaasPaymentRequest{
		BillingType:       "PIX",
		DueDate:           dueDate,
		ExternalReference: req.OrderID,
		Value:             payload.OrderTotal,
		Description:       fmt.Sprintf("Pedido %s", req.OrderID),
		Split:             splits,
	}

	paymentResp, err := client.CreatePayment(paymentReq)
	if err != nil {
		log.Printf("[ASAAS] Failed to create split payment for order %s: %v", req.OrderID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create Asaas payment: " + err.Error()})
	}

	// Step 10: Record the payment in the database for audit trail
	payment := models.Payment{
		OrderID:         req.OrderID,
		EstablishmentID: payload.EstablishmentId,
		Amount:          payload.OrderTotal,
		DeliveryAmount:  payload.DeliveryValue,
		Method:          "pix",
		Status:          "pending",
		PixCopyPaste:    paymentResp.PixTransaction.Payload,
		PixQRCode:       paymentResp.PixTransaction.QRCode,
	}

	if err := models.DB.Create(&payment).Error; err != nil {
		log.Printf("[ASAAS] Warning: Failed to record payment for order %s: %v", req.OrderID, err)
		// Don't fail the request - payment was created successfully in Asaas
	}

	log.Printf("[ASAAS] Split payment created for order %s: payment_id=%s, amount=%.2f, platform=%.1f%%, establishment=%.1f%%, delivery=%.1f%%",
		req.OrderID, paymentResp.ID, payload.OrderTotal, platformPct, establishmentPct, deliveryPct)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"payment_id":  paymentResp.ID,
		"status":      paymentResp.Status,
		"pix_payload": paymentResp.PixTransaction.Payload,
		"pix_qr_code": paymentResp.PixTransaction.QRCode,
	})
}
