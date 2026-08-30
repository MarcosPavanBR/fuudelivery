package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	
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
//
// Autorização: valida que o usuário autenticado (JWT) é o dono do pedido
// antes de criar a cobrança. Usa o total do pedido de order_documents.
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

	// Valida que o usuário autenticado é o dono do pedido e carrega os dados
	// autorizativos (establishment_id, customer_id, order_total, etc.).
	orderData, err := authorizeAndLoadOrder(c, req.OrderID)
	if err != nil {
		if err == ErrUnauthorizedPayment {
			log.Printf("[CARD] Authorization failed for order %s: user does not own order", req.OrderID)
			return c.Status(403).JSON(fiber.Map{"error": "Acesso negado: você não é o dono deste pedido"})
		}
		if err == ErrOrderNotFound {
			log.Printf("[CARD] Order %s not found", req.OrderID)
			return c.Status(404).JSON(fiber.Map{"error": "Pedido não encontrado"})
		}
		if err == ErrOrderAlreadyPaid {
			log.Printf("[CARD] Order %s already has a confirmed payment", req.OrderID)
			return c.Status(409).JSON(fiber.Map{"error": "Este pedido já foi pago"})
		}
		log.Printf("[CARD] Failed to authorize order %s: %v", req.OrderID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao validar pedido"})
	}

	// Usa o total do pedido extraído de order_documents (fonte da verdade).
	// Ignora completamente o amount enviado pelo cliente.
	amount := orderData.OrderTotal

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
		Amount:          int64(amount * 100),
		Currency:        "BRL",
		PaymentMethod:   gateway.MethodCreditCard,
		CustomerEmail:   email,
		CustomerName:    name,
		CustomerDoc:     req.CPF,
		CustomerPhone:   orderData.CustomerPhone,
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
		log.Printf("[CARD] Error creating card payment via router: amount=%.2f err=%v", amount, err)
		return c.Status(500).JSON(fiber.Map{"error": "Card payment failed"})
	}

	log.Printf("[CARD] ChargeCard for order %s: customer=%d establishment=%d amount=%.2f",
		req.OrderID, orderData.CustomerID, orderData.EstablishmentID, amount)

	return c.Status(200).JSON(fiber.Map{
		"charge_id":    resp.GatewayID,
		"status":       resp.Status,
		"installments": installments,
		"last_digits":  resp.CardLast4,
		"gateway":      resp.Gateway,
		"message":      "Card payment processed via PaymentRouter",
	})
}

// ProcessPayment cria a cobrança (cartão ou PIX) e persiste o pagamento em
// Postgres (corte 4 — fonte da verdade), com dual-write best-effort no Mongo.
//
// Autorização: valida que o usuário autenticado (JWT) é o dono do pedido
// antes de criar a cobrança. Os IDs de cliente/estabelecimento são extraídos
// de order_documents (fonte da verdade) — nunca confia no request body.
func ProcessPayment(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Valida que o usuário autenticado é o dono do pedido e carrega os dados
	// autorizativos (establishment_id, customer_id, order_total, etc.).
	orderData, err := authorizeAndLoadOrder(c, req.OrderID)
	if err != nil {
		if err == ErrUnauthorizedPayment {
			log.Printf("[CARD] Authorization failed for order %s: user does not own order", req.OrderID)
			return c.Status(403).JSON(fiber.Map{"error": "Acesso negado: você não é o dono deste pedido"})
		}
		if err == ErrOrderNotFound {
			log.Printf("[CARD] Order %s not found", req.OrderID)
			return c.Status(404).JSON(fiber.Map{"error": "Pedido não encontrado"})
		}
		if err == ErrOrderAlreadyPaid {
			log.Printf("[CARD] Order %s already has a confirmed payment", req.OrderID)
			return c.Status(409).JSON(fiber.Map{"error": "Este pedido já foi pago"})
		}
		log.Printf("[CARD] Failed to authorize order %s: %v", req.OrderID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao validar pedido"})
	}

	email := req.CustomerEmail
	if email == "" {
		email = "cliente@email.com"
	}

	// Usa o total do pedido extraído de order_documents (fonte da verdade).
	// Ignora completamente o amount enviado pelo cliente.
	amount := orderData.OrderTotal

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
			OrderID:        0,
			Amount:         int64(amount * 100),
			Currency:       "BRL",
			PaymentMethod:  gateway.MethodCreditCard,
			CustomerEmail:  email,
			CustomerName:   req.CustomerName,
			CustomerDoc:    "",
			CustomerPhone:  orderData.CustomerPhone,
			CardData: &gateway.CardData{
				Token:        req.CardToken,
				Installments: installments,
				HolderName:   req.CustomerName,
				HolderDoc:    "",
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

		// Usa os IDs autorizativos de order_documents (nunca confia no request body).
		payment := models.Payment{
			OrderID:         req.OrderID,
			CustomerID:      orderData.CustomerID,
			CustomerPhone:   orderData.CustomerPhone,
			EstablishmentID: orderData.EstablishmentID,
			Amount:          amount,
			DeliveryAmount:  orderData.DeliveryAmount,
			Method:          req.Method,
			Status:          paymentStatus,
			Installments:    installments,
			CardLastDigits:  resp.CardLast4,
			CreatedAt:       time.Now(),
			ConfirmedAt:     confirmedAt,
		}

		if err := models.DB.Create(&payment).Error; err != nil {
			log.Printf("[CARD] Erro ao salvar pagamento no Postgres: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		log.Printf("[CARD] Payment created for order %s: customer=%d establishment=%d amount=%.2f status=%s",
			req.OrderID, orderData.CustomerID, orderData.EstablishmentID, amount, paymentStatus)

		response := dto.PaymentResponse{
			PaymentID:    payment.IDString(),
			Status:       paymentStatus,
			Message:      fmt.Sprintf("Payment processed via %s", resp.Gateway),
		}

		return c.Status(201).JSON(response)
	}

	if req.Method == "pix" {
		gatewayReq := &gateway.TransactionRequest{
			OrderID:        parseOrderID(req.OrderID),
			Amount:        int64(amount * 100),
			Currency:      "BRL",
			PaymentMethod: gateway.MethodPIX,
			CustomerEmail: email,
			CustomerName:  req.CustomerName,
			CustomerDoc:   "",
			CustomerPhone: orderData.CustomerPhone,
			Capture:       true,
		}

		resp, err := router.CreateTransactionWithFallback(c.Context(), gatewayReq)
		if err != nil {
			log.Printf("Error processing PIX payment via router: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "PIX payment failed"})
		}

		// Usa os IDs autorizativos de order_documents (nunca confia no request body).
		payment := models.Payment{
			OrderID:         req.OrderID,
			CustomerID:      orderData.CustomerID,
			CustomerPhone:   orderData.CustomerPhone,
			EstablishmentID: orderData.EstablishmentID,
			Amount:          amount,
			DeliveryAmount:  orderData.DeliveryAmount,
			Method:          "pix",
			Status:          "PENDING",
			PixCopyPaste:    resp.PIXCopyPaste,
			QRCodeBase64:    resp.PIXQRCode,
			PixQRCode:       resp.PIXQRCode,
			CreatedAt:       time.Now(),
		}

		if err := models.DB.Create(&payment).Error; err != nil {
			log.Printf("[PIX] Erro ao salvar pagamento no Postgres: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		log.Printf("[PIX] Payment created for order %s: customer=%d establishment=%d amount=%.2f",
			req.OrderID, orderData.CustomerID, orderData.EstablishmentID, amount)

		response := dto.PaymentResponse{
			PaymentID:    payment.IDString(),
			Status:       "PENDING",
			PixCopyPaste: resp.PIXCopyPaste,
			QRCodeBase64: resp.PIXQRCode,
			PixQRCode:    resp.PIXQRCode,
			Message:      fmt.Sprintf("PIX payment created via %s", resp.Gateway),
		}

		return c.Status(201).JSON(response)
	}

	return c.Status(400).JSON(fiber.Map{"error": "Invalid payment method"})
}

func parseOrderID(id string) int64 {
	if id == "" {
		return 0
	}
	var n int64
	fmt.Sscanf(id, "%d", &n)
	return n
}
