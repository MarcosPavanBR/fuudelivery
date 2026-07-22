// Package handlers - report_handler.go
// Handler HTTP para o endpoint de relatorios de pagamentos.
// Fornece metricas consolidadas de receita, pedidos e ticket medio
// para um restaurante especifico em um periodo determinado.
package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/repository"
)

// ReportHandler e responsavel pelas rotas de relatorios.
type ReportHandler struct{}

// NewReportHandler cria uma nova instancia do handler.
func NewReportHandler() *ReportHandler {
	return &ReportHandler{}
}

// GetEstablishmentReport retorna o relatorio consolidado de um restaurante.
// GET /api/reports/establishment/:id?period=month
//
// Periodos aceitos: week (7 dias), month (30 dias), quarter (90 dias), year (365 dias).
// Retorna: total_revenue, total_orders, avg_ticket, delivery_revenue,
// orders_by_status, revenue_by_day, payment_methods.
func (rh *ReportHandler) GetEstablishmentReport(c *fiber.Ctx) error {
	establishmentID := c.Params("id")
	if establishmentID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Establishment ID is required"})
	}

	period := c.Query("period", "month")
	if period == "" {
		period = "month"
	}

	// Valida periodo
	validPeriods := map[string]bool{
		"week": true, "month": true, "quarter": true, "year": true,
	}
	if !validPeriods[period] {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid period. Use: week, month, quarter, or year",
		})
	}

	report, err := repository.GetEstablishmentReport(establishmentID, period)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate report"})
	}

	return c.JSON(report)
}
