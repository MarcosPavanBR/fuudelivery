package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
)

// ChargeCard cobra diretamente um cartão tokenizado no gateway (AbacatePay).
// Não persiste pagamento — é uma cobrança avulsa (o fluxo de pedidos usa
// ProcessPayment, que grava em Postgres com dual-write).
func ChargeCard(c *fiber.Ctx) error {
	var req struct {
		CardToken    string  `json:"card_token"`
		Amount       float64 `json:"amount"`
		OrderID      string  `json:"order_id"`
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

	if req.OrderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id is required"})
	}

	serverTotal, ok := validateChargeAmount(req.OrderID, req.Amount)
	if !ok {
		log.Printf("[CARD] Cobrança rejeitada: valor diverge do pedido %s (client=%.2f)", req.OrderID, req.Amount)
		return c.Status(400).JSON(fiber.Map{"error": "Valor da cobrança não corresponde ao pedido"})
	}
	req.Amount = serverTotal

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
		log.Printf("[CARD] Error creating card payment via AbacatePay: amount=%.2f", req.Amount)
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

// ProcessPayment cria a cobrança (cartão ou PIX) e persiste o pagamento em
// Postgres (corte 4 — fonte da verdade), com dual-write best-effort no Mongo.
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

	// O valor cobrado é o total recalculado no servidor na criação do pedido —
	// nunca o amount enviado pelo cliente.
	serverTotal, ok := validateChargeAmount(req.OrderID, req.Amount)
	if !ok {
		log.Printf("[CARD] Cobrança rejeitada: valor diverge do pedido %s (client=%.2f)", req.OrderID, req.Amount)
		return c.Status(400).JSON(fiber.Map{"error": "Valor da cobrança não corresponde ao pedido"})
	}
	req.Amount = serverTotal

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

		// ID é BIGSERIAL no Postgres — preenchido automaticamente pelo Create.
		// CardToken NÃO é mais persistido: PAN/CVV não podem transitar pela
		// nossa API (PCI). O campo fica no modelo apenas para histórico.
		payment := models.Payment{
			OrderID:         req.OrderID,
			CustomerID:      req.CustomerID,
			CustomerPhone:   req.CustomerPhone,
			EstablishmentID: req.EstablishmentID,
			Amount:          req.Amount,
			DeliveryAmount:  req.DeliveryAmount,
			Method:          req.Method,
			Status:          paymentStatus,
			Installments:    installments,
			CardLastDigits:  apiResp.LastDigits,
			AbacatePayID:    apiResp.ID,
			CreatedAt:       time.Now(),
			ConfirmedAt:     confirmedAt,
		}

		if err := models.DB.Create(&payment).Error; err != nil {
			log.Printf("[CARD] Erro ao salvar pagamento no Postgres: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		response := dto.PaymentResponse{
			PaymentID:    payment.IDString(),
			Status:       paymentStatus,
			AbacatePayID: apiResp.ID,
			Message:      "Payment processed via AbacatePay",
		}

		return c.Status(201).JSON(response)
	}

	if req.Method == "pix" {
		pixReq := services.PIXChargeRequest{}
		// toCents com arredondamento: int64(req.Amount) truncava
		// R$99,99 → 9999*100 falhando em 99.99*100=9998.999…
		pixReq.Data.Amount = toCents(req.Amount)
		pixReq.Data.Description = description
		pixReq.Data.ExternalID = req.OrderID
		// customer é opcional para PIX; sem CPF válido o gateway responde 422.

		apiResp, err := client.CreatePIXCharge(pixReq)
		if err != nil {
			log.Printf("Error processing PIX payment via AbacatePay: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "PIX payment failed"})
		}

		payment := models.Payment{
			OrderID:         req.OrderID,
			CustomerID:      req.CustomerID,
			CustomerPhone:   req.CustomerPhone,
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

		if err := models.DB.Create(&payment).Error; err != nil {
			log.Printf("[PIX] Erro ao salvar pagamento no Postgres: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		response := dto.PaymentResponse{
			PaymentID:    payment.IDString(),
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
