package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GeneratePIX(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	email := req.CustomerEmail
	if email == "" {
		email = "cliente@email.com"
	}

	name := req.CustomerName
	if name == "" {
		name = "Cliente"
	}

	phone := req.CustomerPhone
	if phone == "" {
		phone = ""
	}

	description := fmt.Sprintf("Pedido %s", req.OrderID)

	client := services.NewAbacatePayClient()
	chargeReq := services.PIXChargeRequest{
		Data: struct {
			Amount      int64  `json:"amount"`
			Description string `json:"description,omitempty"`
			ExternalID  string `json:"externalId,omitempty"`
			Customer    struct {
				Name      string `json:"name,omitempty"`
				TaxID     string `json:"taxId,omitempty"`
				Email     string `json:"email,omitempty"`
				Cellphone string `json:"cellphone,omitempty"`
			} `json:"customer,omitempty"`
		}{Amount: int64(req.Amount), Description: description, ExternalID: req.OrderID},
	}
	chargeReq.Data.Customer.Name = name
	chargeReq.Data.Customer.Email = email
	chargeReq.Data.Customer.Cellphone = phone

	apiResp, err := client.CreatePIXCharge(chargeReq)
	if err != nil {
		log.Printf("Error creating PIX payment via AbacatePay: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create PIX payment"})
	}

	payment := models.Payment{
		ID:              primitive.NewObjectID(),
		OrderID:         req.OrderID,
		CustomerID:      req.CustomerID,
		CustomerPhone:   req.CustomerPhone,
		EstablishmentID: req.EstablishmentID,
		Amount:          req.Amount,
		DeliveryAmount:  req.DeliveryAmount,
		Method:          "pix",
		Status:          "PENDING",
		PixQRCode:       apiResp.QRCode,
		PixCopyPaste:    apiResp.CopyPaste,
		QRCodeBase64:    apiResp.QRCodeBase64,
		AbacatePayID:    apiResp.ID,
		CreatedAt:       time.Now(),
	}

	_, err = models.MongoDabase.Collection("payments").InsertOne(mongoCtx(), payment)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
	}

	response := dto.PaymentResponse{
		PaymentID:    payment.ID.Hex(),
		Status:       "PENDING",
		PixQRCode:    apiResp.QRCode,
		PixCopyPaste: apiResp.CopyPaste,
		QRCodeBase64: apiResp.QRCodeBase64,
		AbacatePayID: apiResp.ID,
		Message:      "PIX payment created via AbacatePay",
	}

	return c.Status(201).JSON(response)
}
