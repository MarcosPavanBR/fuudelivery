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

// canAccessDeliveryman: admin ou o próprio entregador. O id sozinho não
// basta — clientes, usuários de loja e entregadores têm sequências de id
// independentes, e o cliente 5 não é o entregador 5.
func canAccessDeliveryman(c *fiber.Ctx, deliverymanID int64) bool {
	role, err := middlewares.GetUserRoleFromToken(c)
	if err != nil {
		return false
	}
	if role == "admin" {
		return true
	}
	return middlewares.IsOwnAccount(c, middlewares.AccountDeliveryMan, deliverymanID)
}

// courierFromToken devolve o id do entregador autenticado. status != 0 é a
// resposta a dar: 401 sem token válido, 403 se o token é de outro tipo de
// conta (cliente ou loja com o mesmo número de id não age como entregador).
func courierFromToken(c *fiber.Ctx) (id int64, status int) {
	accountType, err := middlewares.GetAccountTypeFromToken(c)
	if err != nil {
		return 0, fiber.StatusUnauthorized
	}
	if accountType != middlewares.AccountDeliveryMan {
		return 0, fiber.StatusForbidden
	}
	id, err = middlewares.GetUserIDFromToken(c)
	if err != nil || id <= 0 {
		return 0, fiber.StatusUnauthorized
	}
	return id, 0
}

// courierDenied é a resposta para um status != 0 de courierFromToken.
func courierDenied(c *fiber.Ctx, status int) error {
	msg := "Token inválido"
	if status == fiber.StatusForbidden {
		msg = "Apenas entregadores"
	}
	return c.Status(status).JSON(fiber.Map{"error": msg})
}

// nextCourierStatus é a progressão da entrega no app do entregador
// (nextDeliveryStatus em AppEntrega/app/delivery_mode.tsx). "" é o estado de
// quem ficou sem status — o app manda IN_ROUTE_COLECT nesse caso.
var nextCourierStatus = map[string]string{
	"":                  "IN_ROUTE_COLECT",
	"IN_ROUTE_COLECT":   "AWAIT_COLECT",
	"AWAIT_COLECT":      "IN_ROUTE_DELIVERY",
	"IN_ROUTE_DELIVERY": "FINISHED",
}

var courierStatuses = map[string]bool{
	"IN_ROUTE_COLECT": true, "AWAIT_COLECT": true, "IN_ROUTE_DELIVERY": true, "FINISHED": true,
}

// validCourierTransition aceita o próximo passo ou o mesmo status (o app
// repete a chamada quando a resposta se perde). Pular etapas — ir direto de
// coleta para FINISHED — não passa. Status fora da progressão (vazio, nulo ou
// legado) recomeça em IN_ROUTE_COLECT, como o app faz.
func validCourierTransition(from, to string) bool {
	if !courierStatuses[to] {
		return false
	}
	if !courierStatuses[from] {
		from = ""
	}
	return from == to || nextCourierStatus[from] == to
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
		Where("status NOT IN ? OR status IS NULL", []string{"FINISHED", "CANCELLED", "DENIED"}).
		Where("delivery_man_status <> ? OR delivery_man_status IS NULL", "FINISHED").
		Find(&rows).Error; err != nil {
		log.Printf("Erro ao consultar os pedidos: %s", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao consultar os pedidos",
		})
	}

	return c.JSON(courierOrderViews(rows, customerDetails))
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

	// O app do entregador manda {order_id, deliveryman: {id, status}}; o
	// "status" na raiz é aceito também. Só o formato da raiz era lido, e o
	// app gravava status vazio a cada passo — a entrega não avançava.
	// deliveryman.id é ignorado: quem é o entregador vem do token.
	var request struct {
		OrderID     string `json:"order_id"`
		Status      string `json:"status"`
		Deliveryman struct {
			Status string `json:"status"`
		} `json:"deliveryman"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}
	status := request.Status
	if status == "" {
		status = request.Deliveryman.Status
	}

	courierID, denied := courierFromToken(c)
	if denied != 0 {
		return courierDenied(c, denied)
	}

	if !courierStatuses[status] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Status inválido"})
	}

	var current models.DeliverySolicitation
	if err := models.DB.Select("id", "status", "delivery_man_name", "delivery_man_status").
		Where("order_id = ? AND delivery_man_id = ?", request.OrderID, courierID).
		First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Pedido nao encontrado ou entregador nao autorizado",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar o status do pedido",
		})
	}
	if current.Status == "CANCELLED" || current.Status == "DENIED" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "O pedido foi cancelado"})
	}
	if !validCourierTransition(current.DeliveryManStatus, status) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Transição de status inválida",
			"from":  current.DeliveryManStatus,
			"to":    status,
		})
	}
	// Sair para a entrega exige o pedido pronto — a mesma trava do botão no
	// app (isSwipeDisabled), agora também no servidor.
	if status == "IN_ROUTE_DELIVERY" && current.DeliveryManStatus != status {
		st := orderStatus(request.OrderID)
		if st == "" {
			st = current.Status
		}
		if st != "DONE" && st != "IN_ROUTE_DELIVERY" {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "O pedido ainda não está pronto para sair",
			})
		}
	}

	// Atualiza somente se o pedido pertence ao entregador autenticado e o
	// status ainda é o lido acima (duas chamadas simultâneas não pulam etapa).
	result := models.DB.Model(&models.DeliverySolicitation{}).
		Where("id = ? AND delivery_man_id = ? AND COALESCE(delivery_man_status, '') = ?",
			current.ID, courierID, current.DeliveryManStatus).
		Updates(map[string]interface{}{
			"delivery_man_status": status,
		})

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar o status do pedido",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "O status do pedido mudou; recarregue",
		})
	}

	log.Printf("[DELIVERY] Order %s: entregador %d em %s", request.OrderID, courierID, status)
	notifyCourierUpdate(request.OrderID, dto.DeliveryManDTO{
		Id: courierID, Name: current.DeliveryManName, Status: status,
	})

	return c.JSON(fiber.Map{
		"message": "Status do pedido atualizado com sucesso",
	})
}
