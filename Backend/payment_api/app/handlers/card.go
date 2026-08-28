package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/gofiber/fiber/v2"
)

// getPaymentRouter extrai o router de pagamento do contexto Fiber.
func getPaymentRouter(c *fiber.Ctx) (*gateway.Router, error) {
	router, ok := c.Locals("payment_router").(*gateway.Router)
	if !ok || router == nil {
		return nil, fmt.Errorf("payment router not available")
	}
	return router, nil
}

// ChargeCard cobra um cartão usando o PaymentRouter (fallback chain + circuit breaker).
// Não persiste pagamento — é uma cobrança avulsa.
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

	router, err := getPaymentRouter(c)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Payment router unavailable"})
	}

	gatewayReq := &gateway.TransactionRequest{
		OrderID:         0, // ChargeCard é avulsa, sem pedido interno
		Amount:          int64(req.Amount * 100),
		Currency:        "BRL",
		PaymentMethod:   gateway.MethodCreditCard,
		CustomerEmail:   email,
		CustomerName:    name,
		CustomerDoc:     req.CPF,
		CustomerPhone:   req.Phone,
		CardData: &gateway.CardData{
			Token:        req.CardToken,
			Installments: installments,
			HolderName:   name,
			HolderDoc:    req.CPF,
		},
		Capture: true,
	}

	resp, err := router.CreateTransactionWithFallback(c.Context(), gatewayReq)
	if err != nil {
		log.Printf("[CARD] Error creating card payment via router: amount=%.2f err=%v", req.Amount, err)
		return c.Status(500).JSON(fiber.Map{"error": "Card payment failed"})
	}

	return c.Status(200).JSON(fiber.Map{
		"charge_id":    resp.GatewayID,
		"status":       resp.Status,
		"installments": installments,
		"last_digits":  resp.CardLastDigits,
		"gateway":      resp.Gateway,
		"message":      "Card payment processed via PaymentRouter",
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

	serverTotal, ok := validateChargeAmount(req.OrderID, req.Amount)
	if !ok {
		log.Printf("[CARD] Cobrança rejeitada: valor diverge do pedido %s (client=%.2f)", req.OrderID, req.Amount)
		return c.Status(400).JSON(fiber.Map{"error": "Valor da cobrança não corresponde ao pedido"})
	}
	req.Amount = serverTotal

	router, err := getPaymentRouter(c)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Payment router unavailable"})
	}

	if req.Method == "credit" || req.Method == "debit" {
		installments := req.Installments
		if installments <= 0 {
			installments = 1
		}

		gatewayReq := &gateway.TransactionRequest{
			OrderID:        req.OrderID,
			Amount:         int64(req.Amount * 100),
			Currency:       "BRL",
			PaymentMethod:  gateway.MethodCreditCard,
			CustomerEmail:  email,
			CustomerName:   req.CustomerName,
			CustomerDoc:    req.CustomerDoc,
			CustomerPhone:  req.CustomerPhone,
			CardData: &gateway.CardData{
				Token:        req.CardToken,
				Installments: installments,
				HolderName:   req.CustomerName,
				HolderDoc:    req.CustomerDoc,
			},
			Capture: true,
		}

		resp, err := router.CreateTransactionWithFallback(c.Context(), gatewayReq)
		if err != nil {
			log.Printf("Error processing card payment via router: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Payment processing failed"})
		}

		paymentStatus := "PENDING"
		now := time.Now()
		var confirmedAt *time.Time
		if resp.Status == "paid" || resp.Status == "authorized" {
			paymentStatus = "CONFIRMED"
			confirmedAt = &now
		} else if resp.Status == "failed" || resp.Status == "canceled" {
			paymentStatus = "REFUSED"
		}

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
			CardLastDigits:  resp.CardLastDigits,
			GatewayID:       resp.GatewayID,
			Gateway:         resp.Gateway,
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
			GatewayID:    resp.GatewayID,
			Gateway:      resp.Gateway,
			Message:      fmt.Sprintf("Payment processed via %s", resp.Gateway),
		}

		return c.Status(201).JSON(response)
	}

	if req.Method == "pix" {
		gatewayReq := &gateway.TransactionRequest{
			OrderID:       req.OrderID,
			Amount:        int64(req.Amount * 100),
			Currency:      "BRL",
			PaymentMethod: gateway.MethodPIX,
			CustomerEmail: email,
			CustomerName:  req.CustomerName,
			CustomerDoc:   req.CustomerDoc,
			CustomerPhone: req.CustomerPhone,
			Capture:       true,
		}

		resp, err := router.CreateTransactionWithFallback(c.Context(), gatewayReq)
		if err != nil {
			log.Printf("Error processing PIX payment via router: %v", err)
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
			PixCopyPaste:    resp.PIXCopyPaste,
			QRCodeBase64:    resp.PIXQRCode,
			PixQRCode:       resp.PIXQRCode,
			GatewayID:       resp.GatewayID,
			Gateway:         resp.Gateway,
			CreatedAt:       time.Now(),
		}

		if err := models.DB.Create(&payment).Error; err != nil {
			log.Printf("[PIX] Erro ao salvar pagamento no Postgres: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		response := dto.PaymentResponse{
			PaymentID:    payment.IDString(),
			Status:       "PENDING",
			PixCopyPaste: resp.PIXCopyPaste,
			QRCodeBase64: resp.PIXQRCode,
			PixQRCode:    resp.PIXQRCode,
			GatewayID:    resp.GatewayID,
			Gateway:      resp.Gateway,
			Message:      fmt.Sprintf("PIX payment created via %s", resp.Gateway),
		}

		return c.Status(201).JSON(response)
	}

	return c.Status(400).JSON(fiber.Map{"error": "Invalid payment method"})
}
