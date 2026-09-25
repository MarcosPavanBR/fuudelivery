package handlers

// scheduling.go — agendamento de pedidos.
// CORTE 5: Postgres primário (colunas scheduled_at / is_scheduled em
// order_documents), com espelho no Mongo best-effort via patchOrderDoc.

import (
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
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

	if !canScheduleOrder(c, doc) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}
	if !scheduledTime.After(time.Now()) {
		return c.Status(400).JSON(fiber.Map{"error": "Scheduled time must be in the future"})
	}

	err = patchOrderDoc(doc, func(_ *models.OrderDocument, p *dto.RequestPayload) error {
		p.ScheduledAt = &scheduledTime
		p.IsScheduled = true
		return nil
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to schedule"})
	}

	return c.JSON(fiber.Map{"message": "Order scheduled", "scheduled_at": scheduledTime})
}

// canScheduleOrder: reagendar é do cliente dono do pedido (telefone do token)
// ou da loja do pedido/admin. Antes, qualquer usuário logado reagendava o
// pedido de qualquer um.
func canScheduleOrder(c *fiber.Ctx, doc *models.OrderDocument) bool {
	if canActOnEstablishment(c, doc.EstablishmentID) {
		return true
	}
	tokenPhone, err := middlewares.GetUserPhoneFromToken(c)
	return err == nil && tokenPhone != "" && doc.UserPhone == tokenPhone
}
