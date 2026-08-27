package handlers

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
)

// toCents converte reais (float64) para centavos (int64) com arredondamento
// seguro — evita truncamento de 99.99*100 = 9998.9999… → 9998.
func toCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

// GeneratePIX cria uma cobrança PIX no gateway (AbacatePay) e persiste o
// pagamento em Postgres (corte 4 — fonte da verdade), com dual-write
// best-effort no Mongo legado.
func GeneratePIX(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	description := fmt.Sprintf("Pedido %s", req.OrderID)

	// O valor cobrado é o total recalculado no servidor na criação do pedido —
	// nunca o amount enviado pelo cliente (que poderia pagar R$0,01 por um
	// pedido de R$100,00).
	serverTotal, ok := validateChargeAmount(req.OrderID, req.Amount)
	if !ok {
		log.Printf("[PIX] Cobrança rejeitada: valor diverge do pedido %s (client=%.2f)", req.OrderID, req.Amount)
		return c.Status(400).JSON(fiber.Map{"error": "Valor da cobrança não corresponde ao pedido"})
	}
	req.Amount = serverTotal

	client := services.NewAbacatePayClient()
	chargeReq := services.PIXChargeRequest{}
	// req.Amount está em REAIS (unidade persistida no Postgres); o gateway
	// AbacatePay espera CENTAVOS (int64). Sem esta conversão um pedido de
	// R$100,00 gerava uma cobrança de R$1,00 (subcobrança).
	chargeReq.Data.Amount = toCents(req.Amount)
	chargeReq.Data.Description = description
	chargeReq.Data.ExternalID = req.OrderID
	// customer é opcional para PIX; se enviado, TODOS os campos (incl. taxId/CPF
	// válido) são obrigatórios. O monolito não coleta CPF → omitir para não
	// tomar 422 do gateway. (O AppComida poderá passar taxId no futuro.)

	apiResp, err := client.CreatePIXCharge(chargeReq)
	if err != nil {
		log.Printf("Error creating PIX payment via AbacatePay: %v", err)
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
		PixQRCode:       apiResp.QRCode,
		PixCopyPaste:    apiResp.CopyPaste,
		QRCodeBase64:    apiResp.QRCodeBase64,
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
		PixQRCode:    apiResp.QRCode,
		PixCopyPaste: apiResp.CopyPaste,
		QRCodeBase64: apiResp.QRCodeBase64,
		AbacatePayID: apiResp.ID,
		Message:      "PIX payment created via AbacatePay",
	}

	return c.Status(201).JSON(response)
}
