package handlers

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/gofiber/fiber/v2"
)

// cardUnavailable é a resposta única de indisponibilidade de cartão.
// Mensagem e status ficam num só lugar para não divergirem entre os
// handlers (ChargeCard e ProcessPayment).
func cardUnavailable() (int, fiber.Map) {
	return fiber.StatusServiceUnavailable, fiber.Map{
		"error": "Pagamento por cartão temporariamente indisponível. Use PIX.",
	}
}

// cardAvailableViaRouter reporta se a CADEIA REAL tem gateway com suporte a
// cartão (Pagar.me, Asaas, Mercado Pago) — fonte única de verdade, do mesmo
// jeito que o /health passou a ler o router e não mais uma lista fixa.
//
// A versão antiga consultava os.Getenv("PAGARME_API_KEY")/ASAAS/MP: era uma
// segunda fonte que divergia da cadeia montada em buildPaymentGateways —
// credencial presente porém INVÁLIDA entrava na cadeia, o runtime falhava
// com 401 e a cobrança respondia 500 (erro do servidor) em vez de 503
// (indisponibilidade). O AbacatePay está na cadeia mas só suporta PIX, e o
// filtro é por método, então ele nunca satisfaz a pergunta de cartão.
func cardAvailableViaRouter(router *gateway.Router) bool {
	return router.HasAvailableForMethod(gateway.MethodCreditCard)
}

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

	routerEarly, rErr := getPaymentRouter(c)
	if rErr != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Payment router unavailable"})
	}

	// Fonte única: a cadeia do router decide se cartão está disponível —
	// não mais env vars espelhadas.
	if !cardAvailableViaRouter(routerEarly) {
		status, body := cardUnavailable()
		return c.Status(status).JSON(body)
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
		OrderID:       0, // ChargeCard é avulsa, sem pedido interno
		Amount:        toCents(req.Amount),
		Currency:      "BRL",
		PaymentMethod: gateway.MethodCreditCard,
		CustomerEmail: email,
		CustomerName:  name,
		CustomerDoc:   req.CPF,
		CustomerPhone: req.Phone,
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
		// Nenhum gateway elegível (na seleção ou depois de esgotar a cadeia) é
		// indisponibilidade, não erro do servidor. 500 fazia o cliente achar
		// que a cobrança pode ter passado.
		if errors.Is(err, gateway.ErrNoGatewayAvailable) || errors.Is(err, gateway.ErrGatewayFailed) {
			status, body := cardUnavailable()
			return c.Status(status).JSON(body)
		}
		return c.Status(500).JSON(fiber.Map{"error": "Card payment failed"})
	}

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
func ProcessPayment(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

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

	// Idem ao PIX: o frete alimenta o split e, se vier >= total, zera
	// plataforma e estabelecimento. Validado antes dos dois ramos abaixo
	// (cartão e pix), que gravam req.DeliveryAmount no pagamento.
	serverDelivery, deliveryOK := resolveDeliveryAmount(req.OrderID, req.DeliveryAmount, serverTotal)
	if !deliveryOK {
		log.Printf("[CARD] Cobrança rejeitada: frete diverge do pedido %s (client=%.2f total=%.2f)",
			req.OrderID, req.DeliveryAmount, serverTotal)
		return c.Status(400).JSON(fiber.Map{"error": "Valor da entrega não corresponde ao pedido"})
	}
	req.DeliveryAmount = serverDelivery

	if !bindRecipientToOrder(c, &req) {
		log.Printf("[CARD] Cobrança rejeitada: pedido %s sem estabelecimento conhecido", req.OrderID)
		return c.Status(400).JSON(fiber.Map{"error": "Pedido inválido para cobrança"})
	}

	router, err := getPaymentRouter(c)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Payment router unavailable"})
	}

	if req.Method == "credit" || req.Method == "debit" {
		if !cardAvailableViaRouter(router) {
			status, body := cardUnavailable()
			return c.Status(status).JSON(body)
		}
		installments := req.Installments
		if installments <= 0 {
			installments = 1
		}

		gatewayReq := &gateway.TransactionRequest{
			OrderID:       0,
			Amount:        toCents(req.Amount),
			Currency:      "BRL",
			PaymentMethod: gateway.MethodCreditCard,
			CustomerEmail: email,
			CustomerName:  req.CustomerName,
			CustomerDoc:   "",
			CustomerPhone: req.CustomerPhone,
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
			// Mesma semântica do ChargeCard: cadeia sem gateway elegível é
			// indisponibilidade (503), não erro do servidor. 500 aqui fazia o
			// cliente suspeitar que a cobrança tivesse passado.
			if errors.Is(err, gateway.ErrNoGatewayAvailable) || errors.Is(err, gateway.ErrGatewayFailed) {
				status, body := cardUnavailable()
				return c.Status(status).JSON(body)
			}
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
			OrderID:          req.OrderID,
			CustomerID:       req.CustomerID,
			CustomerPhone:    req.CustomerPhone,
			EstablishmentID:  req.EstablishmentID,
			Amount:           req.Amount,
			DeliveryAmount:   req.DeliveryAmount,
			DiscountAmount:   req.DiscountAmount,
			DiscountFundedBy: req.DiscountFundedBy,
			Method:           req.Method,
			Status:           paymentStatus,
			Installments:     installments,
			CardLastDigits:   resp.CardLast4,
			CreatedAt:        time.Now(),
			ConfirmedAt:      confirmedAt,
		}

		if err := models.DB.Create(&payment).Error; err != nil {
			log.Printf("[CARD] Erro ao salvar pagamento no Postgres: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		response := dto.PaymentResponse{
			PaymentID: payment.IDString(),
			Status:    paymentStatus,
			Message:   fmt.Sprintf("Payment processed via %s", resp.Gateway),
		}

		return c.Status(201).JSON(response)
	}

	if req.Method == "pix" {
		gatewayReq := &gateway.TransactionRequest{
			OrderID:       parseOrderID(req.OrderID),
			Amount:        toCents(req.Amount),
			Currency:      "BRL",
			PaymentMethod: gateway.MethodPIX,
			CustomerEmail: email,
			CustomerName:  req.CustomerName,
			CustomerDoc:   "",
			CustomerPhone: req.CustomerPhone,
			Description:   fmt.Sprintf("Pedido %s", req.OrderID),
			Capture:       true,
			// IDs de pedido legados são strings (hex ObjectID): vão no metadata
			// para o adapter montar o externalId real do gateway.
			Metadata: map[string]string{
				"order_id":       req.OrderID,
				"customer_phone": req.CustomerPhone,
			},
		}

		resp, err := router.CreateTransactionWithFallback(c.Context(), gatewayReq)
		if err != nil {
			log.Printf("Error processing PIX payment via router: %v", err)
			if errors.Is(err, gateway.ErrNoGatewayAvailable) {
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
					"error": "Pagamento por PIX temporariamente indisponível. Tente novamente em instantes.",
				})
			}
			return c.Status(500).JSON(fiber.Map{"error": "PIX payment failed"})
		}

		payment := models.Payment{
			OrderID:          req.OrderID,
			CustomerID:       req.CustomerID,
			CustomerPhone:    req.CustomerPhone,
			EstablishmentID:  req.EstablishmentID,
			Amount:           req.Amount,
			DeliveryAmount:   req.DeliveryAmount,
			DiscountAmount:   req.DiscountAmount,
			DiscountFundedBy: req.DiscountFundedBy,
			Method:           "pix",
			Status:           "PENDING",
			PixCopyPaste:     resp.PIXCopyPaste,
			QRCodeBase64:     resp.PIXQRCodeBase64,
			PixQRCode:        resp.PIXQRCode,
			CreatedAt:        time.Now(),
		}

		if err := models.DB.Create(&payment).Error; err != nil {
			log.Printf("[PIX] Erro ao salvar pagamento no Postgres: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		response := dto.PaymentResponse{
			PaymentID:    payment.IDString(),
			Status:       "PENDING",
			PixCopyPaste: resp.PIXCopyPaste,
			QRCodeBase64: resp.PIXQRCodeBase64,
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
