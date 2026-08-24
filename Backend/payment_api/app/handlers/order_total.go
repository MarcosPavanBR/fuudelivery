package handlers

import (
	"log"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
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
