package handlers

// scheduling.go — agendamento de pedidos.
// CORTE 5: Postgres primário (colunas scheduled_at / is_scheduled em
// order_documents), com espelho no Mongo best-effort via patchOrderDoc.

import (
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/gofiber/fiber/v2"
)

type ScheduleRequest struct {
	OrderID     string `json:"order_id"`
	ScheduledAt string `json:"scheduled_at"`
}

func ScheduleOrder(c *fiber.Ctx) error {
	var req ScheduleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	scheduledTime, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid date format"})
	}

	doc, err := findOrderByLegacyID(req.OrderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
	}

	err = patchOrderDoc(doc, func(p *dto.RequestPayload) {
		p.ScheduledAt = &scheduledTime
		p.IsScheduled = true
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to schedule"})
	}

	return c.JSON(fiber.Map{"message": "Order scheduled", "scheduled_at": scheduledTime})
}
