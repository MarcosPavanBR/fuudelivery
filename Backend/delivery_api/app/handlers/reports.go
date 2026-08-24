package handlers

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/carloshomar/fuudelivery/delivery_api/app/models"
)

func GetExtrato(c *fiber.Ctx) error {
	deliverymanIDStr := c.Params("id")
	deliverymanID, err := strconv.ParseInt(deliverymanIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de deliveryman inválido",
		})
	}

	ctx, cancel := deliverymanCtx()
	defer cancel()

	orders, err := models.FindFinishedOrdersByDeliveryman(ctx, deliverymanID)
	if err != nil {
		log.Printf("Erro ao consultar os pedidos: %s", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao consultar os pedidos",
		})
	}

	return c.JSON(orders)
}
