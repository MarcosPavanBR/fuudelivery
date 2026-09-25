package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"strconv"

	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	"github.com/carloshomar/fuudelivery/delivery_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// CreateSolicitation é chamado pela fila do monolito quando um pedido é aprovado.
// Cria (ou atualiza) a solicitação no read-model do motor de despacho.
func CreateSolicitation(msg string, sendMessageToClient func(clientID int64, message []byte) error) error {
	var orderDTO dto.OrderDTO

	if err := json.Unmarshal([]byte(msg), &orderDTO); err != nil {
		log.Printf("Erro ao decodificar a mensagem JSON: %s", err)
		return nil
	}

	var existing models.DeliverySolicitation
	err := models.DB.Where("order_id = ?", orderDTO.OrderId).First(&existing).Error

	if err == nil {
		// Pedido já existe no read-model: atualiza status e preserva o
		// entregador já atribuído (comportamento idêntico ao fluxo antigo).
		log.Printf("Atualizando pedido %s para Status: %s", orderDTO.OrderId, orderDTO.Status)

		existing.Status = orderDTO.Status
		if err := models.DB.Save(&existing).Error; err != nil {
			log.Printf("Erro ao atualizar a solicitação: %s", err)
			return nil
		}

		orderDTO.DeliveryMan = existing.ToDTO().DeliveryMan

		jsonData, _ := json.Marshal(&orderDTO)
		sendMessageToClient(orderDTO.DeliveryMan.Id, jsonData)

		return nil
	}

	// ErrRecordNotFound = fluxo normal de criação; outro erro = problema real.
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Erro ao buscar a solicitação existente: %s", err)
		return nil
	}

	var row models.DeliverySolicitation
	row.FromDTO(orderDTO)
	if err := models.DB.Create(&row).Error; err != nil {
		log.Printf("[SOLICITATION] Failed to insert: %v", err)
		return err
	}

	return nil
}

func HandShakeDeliveryman(c *fiber.Ctx) error {
	// Um entregador só pode assumir um pedido ainda sem atribuição, e a
	// atribuição usa SEMPRE a identidade do token (não do body) — evita que
	// um autenticado qualquer assuma entregas em nome de outro. O token tem
	// de ser de entregador: o id de um cliente ou de uma loja gravado em
	// delivery_man_id apontaria para outro entregador (o de mesmo número).
	tokenCourierID, denied := courierFromToken(c)
	if denied != 0 {
		return courierDenied(c, denied)
	}

	// O app manda {order_id, deliveryman: {...}}; "orderid" (o JSON do
	// OrderDTO) é aceito também. Só "orderid" era lido e todo aceite do app
	// virava 404.
	var request struct {
		OrderID     string             `json:"order_id"`
		OrderIDAlt  string             `json:"orderid"`
		DeliveryMan dto.DeliveryManDTO `json:"deliveryman"`
	}
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}
	orderID := request.OrderID
	if orderID == "" {
		orderID = request.OrderIDAlt
	}

	var existing models.DeliverySolicitation
	err := models.DB.Where("order_id = ?", orderID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Pedido não encontrado",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao consultar o pedido",
		})
	}

	// Claim atômico: UPDATE condicional garante que só UM handshake vence
	// quando dois entregadores aceitam ao mesmo tempo (TOCTOU do read-then-save).
	// Só pedido ainda na fila: cancelado ou finalizado não se aceita.
	res := models.DB.Model(&existing).
		Where("id = ? AND (delivery_man_id = 0 OR delivery_man_id IS NULL) AND status IN ?",
			existing.ID, claimableStatuses).
		Updates(map[string]interface{}{
			"delivery_man_id":     tokenCourierID,
			"delivery_man_name":   request.DeliveryMan.Name,
			"delivery_man_status": "IN_ROUTE_COLECT",
		})
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar a solicitação",
		})
	}
	if res.RowsAffected == 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "O pedido já foi aceito por outro entregador ou não está mais disponível",
		})
	}

	log.Printf("[DELIVERY] Order %s aceito pelo entregador %d", orderID, tokenCourierID)
	notifyCourierUpdate(orderID, dto.DeliveryManDTO{
		Id: tokenCourierID, Name: request.DeliveryMan.Name, Status: "IN_ROUTE_COLECT",
	})

	return c.JSON(fiber.Map{
		"message": "Pedido atualizado com sucesso",
	})
}

// GetApprovedSolicitations lista pedidos aprovados/feitos num raio de
// `limitDistance` km das coordenadas informadas (busca do app do entregador).
//
// Só entregador. A checagem antiga era role == "delivery_man", mas o token de
// entregador não tem role (GenerateJWTDeliveryMan) — todo entregador levava
// 403 e o app não mostrava pedido disponível nenhum.
func GetApprovedSolicitations(c *fiber.Ctx) error {
	if _, denied := courierFromToken(c); denied != 0 {
		return courierDenied(c, denied)
	}

	lat := c.Query("latitude")
	long := c.Query("longitude")
	limitDistance := c.Query("limitDistance")

	latitude, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid latitude parameter"})
	}

	longitude, err := strconv.ParseFloat(long, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid longitude parameter"})
	}

	limitDist, err := strconv.ParseFloat(limitDistance, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid limitDistance parameter"})
	}

	var rows []models.DeliverySolicitation
	// Pedidos na fila (aprovados, em preparo ou prontos) sem entregador.
	if err := models.DB.
		Where("status IN ?", claimableStatuses).
		Where("delivery_man_id = 0 OR delivery_man_id IS NULL").
		Find(&rows).Error; err != nil {
		log.Printf("[SOLICITATION] Erro ao listar aprovados: %s", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao consultar pedidos",
		})
	}

	nearby := make([]models.DeliverySolicitation, 0, len(rows))
	for _, row := range rows {
		// Distância entre o estabelecimento e o entregador.
		if calculateDistance(latitude, longitude, row.EstablishmentLat, row.EstablishmentLong) <= limitDist {
			nearby = append(nearby, row)
		}
	}

	return c.JSON(courierOrderViews(nearby, customerNone))
}

// Função para calcular a distância entre dois pontos usando a fórmula de Haversine (https://pt.wikipedia.org/wiki/F%C3%B3rmula_de_haversine)
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371 // Raio da Terra em quilômetros

	// Converte as coordenadas de graus para radianos
	lat1Rad := degreesToRadians(lat1)
	lon1Rad := degreesToRadians(lon1)
	lat2Rad := degreesToRadians(lat2)
	lon2Rad := degreesToRadians(lon2)

	// Calcula as diferenças de coordenadas
	deltaLat := lat2Rad - lat1Rad
	deltaLon := lon2Rad - lon1Rad

	// Calcula as distância usando a Haversine
	a := math.Pow(math.Sin(deltaLat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(deltaLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	return distance
}

// Função para converter graus em radianos
func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
