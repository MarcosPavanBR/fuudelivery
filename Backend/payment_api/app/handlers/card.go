package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/vercardapio/payment_api/app/dto"
	"github.com/carloshomar/vercardapio/payment_api/app/models"
	"github.com/carloshomar/vercardapio/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)



func ChargeCard(c *fiber.Ctx) error {
	var req struct {
		CardToken    string  `json:"card_token"`
		Amount       float64 `json:"amount"`
		Installments int     `json:"installments"`
		Email        string  `json:"email"`
		Name         string  `json:"name"`
		Phone        string  `json:"phone"`
		CPF          string  `json:"cpf"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.CardToken == "" {
		return c.Status(400).JSON(fiber.Map{"error": "card_token is required"})
	}

	email := req.Email
	if email == "" {
		email = "cliente@email.com"
	}

	name := req.Name
	if name == "" {
		name = "Cliente"
	}

	installments := req.Installments
	if installments <= 0 {
		installments = 1
	}

	client := services.NewAbacatePayClient()
	chargeReq := services.CardChargeRequest{
		Amount:       req.Amount,
		Description:  "Pagamento cartao",
		Installments: installments,
		CardToken:    req.CardToken,
	}
	chargeReq.Customer.Name = name
	chargeReq.Customer.Email = email
	chargeReq.Customer.Phone = req.Phone
	chargeReq.Customer.CPF = req.CPF

	apiResp, err := client.CreateCardCharge(chargeReq)
	if err != nil {
		log.Printf("Error creating card payment via AbacatePay: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Card payment failed"})
	}

	return c.Status(200).JSON(fiber.Map{
		"charge_id":    apiResp.ID,
		"status":       apiResp.Status,
		"installments": apiResp.Installments,
		"last_digits":  apiResp.LastDigits,
		"message":      "Card payment processed via AbacatePay",
	})
}

func ProcessPayment(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	description := fmt.Sprintf("Pedido %s", req.OrderID)
	email := req.CustomerEmail
	if email == "" {
		email = "cliente@email.com"
	}

	client := services.NewAbacatePayClient()

	if req.Method == "credit" || req.Method == "debit" {
		installments := req.Installments
		if installments <= 0 {
			installments = 1
		}

		cardReq := services.CardChargeRequest{
			Amount:       req.Amount,
			Description:  description,
			Installments: installments,
			CardToken:    req.CardToken,
		}
		cardReq.Customer.Name = req.CustomerName
		cardReq.Customer.Email = email
		cardReq.Customer.Phone = req.CustomerPhone
		cardReq.Customer.CPF = ""

		apiResp, err := client.CreateCardCharge(cardReq)
		if err != nil {
			log.Printf("Error processing card payment via AbacatePay: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Payment processing failed"})
		}

		paymentStatus := "PENDING"
		now := time.Now()
		var confirmedAt *time.Time
		if apiResp.Status == "paid" {
			paymentStatus = "CONFIRMED"
			confirmedAt = &now
		} else if apiResp.Status == "refused" {
			paymentStatus = "REFUSED"
		}

		payment := models.Payment{
			ID:              primitive.NewObjectID(),
			OrderID:         req.OrderID,
			CustomerID:      req.CustomerID,
			EstablishmentID: req.EstablishmentID,
			Amount:          req.Amount,
			DeliveryAmount:  req.DeliveryAmount,
			Method:          req.Method,
			Status:          paymentStatus,
			CardToken:       req.CardToken,
			Installments:    installments,
			CardLastDigits:  apiResp.LastDigits,
			AbacatePayID:    apiResp.ID,
			CreatedAt:       time.Now(),
			ConfirmedAt:     confirmedAt,
		}

		_, err = models.MongoDabase.Collection("payments").InsertOne(mongoCtx(), payment)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		response := dto.PaymentResponse{
			PaymentID:    payment.ID.Hex(),
			Status:       paymentStatus,
			AbacatePayID: apiResp.ID,
			Message:      "Payment processed via AbacatePay",
		}

		return c.Status(201).JSON(response)
	}

	if req.Method == "pix" {
		pixReq := services.PIXChargeRequest{
			Amount:      req.Amount,
			Description: description,
		}
		pixReq.Customer.Name = req.CustomerName
		pixReq.Customer.Email = email
		pixReq.Customer.Phone = req.CustomerPhone

		apiResp, err := client.CreatePIXCharge(pixReq)
		if err != nil {
			log.Printf("Error processing PIX payment via AbacatePay: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "PIX payment failed"})
		}

		payment := models.Payment{
			ID:              primitive.NewObjectID(),
			OrderID:         req.OrderID,
			CustomerID:      req.CustomerID,
			EstablishmentID: req.EstablishmentID,
			Amount:          req.Amount,
			DeliveryAmount:  req.DeliveryAmount,
			Method:          "pix",
			Status:          "PENDING",
			PixCopyPaste:    apiResp.CopyPaste,
			QRCodeBase64:    apiResp.QRCodeBase64,
			PixQRCode:       apiResp.QRCode,
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
			PixCopyPaste: apiResp.CopyPaste,
			QRCodeBase64: apiResp.QRCodeBase64,
			PixQRCode:    apiResp.QRCode,
			AbacatePayID: apiResp.ID,
			Message:      "PIX payment created via AbacatePay",
		}

		return c.Status(201).JSON(response)
	}

	return c.Status(400).JSON(fiber.Map{"error": "Invalid payment method"})
}
