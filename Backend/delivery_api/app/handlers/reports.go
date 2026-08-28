package handlers

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	"github.com/carloshomar/fuudelivery/delivery_api/app/models"
)

// GetExtrato lista os pedidos FINALIZADOS de um entregador, mais recentes
// primeiro — CORTE 3 banco-único: leitura 100% Postgres.
func GetExtrato(c *fiber.Ctx) error {
	deliverymanIDStr := c.Params("id")
	deliverymanID, err := strconv.ParseInt(deliverymanIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de deliveryman inválido",
		})
	}

	if !canAccessDeliveryman(c, deliverymanID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	var rows []models.DeliverySolicitation
	// Equivalente ao filtro Mongo antigo {deliveryman.id, status=FINISHED,
	// deliveryman.status=FINISHED} ordenado por operationDate DESC — aqui,
	// updated_at (espelhado como Operation no modelo).
	if err := models.DB.
		Where("delivery_man_id = ? AND status = ? AND delivery_man_status = ?",
			deliverymanID, "FINISHED", "FINISHED").
		Order("updated_at DESC").
		Find(&rows).Error; err != nil {
		log.Printf("Erro ao consultar os pedidos: %s", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao consultar os pedidos",
		})
	}

	orders := make([]dto.OrderDTO, 0, len(rows))
	for _, row := range rows {
		orders = append(orders, row.ToDTO())
	}

	return c.JSON(orders)
}
