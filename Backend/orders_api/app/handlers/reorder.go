package handlers

// reorder.go — "repetir pedido" (monta carrinho a partir de um pedido antigo).
// CORTE 5: leitura Postgres-first com lazy import do Mongo legado.

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

func RepeatOrder(c *fiber.Ctx) error {
	orderID := c.Params("orderId")

	doc, err := findOrderByLegacyID(orderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
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
