package handlers

import (
	"log"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// lookupOrderTotal devolve o total recalculado pelo servidor no momento da
// criação do pedido (campo order_total do JSONB em order_documents, escrito
// por orders_api/computeOrderTotal). Retorna false se o pedido não existir
// ou ainda não tiver total válido (pedidos anteriores ao corte de valores
// server-side) — nesses casos a cobrança é rejeitada em vez de confiar no
// amount enviado pelo cliente.
func lookupOrderTotal(orderID string) (float64, bool) {
	if models.DB == nil {
		return 0, false
	}
	var row struct {
		Total *float64
	}
	err := models.DB.Raw(
		`SELECT NULLIF(payload->>'order_total', '')::float8 AS total
		 FROM order_documents
		 WHERE legacy_id = ?
		 LIMIT 1`, orderID).Scan(&row).Error
	if err != nil {
		log.Printf("[PAYMENT] lookupOrderTotal(%s): %v", orderID, err)
		return 0, false
	}
	if row.Total == nil || *row.Total <= 0 {
		return 0, false
	}
	return *row.Total, true
}

// validateChargeAmount garante que o valor cobrado é exatamente o total do
// pedido calculado no servidor. Tolerância de 1 centavo para ruído de float.
func validateChargeAmount(orderID string, clientAmount float64) (float64, bool) {
	serverTotal, ok := lookupOrderTotal(orderID)
	if !ok {
		return 0, false
	}
	diff := toCents(serverTotal) - toCents(clientAmount)
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		return serverTotal, false
	}
	return serverTotal, true
}

// GetPaymentByOrder devolve o status da cobrança mais recente de um pedido.
// Usado pelo app do cliente para confirmar o pagamento do PIX (polling) sem
// confiar num botão "já paguei".
// GET /payments/order/:order_id (protegido)
func GetPaymentByOrder(c *fiber.Ctx) error {
	orderID := c.Params("order_id")
	if orderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id obrigatório"})
	}

	var payment models.Payment
	if err := models.DB.Where("order_id = ?", orderID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Nenhuma cobrança encontrada para este pedido"})
	}

	// IDOR protection: verify the caller can access this payment.
	// Admin can see anything; restaurant can only see their own establishment's payments.
	role, _ := middlewares.GetUserRoleFromToken(c)
	if role != "admin" {
		tokenEst, err := middlewares.GetEstablishmentIDFromToken(c)
		if err != nil || tokenEst != payment.EstablishmentID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot view another establishment's payment"})
		}
	}

	return c.JSON(fiber.Map{
		"order_id":       payment.OrderID,
		"status":         payment.Status,
		"amount":         payment.Amount,
		"payment_method": payment.Method,
	})
}
