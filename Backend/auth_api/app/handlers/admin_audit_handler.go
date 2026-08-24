package handlers

import (
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/audit"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// ListAdminAuditLog lista as acoes administrativas registradas no log de
// auditoria (quem aprovou/rejeitou pagamento, excluiu usuario, etc.).
//
//	GET /audit-log?page=1&limit=20&action=PAYMENT_APPROVED&admin=joao&resource_type=user&from=2026-08-01T00:00:00Z&to=2026-08-17T23:59:59Z
func ListAdminAuditLog(c *fiber.Ctx) error {
	q := audit.Query{
		Action:       c.Query("action"),
		Admin:        c.Query("admin"),
		ResourceType: c.Query("resource_type"),
		Page:         c.QueryInt("page", 1),
		Limit:        c.QueryInt("limit", 20),
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			q.From = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			q.To = &t
		}
	}

	entries, total, err := audit.List(models.DB, q)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao listar auditoria"})
	}

	limit := q.Limit
	if limit < 1 {
		limit = 20
	}
	totalPages := (int(total) + limit - 1) / limit

	return c.JSON(fiber.Map{
		"data":        entries,
		"total":       total,
		"page":        q.Page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}
