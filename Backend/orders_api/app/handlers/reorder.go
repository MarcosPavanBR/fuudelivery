package handlers

// reorder.go — "repetir pedido" (monta carrinho a partir de um pedido antigo).
// CORTE 5: leitura Postgres-first com lazy import do Mongo legado.

import (
	"encoding/json"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/gofiber/fiber/v2"
)

func RepeatOrder(c *fiber.Ctx) error {
	orderID := c.Params("orderId")

	doc, err := findOrderByLegacyID(orderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
	}

	// Authorization: verify caller owns this order or is admin
	role, roleErr := middlewares.GetUserRoleFromToken(c)
	if roleErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	if role != "admin" {
		tokenPhone, phoneErr := middlewares.GetUserPhoneFromToken(c)
		if phoneErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}

		// Non-admin users can only repeat their own orders
		if doc.UserPhone != tokenPhone {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Cannot repeat another user's order",
			})
		}
	}

	// O payload completo (cart + establishment) está no JSONB; extraímos
	// apenas os campos que o cliente consome.
	var payload struct {
		Cart          json.RawMessage `json:"cart"`
		Establishment json.RawMessage `json:"establishment"`
	}
	if err := json.Unmarshal(doc.Payload, &payload); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Falha ao ler o pedido"})
	}

	response := fiber.Map{
		"cart":          json.RawMessage(payload.Cart),
		"establishment": json.RawMessage(payload.Establishment),
	}

	return c.JSON(response)
}
