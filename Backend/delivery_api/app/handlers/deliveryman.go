package handlers

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	"github.com/carloshomar/fuudelivery/delivery_api/app/models"
)

func deliverymanCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func GetOrdersByDeliverymanID(c *fiber.Ctx) error {
	deliverymanIDStr := c.Params("id")
	deliverymanID, err := strconv.ParseInt(deliverymanIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de deliveryman inválido",
		})
	}

	ctx, cancel := deliverymanCtx()
	defer cancel()

	orders, err := models.FindActiveOrdersByDeliveryman(ctx, deliverymanID)
	if err != nil {
		log.Printf("Erro ao consultar os pedidos: %s", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao consultar os pedidos",
		})
	}

	return c.JSON(orders)
}

func GetOrderByID(orderID string) (*dto.OrderDTO, error) {
	ctx, cancel := deliverymanCtx()
	defer cancel()
	return models.GetSolicitationByOrderID(ctx, orderID)
}

func UpdateOrderStatusByDeliverymanID(c *fiber.Ctx, sendMessageToClient func(clientID int64, message []byte) error) error {

	var request struct {
		OrderID     string `json:"order_id"`
		Deliveryman struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"deliveryman"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}

	ctx, cancel := deliverymanCtx()
	defer cancel()

	found, err := models.UpdateSolicitationDeliveryManStatus(ctx, request.OrderID, request.Deliveryman.Id, request.Deliveryman.Status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar o status do pedido",
		})
	}

	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Pedido nao encontrado ou entregador nao autorizado",
		})
	}

	order, err := GetOrderByID(request.OrderID)
	if err != nil || order == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	// RabbitMQ removido — fila gerenciada pelo monolito via Redis
	log.Printf("[DELIVERY] Order %s status update published", order.OrderId)

	return c.JSON(fiber.Map{
		"message": "Status do pedido atualizado com sucesso",
	})
}
