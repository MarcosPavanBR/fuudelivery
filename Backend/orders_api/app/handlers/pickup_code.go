package handlers

// pickup_code.go — geração e validação do código de retirada.
// CORTE 5: Postgres primário (coluna tipada pickup_code em order_documents);
// o Mongo legado é espelhado best-effort dentro de saveOrderPrimary.

import (
	"crypto/rand"
	"math/big"

	"github.com/gofiber/fiber/v2"
)

func generateSecureCode() string {
	const charset = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		code[i] = charset[n.Int64()]
	}
	return string(code)
}

func GeneratePickupCode(c *fiber.Ctx) error {
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	doc, err := findOrderByLegacyID(req.OrderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
	}

	doc.PickupCode = generateSecureCode()
	if err := saveOrderPrimary(doc); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate code"})
	}

	return c.JSON(fiber.Map{
		"pickup_code": doc.PickupCode,
		"order_id":    req.OrderID,
		"message":     "Código de retirada gerado com sucesso",
	})
}

func ValidatePickupCode(c *fiber.Ctx) error {
	var req struct {
		OrderID    string `json:"order_id"`
		PickupCode string `json:"pickup_code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	doc, err := findOrderByLegacyID(req.OrderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
	}

	if doc.PickupCode == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nenhum código de retirada gerado"})
	}

	if doc.PickupCode != req.PickupCode {
		return c.Status(401).JSON(fiber.Map{
			"valid": false,
			"error": "Código inválido",
		})
	}

	return c.JSON(fiber.Map{
		"valid":    true,
		"message":  "Código válido! Pedido liberado para retirada.",
		"order_id": req.OrderID,
	})
}

func GetPickupCode(c *fiber.Ctx) error {
	orderID := c.Params("id")

	doc, err := findOrderByLegacyID(orderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
	}

	generatedAt := ""
	if !doc.UpdatedAt.IsZero() && doc.PickupCode != "" {
		generatedAt = doc.UpdatedAt.Format("2006-01-02 15:04:05")
	}

	return c.JSON(fiber.Map{
		"order_id":        orderID,
		"pickup_code":     doc.PickupCode,
		"generated_at":    generatedAt,
		"has_pickup_code": doc.PickupCode != "",
	})
}
