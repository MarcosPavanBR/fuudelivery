package handlers

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	"github.com/carloshomar/fuudelivery/delivery_api/app/models"
)

func solicitationCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func CreateSolicitation(msg string, sendMessageToClient func(clientID int64, message []byte) error) error {
	var orderDTO dto.OrderDTO

	err := json.Unmarshal([]byte(msg), &orderDTO)
	if err != nil {
		log.Printf("Erro ao decodificar a mensagem JSON: %s", err)
		return nil
	}

	ctx, cancel := solicitationCtx()
	defer cancel()

	stored, existed, err := models.UpsertSolicitation(ctx, orderDTO)
	if err != nil {
		log.Printf("[SOLICITATION] Failed to upsert %s: %v", orderDTO.OrderId, err)
		return err
	}

	if existed {
		// Preserva o entregador já atribuído e notifica o app dele
		orderDTO.DeliveryMan = stored.DeliveryMan

		log.Printf("Atualizando pedido %s", orderDTO.OrderId)
		log.Printf("Para Status: %s", orderDTO.Status)

		jsonData, _ := json.Marshal(&orderDTO)
		sendMessageToClient(orderDTO.DeliveryMan.Id, jsonData)
	}

	return nil
}

func HandShakeDeliveryman(c *fiber.Ctx) error {
	var orderDTO dto.OrderDTO
	if err := c.BodyParser(&orderDTO); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}

	ctx, cancel := solicitationCtx()
	defer cancel()

	existingOrder, err := models.GetSolicitationByOrderID(ctx, orderDTO.OrderId)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Pedido não encontrado",
		})
	}

	if existingOrder.DeliveryMan != (dto.DeliveryManDTO{}) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "O deliveryman já foi atribuído a este pedido",
		})
	}

	orderDTO.DeliveryMan.Status = "IN_ROUTE_COLECT"
	if err := models.UpdateSolicitationDeliveryMan(ctx, orderDTO.OrderId, orderDTO.DeliveryMan); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar a solicitação",
		})
	}

	order, err := GetOrderByID(orderDTO.OrderId)
	if err != nil || order == nil {
		log.Printf("[SOLICITATION] Order %s not found after handshake", orderDTO.OrderId)
		return c.JSON(fiber.Map{"message": "Pedido atualizado com sucesso"})
	}

	// RabbitMQ removido — fila gerenciada pelo monolito via Redis
	log.Printf("[DELIVERY] Order %s handshake published", order.OrderId)

	return c.JSON(fiber.Map{
		"message": "Pedido atualizado com sucesso",
	})
}

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

	ctx, cancel := solicitationCtx()
	defer cancel()

	approvedSolicitations, err := models.FindApprovedSolicitations(ctx)
	if err != nil {
		log.Printf("[SOLICITATION] GetApprovedSolicitations: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao consultar solicitações"})
	}

	// Filtra pela distância entre o estabelecimento e as coordenadas fornecidas
	result := make([]dto.OrderDTO, 0, len(approvedSolicitations))
	for _, order := range approvedSolicitations {
		distance := calculateDistance(latitude, longitude, order.Establishment.Lat, order.Establishment.Long)
		if distance <= limitDist {
			result = append(result, order)
		}
	}

	return c.JSON(result)
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

	// Calcula as distância usando a fórmula de Haversine
	a := math.Pow(math.Sin(deltaLat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(deltaLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	return distance
}

// Função para converter graus em radianos
func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
