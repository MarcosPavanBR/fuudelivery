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
//
// Autorização: valida que o usuário autenticado (JWT) é o dono do pedido
// antes de criar a cobrança. Os IDs de cliente/estabelecimento são extraídos
// de order_documents (fonte da verdade) — nunca confia no request body.
func GeneratePIX(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Valida que o usuário autenticado é o dono do pedido e carrega os dados
	// autorizativos (establishment_id, customer_id, order_total, etc.).
	orderData, err := authorizeAndLoadOrder(c, req.OrderID)
	if err != nil {
		if err == ErrUnauthorizedPayment {
			log.Printf("[PIX] Authorization failed for order %s: user does not own order", req.OrderID)
			return c.Status(403).JSON(fiber.Map{"error": "Acesso negado: você não é o dono deste pedido"})
		}
		if err == ErrOrderNotFound {
			log.Printf("[PIX] Order %s not found", req.OrderID)
			return c.Status(404).JSON(fiber.Map{"error": "Pedido não encontrado"})
		}
		if err == ErrOrderAlreadyPaid {
			log.Printf("[PIX] Order %s already has a confirmed payment", req.OrderID)
			return c.Status(409).JSON(fiber.Map{"error": "Este pedido já foi pago"})
		}
		log.Printf("[PIX] Failed to authorize order %s: %v", req.OrderID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao validar pedido"})
	}

	description := fmt.Sprintf("Pedido %s", req.OrderID)

	// Usa o total do pedido extraído de order_documents (fonte da verdade).
	// Ignora completamente o amount enviado pelo cliente.
	amount := orderData.OrderTotal

	client := services.NewAbacatePayClient()
	chargeReq := services.PIXChargeRequest{}
	// amount está em REAIS (unidade persistida no Postgres); o gateway
	// AbacatePay espera CENTAVOS (int64). Sem esta conversão um pedido de
	// R$100,00 gerava uma cobrança de R$1,00 (subcobrança).
	chargeReq.Data.Amount = toCents(amount)
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

	log.Printf("[PIX] Payment created for order %s: customer=%d establishment=%d amount=%.2f",
		req.OrderID, orderData.CustomerID, orderData.EstablishmentID, amount)

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
