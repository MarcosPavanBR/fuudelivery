package handlers

import (
	"log"
	"time"

	"github.com/carloshomar/vercardapio/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
)

type CreateAsaasWalletRequest struct {
	Name        string  `json:"name"`
	CpfCnpj     string  `json:"cpf_cnpj"`
	Email       string  `json:"email"`
	Phone       string  `json:"phone"`
	PersonType  string  `json:"person_type"` // "JURIDICA" or "FISICA"
	IncomeValue float64 `json:"income_value,omitempty"`
}

func CreateAsaasWallet(c *fiber.Ctx) error {
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
		"message": "Asaas wallet created successfully",
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

func CreateAsaasSplitPayment(c *fiber.Ctx) error {
	var req struct {
		CustomerName   string                   `json:"customer_name"`
		CustomerEmail  string                   `json:"customer_email"`
		CustomerPhone  string                   `json:"customer_phone"`
		Amount         float64                  `json:"amount"`
		EstablishmentWalletID string           `json:"establishment_wallet_id"`
		DeliveryManWalletID   string           `json:"deliveryman_wallet_id"`
		EstablishmentSplitPct float64          `json:"establishment_split_pct"`
		DeliveryAmount        float64          `json:"delivery_amount"`
		OrderID               string           `json:"order_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Amount must be positive"})
	}

	if req.EstablishmentWalletID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "establishment_wallet_id is required"})
	}

	client := services.NewAsaasClient()

	dueDate := time.Now().Format("2006-01-02")

	paymentReq := services.AsaasPaymentRequest{
		BillingType:       "PIX",
		DueDate:           dueDate,
		ExternalReference: req.OrderID,
		Value:             req.Amount,
		Description:       "Pedido " + req.OrderID,
	}

	// Platform keeps 5%
	platformPct := 5.0

	// Build split rules
	var splits []services.AsaasSplitRequest
	splits = append(splits, services.AsaasSplitRequest{
		SubMerchantWalletId: req.EstablishmentWalletID,
		Percentual:          req.EstablishmentSplitPct,
	})

	if req.DeliveryManWalletID != "" && req.DeliveryAmount > 0 {
		deliveryPct := (req.DeliveryAmount / req.Amount) * 100
		splits = append(splits, services.AsaasSplitRequest{
			SubMerchantWalletId: req.DeliveryManWalletID,
			Percentual:          deliveryPct,
		})
	}

	_ = platformPct // platform fee is implicit (100% - sum of splits)

	paymentReq.Split = splits

	paymentResp, err := client.CreatePayment(paymentReq)
	if err != nil {
		log.Printf("[ASAAS] Failed to create split payment for order %s: %v", req.OrderID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create Asaas payment: " + err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"payment_id": paymentResp.ID,
		"status":     paymentResp.Status,
		"pix_payload": paymentResp.PixTransaction.Payload,
		"pix_qr_code": paymentResp.PixTransaction.QRCode,
	})
}
