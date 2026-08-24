package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"strconv"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	"github.com/carloshomar/fuudelivery/delivery_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

// ============================================================================
// Handlers de solicitações de entrega — CORTE 3 da migração banco-único.
//
// Fonte da verdade: tabela Postgres `delivery_solicitations` (sql/02).
// O MongoDB permanece apenas como dual-write best-effort durante a
// transição: qualquer falha nea é logada mas NÃO falha a request — o
// Mongo não é mais consultado para leitura em nenhum ponto.
//
// Para desligar o Mongo definitivamente: remover os blocos marcados
// com "DUAL-WRITE LEGADO" neste arquivo e em deliveryman.go/reports.go.
// ============================================================================

// dualWriteMongo grava/atualiza a solicitação na collection legada do Mongo.
// Best-effort: erro só aparece no log, nunca quebra o fluxo principal.
func dualWriteMongo(order dto.OrderDTO) {
	if models.MongoDabase == nil {
		return // Mongo não configurado — dual-write desativado
	}
	collection := models.MongoDabase.Collection("solicitations")
	filter := bson.M{"orderid": order.OrderId}
	update := bson.M{"$set": order}
	if _, err := collection.UpdateOne(mongoCtx(), filter, update, options.Update().SetUpsert(true)); err != nil {
		log.Printf("[DUAL-WRITE] Mongo solicitations %s: %v (ignorado)", order.OrderId, err)
	}
}

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

		dualWriteMongo(orderDTO) // DUAL-WRITE LEGADO
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

	dualWriteMongo(orderDTO) // DUAL-WRITE LEGADO
	return nil
}

func HandShakeDeliveryman(c *fiber.Ctx) error {
	var orderDTO dto.OrderDTO
	if err := c.BodyParser(&orderDTO); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}

	var existing models.DeliverySolicitation
	err := models.DB.Where("order_id = ?", orderDTO.OrderId).First(&existing).Error
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

	// Um entregador só pode assumir um pedido ainda sem atribuição, e a
	// atribuição usa SEMPRE a identidade do token (não do body) — evita que
	// um autenticado qualquer assuma entregas em nome de outro.
	tokenCourierID, tErr := middlewares.GetUserIDFromToken(c)
	if tErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token inválido",
		})
	}

	// Claim atômico: UPDATE condicional garante que só UM handshake vence
	// quando dois entregadores aceitam ao mesmo tempo (TOCTOU do read-then-save).
	res := models.DB.Model(&existing).
		Where("id = ? AND delivery_man_id = 0", existing.ID).
		Updates(map[string]interface{}{
			"delivery_man_id":     tokenCourierID,
			"delivery_man_name":   orderDTO.DeliveryMan.Name,
			"delivery_man_status": "IN_ROUTE_COLECT",
		})
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar a solicitação",
		})
	}
	if res.RowsAffected == 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "O deliveryman já foi atribuído a este pedido",
		})
	}
	existing.DeliveryManID = tokenCourierID
	existing.DeliveryManStatus = "IN_ROUTE_COLECT"

	dualWriteMongo(existing.ToDTO()) // DUAL-WRITE LEGADO

	order := existing.ToDTO()
	log.Printf("[DELIVERY] Order %s handshake published", order.OrderId)

	return c.JSON(fiber.Map{
		"message": "Pedido atualizado com sucesso",
	})
}

// GetApprovedSolicitations lista pedidos aprovados/feitos num raio de
// `limitDistance` km das coordenadas informadas (busca do app do entregador).
func GetApprovedSolicitations(c *fiber.Ctx) error {
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

	approvedSolicitations := []dto.OrderDTO{}

	var rows []models.DeliverySolicitation
	// Equivalente ao filtro Mongo antigo:
	//   status IN (APPROVED, DONE) AND deliveryman ausente ou id = 0
	if err := models.DB.
		Where("status IN ?", []string{"APPROVED", "DONE"}).
		Where("delivery_man_id = 0 OR delivery_man_id IS NULL").
		Find(&rows).Error; err != nil {
		log.Printf("[SOLICITATION] Erro ao listar aprovados: %s", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao consultar pedidos",
		})
	}

	for _, row := range rows {
		orderDTO := row.ToDTO()

		// Calcular a distância entre o estabelecimento e as coordenadas fornecidas
		distance := calculateDistance(latitude, longitude, orderDTO.Establishment.Lat, orderDTO.Establishment.Long)

		// Se a distância for menor ou igual ao limite de distância, adiciona a solicitação à lista
		if distance <= limitDist {
			approvedSolicitations = append(approvedSolicitations, orderDTO)
		}
	}

	return c.JSON(approvedSolicitations)
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
