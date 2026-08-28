package handlers

import (
	"errors"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	"github.com/carloshomar/fuudelivery/delivery_api/app/models"
	"gorm.io/gorm"
)

// ============================================================================
// Handlers do entregador — 100% Postgres
// (tabela delivery_solicitations, sql/02)
// ============================================================================

func canAccessDeliveryman(c *fiber.Ctx, deliverymanID int64) bool {
	role, err := middlewares.GetUserRoleFromToken(c)
	if err != nil {
		return false
	}
	if role == "admin" {
		return true
	}
	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return false
	}
	return tokenUserID == deliverymanID
}

func GetOrdersByDeliverymanID(c *fiber.Ctx) error {
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
	if err := models.DB.
		Where("delivery_man_id = ?", deliverymanID).
		Where("status <> ? OR status IS NULL", "FINISHED").
		Where("delivery_man_status <> ? OR delivery_man_status IS NULL", "FINISHED").
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

// GetOrderByID busca uma solicitação no read-model Postgres.
// Usado pelo handshake e pelo motor de despacho.
func GetOrderByID(orderID string) (*dto.OrderDTO, error) {
	var row models.DeliverySolicitation
	err := models.DB.Where("order_id = ?", orderID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		log.Printf("Erro ao consultar o pedido %s: %s", orderID, err)
		return nil, err
	}

	order := row.ToDTO()
	return &order, nil
}

func UpdateOrderStatusByDeliverymanID(c *fiber.Ctx, sendMessageToClient func(clientID int64, message []byte) error) error {

	var request struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}

	tokenUserID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	// Atualiza somente se o pedido pertence ao entregador autenticado.
	result := models.DB.Model(&models.DeliverySolicitation{}).
		Where("order_id = ? AND delivery_man_id = ?", request.OrderID, tokenUserID).
		Updates(map[string]interface{}{
			"delivery_man_status": request.Status,
		})

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar o status do pedido",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Pedido nao encontrado ou entregador nao autorizado",
		})
	}

	order, err := GetOrderByID(request.OrderID)
	if err != nil || order == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Order not found"})
	}

	log.Printf("[DELIVERY] Order %s status update published", order.OrderId)

	return c.JSON(fiber.Map{
		"message": "Status do pedido atualizado com sucesso",
	})
}
