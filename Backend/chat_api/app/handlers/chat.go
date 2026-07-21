package handlers

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/carloshomar/vercardapio/chat_api/app/dto"
	"github.com/carloshomar/vercardapio/chat_api/app/models"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Room struct {
	Clients map[*websocket.Conn]*ClientInfo
	Mu      sync.Mutex
}

type ClientInfo struct {
	UserID   int64
	UserType string
}

var (
	rooms = make(map[string]*Room)
	roomsMu sync.Mutex
)

func getOrCreateRoom(orderID string) *Room {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	if room, ok := rooms[orderID]; ok {
		return room
	}

	room := &Room{
		Clients: make(map[*websocket.Conn]*ClientInfo),
	}
	rooms[orderID] = room
	return room
}

func removeClientFromRoom(orderID string, conn *websocket.Conn) {
	roomsMu.Lock()
	room, ok := rooms[orderID]
	roomsMu.Unlock()

	if !ok {
		return
	}

	room.Mu.Lock()
	delete(room.Clients, conn)
	room.Mu.Unlock()
}

func broadcastToRoom(orderID string, sender *websocket.Conn, message []byte) {
	roomsMu.Lock()
	room, ok := rooms[orderID]
	roomsMu.Unlock()

	if !ok {
		return
	}

	room.Mu.Lock()
	defer room.Mu.Unlock()

	for client := range room.Clients {
		if client != sender {
			if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Erro ao enviar mensagem para cliente: %v", err)
				client.Close()
				delete(room.Clients, client)
			}
		}
	}
}

func validateWSJWT(tokenString string) (jwt.MapClaims, error) {
	if len(tokenString) > 7 {
		tokenString = tokenString[7:]
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid token claims")
	}
	return claims, nil
}

func HandleChatWebSocket(c *websocket.Conn) {
	orderID := c.Params("orderId")
	userIDStr := c.Params("userId")
	userType := c.Params("userType")

	// Validate JWT from query param
	token := c.Query("token")
	if token == "" {
		log.Printf("[WS-CHAT] Rejected: no token provided for order %s", orderID)
		c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Authentication required"}}`))
		return
	}
	claims, err := validateWSJWT(token)
	if err != nil {
		log.Printf("[WS-CHAT] Rejected: invalid token for order %s: %v", orderID, err)
		c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid token"}}`))
		return
	}

	// Verify userId matches token
	tokenUserID, _ := claims["id"].(float64)
	urlUserID, _ := strconv.ParseInt(userIDStr, 10, 64)
	if int64(tokenUserID) != urlUserID {
		log.Printf("[WS-CHAT] Rejected: userId mismatch token=%d url=%d", int64(tokenUserID), urlUserID)
		c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"User ID mismatch"}}`))
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		log.Printf("Erro ao parsear userID: %v", err)
		return
	}

	room := getOrCreateRoom(orderID)

	room.Mu.Lock()
	room.Clients[c] = &ClientInfo{
		UserID:   userID,
		UserType: userType,
	}
	room.Mu.Unlock()

	defer func() {
		removeClientFromRoom(orderID, c)
		c.Close()
	}()

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			log.Printf("Erro ao ler mensagem: %v", err)
			break
		}

		var wsMsg dto.ChatWebSocketMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			log.Printf("Erro ao decodificar mensagem: %v", err)
			continue
		}

		switch wsMsg.Type {
		case "message":
			payloadBytes, _ := json.Marshal(wsMsg.Payload)
			var msgReq dto.ChatMessageRequest
			if err := json.Unmarshal(payloadBytes, &msgReq); err != nil {
				log.Printf("Erro ao decodificar payload: %v", err)
				continue
			}

			msgReq.SenderID = userID
			msgReq.SenderType = userType

			savedMsg, err := saveMessage(msgReq)
			if err != nil {
				log.Printf("Erro ao salvar mensagem: %v", err)
				continue
			}

			broadcastBytes, _ := json.Marshal(map[string]interface{}{
				"type":    "new_message",
				"payload": savedMsg,
			})

			broadcastToRoom(orderID, c, broadcastBytes)

			responseBytes, _ := json.Marshal(map[string]interface{}{
				"type":    "message_sent",
				"payload": savedMsg,
			})
			c.WriteMessage(websocket.TextMessage, responseBytes)

		case "typing":
			broadcastBytes, _ := json.Marshal(map[string]interface{}{
				"type": "typing",
				"payload": map[string]interface{}{
					"sender_id":   userID,
					"sender_type": userType,
				},
			})
			broadcastToRoom(orderID, c, broadcastBytes)
		}
	}
}

func saveMessage(req dto.ChatMessageRequest) (*models.ChatMessage, error) {
	collection := models.MongoDabase.Collection("chat_messages")

	msg := models.ChatMessage{
		ID:          primitive.NewObjectID(),
		OrderID:     req.OrderID,
		SenderID:    req.SenderID,
		SenderType:  req.SenderType,
		SenderName:  req.SenderName,
		Message:     req.Message,
		MessageType: req.MessageType,
		ImageURL:    req.ImageURL,
		CreatedAt:   time.Now(),
	}

	if msg.MessageType == "" {
		msg.MessageType = "text"
	}

	_, err := collection.InsertOne(context.Background(), msg)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

func GetMessages(c *fiber.Ctx) error {
	orderID := c.Params("orderId")
	if orderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "orderId é obrigatório"})
	}

	collection := models.MongoDabase.Collection("chat_messages")

	filter := bson.M{"order_id": orderID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := collection.Find(context.Background(), filter, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao buscar mensagens"})
	}
	defer cursor.Close(context.Background())

	var messages []models.ChatMessage
	if err := cursor.All(context.Background(), &messages); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao decodificar mensagens"})
	}

	return c.JSON(messages)
}

func SendMessage(c *fiber.Ctx) error {
	var req dto.ChatMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	msg, err := saveMessage(req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao salvar mensagem"})
	}

	broadcastBytes, _ := json.Marshal(map[string]interface{}{
		"type":    "new_message",
		"payload": msg,
	})

	go broadcastToRoom(req.OrderID, nil, broadcastBytes)

	return c.JSON(msg)
}

func MarkAsRead(c *fiber.Ctx) error {
	orderID := c.Params("orderId")

	// Use userId from JWT, not from URL (prevents IDOR)
	tokenString := c.Get("Authorization")
	if len(tokenString) > 7 {
		tokenString = tokenString[7:]
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	claims := token.Claims.(jwt.MapClaims)
	userIDFloat, ok := claims["id"].(float64)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User ID not in token"})
	}
	userID := int64(userIDFloat)

	now := time.Now()

	collection := models.MongoDabase.Collection("chat_messages")
	filter := bson.M{
		"order_id":  orderID,
		"sender_id": bson.M{"$ne": userID},
		"read_at":   nil,
	}
	update := bson.M{"$set": bson.M{"read_at": now}}

	_, err = collection.UpdateMany(context.Background(), filter, update)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao marcar como lido"})
	}

	return c.JSON(fiber.Map{"message": "Mensagens marcadas como lidas"})
}
