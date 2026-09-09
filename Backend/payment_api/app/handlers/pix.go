package handlers

import (
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/gofiber/fiber/v2"
)

// toCents converte reais (float64) para centavos (int64) com arredondamento
// seguro — evita truncamento de 99.99*100 = 9998.9999… → 9998.
func toCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

// GeneratePIX cria uma cobrança PIX e persiste o pagamento em Postgres
// (corte 4 — fonte da verdade), com dual-write best-effort no Mongo legado.
//
// A cobrança passa pelo PaymentRouter (item 1.7 do roadmap de split) em vez
// de chamar o AbacatePay direto: o router resolve o gateway elegível para PIX
// (fallback chain + circuit breaker), e amanhã o PIX pode migrar de provider
// sem tocar neste handler. ProcessPayment (cartão/PIX) já usa este caminho.
func GeneratePIX(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// O valor cobrado é o total recalculado no servidor na criação do pedido —
	// nunca o amount enviado pelo cliente (que poderia pagar R$0,01 por um
	// pedido de R$100,00).
	serverTotal, ok := validateChargeAmount(req.OrderID, req.Amount)
	if !ok {
		log.Printf("[PIX] Cobrança rejeitada: valor diverge do pedido %s (client=%.2f)", req.OrderID, req.Amount)
		return c.Status(400).JSON(fiber.Map{"error": "Valor da cobrança não corresponde ao pedido"})
	}
	req.Amount = serverTotal

	// O frete também não pode vir do cliente: ele entra no split e, se for
	// >= total, zera plataforma e estabelecimento (ver resolveDeliveryAmount).
	serverDelivery, deliveryOK := resolveDeliveryAmount(req.OrderID, req.DeliveryAmount, serverTotal)
	if !deliveryOK {
		log.Printf("[PIX] Cobrança rejeitada: frete diverge do pedido %s (client=%.2f total=%.2f)",
			req.OrderID, req.DeliveryAmount, serverTotal)
		return c.Status(400).JSON(fiber.Map{"error": "Valor da entrega não corresponde ao pedido"})
	}
	req.DeliveryAmount = serverDelivery

	// Destinatário do dinheiro também não pode vir do corpo — era por aí que
	// dava para redirecionar o split do estabelecimento.
	if !bindRecipientToOrder(c, &req) {
		log.Printf("[PIX] Cobrança rejeitada: pedido %s sem estabelecimento conhecido", req.OrderID)
		return c.Status(400).JSON(fiber.Map{"error": "Pedido inválido para cobrança"})
	}

	router, err := getPaymentRouter(c)
	if err != nil {
		log.Printf("[PIX] Router indisponível: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Payment router unavailable"})
	}

	// req.Amount está em REAIS (unidade persistida no Postgres); o gateway
	// espera CENTAVOS (int64). Sem esta conversão um pedido de R$100,00
	// geraria uma cobrança de R$1,00 (subcobrança).
	gatewayReq := &gateway.TransactionRequest{
		// OrderID é numérico no request do router; o copia-e-cola leva o
		// externalId legado (string) via Description + Metadata — o webhook
		// casa a cobrança pelo AbacatePayID persistido, não pelo externalId.
		Amount:        toCents(req.Amount),
		Currency:      "BRL",
		PaymentMethod: gateway.MethodPIX,
		CustomerEmail: defaultString(req.CustomerEmail, "cliente@email.com"),
		CustomerName:  defaultString(req.CustomerName, "Cliente"),
		CustomerPhone: req.CustomerPhone,
		Description:   fmt.Sprintf("Pedido %s", req.OrderID),
		Capture:       true,
		Metadata: map[string]string{
			"order_id":       req.OrderID,
			"customer_phone": req.CustomerPhone,
		},
		// customer é opcional para PIX; se enviado, TODOS os campos (incl.
		// taxId/CPF válido) são obrigatórios. O monolito não coleta CPF →
		// omitir para não tomar 422 do gateway.
	}

	resp, err := router.CreateTransactionWithFallback(c.Context(), gatewayReq)
	if err != nil {
		log.Printf("[PIX] Erro criando cobrança via router: %v", err)
		// Cadeia sem gateway elegível é indisponibilidade, não erro do
		// servidor — mesma semântica do ProcessPayment. 500 faria o cliente
		// achar que a cobrança pode ter passado.
		if errors.Is(err, gateway.ErrNoGatewayAvailable) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Pagamento por PIX temporariamente indisponível. Tente novamente em instantes.",
			})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create PIX payment"})
	}

	// ID é BIGSERIAL no Postgres — preenchido automaticamente pelo Create.
	payment := models.Payment{
		OrderID:         req.OrderID,
		CustomerID:      req.CustomerID,
		CustomerPhone:   req.CustomerPhone,
		EstablishmentID: req.EstablishmentID,
		Amount:          req.Amount,
		DeliveryAmount:  req.DeliveryAmount,
		Method:          "pix",
		Status:          "PENDING",
		PixQRCode:       resp.PIXQRCode,
		PixCopyPaste:    resp.PIXCopyPaste,
		QRCodeBase64:    resp.PIXQRCodeBase64,
		AbacatePayID:    resp.GatewayID,
		CreatedAt:       time.Now(),
	}

	if err := models.DB.Create(&payment).Error; err != nil {
		log.Printf("[PIX] Erro ao salvar pagamento no Postgres: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
	}

	response := dto.PaymentResponse{
		PaymentID:    payment.IDString(),
		Status:       "PENDING",
		PixQRCode:    resp.PIXQRCode,
		PixCopyPaste: resp.PIXCopyPaste,
		QRCodeBase64: resp.PIXQRCodeBase64,
		AbacatePayID: resp.GatewayID,
		Message:      fmt.Sprintf("PIX payment created via %s", resp.Gateway),
	}

	return c.Status(201).JSON(response)
}

// defaultString devolve fallback quando s é vazio.
func defaultString(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
