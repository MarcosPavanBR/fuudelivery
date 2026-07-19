package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/carloshomar/vercardapio/orders_api/app/dto"
	"github.com/carloshomar/vercardapio/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreateOrder(c *fiber.Ctx, sendMessageToClient func(clientID int64, message []byte) error) error {
	var request dto.RequestPayload

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}

	if request.ScheduledAt != nil && !request.ScheduledAt.IsZero() {
		request.Status = "SCHEDULED"
		request.IsScheduled = true
	} else {
		request.Status = "AWAIT_APPROVE"
		now := time.Now()
		request.ScheduledAt = &now
	}

	if !request.IsScheduled {
		isOpen, err := checkEstablishmentOpen(request.EstablishmentId)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Erro ao verificar horário do estabelecimento",
			})
		}
		if !isOpen {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Estabelecimento fechado neste horário",
			})
		}
	}

	establishment, err := GetEstablishment(request.EstablishmentId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao obter detalhes do estabelecimento",
		})
	}

	request.Establishment = *establishment

	collection := models.MongoDabase.Collection("orders")

	insertResult, err := collection.InsertOne(context.Background(), request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao inserir a ordem no banco de dados",
		})
	}

	jsonData, _ := json.Marshal(request)
	if err := sendMessageToClient(request.EstablishmentId, jsonData); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"message": "Ordem criada com sucesso",
		"orderId": insertResult.InsertedID,
	})
}

func GetEstablishment(establishmentID int64) (*dto.Establishment, error) {
	urlEnv := os.Getenv("URL_GET_ESTABLISHMENT_ID")
	if urlEnv == "" {
		panic("URL_GET_ESTABLISHMENT_ID não configurado.")
	}

	url := fmt.Sprintf(urlEnv, establishmentID)
	log.Println(url)
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API retornou status não OK: %d", response.StatusCode)
	}

	var establishmentDTO dto.Establishment
	if err := json.NewDecoder(response.Body).Decode(&establishmentDTO); err != nil {
		return nil, err
	}

	return &establishmentDTO, nil
}

func checkEstablishmentOpen(establishmentID int64) (bool, error) {
	urlEnv := os.Getenv("URL_CHECK_ESTABLISHMENT_OPEN")
	if urlEnv == "" {
		urlEnv = "http://auth-api:3000/api/auth/establishments/%d/is-open"
	}

	url := fmt.Sprintf(urlEnv, establishmentID)
	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var result struct {
		IsOpen bool `json:"is_open"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	return result.IsOpen, nil
}

func UpdateOrderStatus(c *fiber.Ctx, sendMessageToClient func(clientID int64, message []byte) error) error {
	var requestBody dto.UpdateOrderStatusRequest
	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}

	orderID, err := primitive.ObjectIDFromHex(requestBody.ID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID inválido",
		})
	}

	filter := bson.M{"_id": orderID}

	collection := models.MongoDabase.Collection("orders")

	var order dto.RequestPayload
	if err := collection.FindOne(context.Background(), filter).Decode(&order); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Pedido não encontrado",
		})
	}
	if requestBody.Status != "REQUEST_APPROVE" {
		order.OrderId = orderID.Hex()
		order.Status = requestBody.Status
		orderBytes, err := json.Marshal(&order)
		if err == nil {
			PublishMessage(orderBytes)
		}
	}

	jsonData, _ := json.Marshal(requestBody)

	if err := sendMessageToClient(order.EstablishmentId, jsonData); err != nil {
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"status": requestBody.Status,
		},
		"$currentDate": bson.M{
			"lastModified": true,
		},
	}

	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar ordem no banco de dados",
		})
	}

	if updateResult.ModifiedCount == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Nenhum pedido encontrado com o ID fornecido",
		})
	}

	go sendStatusPushNotification(order, requestBody.Status)

	if requestBody.Status == "DONE" {
		code := generateSecureCode()
		collection.UpdateOne(context.Background(), filter, bson.M{
			"$set": bson.M{
				"pickup_code":              code,
				"pickup_code_generated_at": time.Now(),
			},
		})
	}

	return c.JSON(fiber.Map{
		"message": "Status do pedido atualizado com sucesso",
	})
}

func sendStatusPushNotification(order dto.RequestPayload, status string) {
	pushTokensCollection := models.MongoDabase.Collection("push_tokens")

	statusMessages := map[string]string{
		"APPROVED":         "Seu pedido foi aprovado e está sendo preparado!",
		"DONE":             "Seu pedido está pronto e saiu para entrega!",
		"IN_ROUTE_DELIVERY": "Seu pedido está a caminho!",
		"FINISHED":         "Seu pedido foi entregue! Bom apetite!",
		"CANCELLED":        "Seu pedido foi cancelado.",
		"SCHEDULED":        "Seu pedido foi agendado com sucesso!",
	}

	msg, ok := statusMessages[status]
	if !ok {
		return
	}

	title := "Atualização do Pedido"
	if status == "FINISHED" {
		title = "Pedido Entregue"
	} else if status == "CANCELLED" {
		title = "Pedido Cancelado"
	}

	userPhone := order.User.Phone

	cursor, err := pushTokensCollection.Find(context.Background(), bson.M{"user_phone": userPhone})
	if err != nil {
		log.Printf("Erro ao buscar push tokens: %v", err)
		return
	}
	defer cursor.Close(context.Background())

	var tokens []struct {
		PushToken string `bson:"push_token"`
	}
	if err := cursor.All(context.Background(), &tokens); err != nil {
		log.Printf("Erro ao decodificar push tokens: %v", err)
		return
	}

	for _, t := range tokens {
		if err := SendPushNotification(t.PushToken, title, msg, map[string]interface{}{
			"order_id": order.OrderId,
			"status":   status,
			"type":     "status_update",
		}); err != nil {
			log.Printf("Erro ao enviar push: %v", err)
		}
	}
}

func ListOrdersByEstablishmentID(c *fiber.Ctx) error {
	establishmentID := c.Params("establishmentId")

	if establishmentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID do estabelecimento não fornecido",
		})
	}

	establishmentIDInt, err := strconv.Atoi(establishmentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID do estabelecimento inválido",
		})
	}

	filter := bson.M{"establishmentid": establishmentIDInt}

	collection := models.MongoDabase.Collection("orders")

	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Falha ao buscar pedidos",
		})
	}
	defer cursor.Close(context.Background())

	var orders []map[string]interface{}
	if err := cursor.All(context.Background(), &orders); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Falha ao decodificar resultados",
		})
	}

	var formattedOrders []map[string]interface{}
	for _, order := range orders {
		formattedOrder := make(map[string]interface{})
		for key, value := range order {
			formattedOrder[key] = value
		}
		formattedOrders = append(formattedOrders, formattedOrder)
	}

	return c.JSON(formattedOrders)
}

func ListOrdersByEstablishmentIDAndPhone(c *fiber.Ctx) error {
	establishmentID := c.Params("establishmentId")
	phoneNumberEncoded := c.Params("phoneNumber")

	if establishmentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID do estabelecimento não fornecido"})
	}

	establishmentIDInt, err := strconv.Atoi(establishmentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID do estabelecimento inválido"})
	}

	phoneNumber, err := url.QueryUnescape(phoneNumberEncoded)
	filter := bson.M{
		"establishmentid": establishmentIDInt,
		"user.phone":      phoneNumber,
	}

	collection := models.MongoDabase.Collection("orders")
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao buscar pedidos"})
	}
	defer cursor.Close(context.Background())

	var orders []map[string]interface{}
	if err := cursor.All(context.Background(), &orders); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao decodificar resultados"})
	}

	return c.JSON(orders)
}

func ListOrdersByPhone(c *fiber.Ctx) error {
	phoneNumberEncoded := c.Params("phone")

	phoneNumber, err := url.QueryUnescape(phoneNumberEncoded)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao decodificar número de telefone"})
	}

	filter := bson.M{
		"user.phone": phoneNumber,
	}

	collection := models.MongoDabase.Collection("orders")
	options := options.Find().SetSort(bson.D{{"lastModified", -1}})
	cursor, err := collection.Find(context.Background(), filter, options)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao buscar pedidos"})
	}
	defer cursor.Close(context.Background())

	var orders []map[string]interface{}
	if err := cursor.All(context.Background(), &orders); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao decodificar resultados"})
	}

	return c.JSON(orders)
}
