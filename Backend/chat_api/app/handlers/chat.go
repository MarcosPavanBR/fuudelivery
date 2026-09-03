package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/carloshomar/fuudelivery/chat_api/app/dto"
	"github.com/carloshomar/fuudelivery/chat_api/app/models"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

type Room struct {
	Clients map[*websocket.Conn]*ClientInfo
	Mu      sync.Mutex
}

type ClientInfo struct {
	UserID   int64
	UserType string
	// conn é o wrapper com mutex de escrita: o broadcast (goroutine de OUTRO
	// cliente) e o ack "message_sent" (read-loop do próprio dono) escrevem no
	// mesmo *websocket.Conn a partir de goroutines diferentes — sem
	// serialização dá "concurrent write to websocket connection".
	conn *safeConn
}

// safeConn serializa escritas em UMA conexão WebSocket (espelho do wrapper de
// cmd/fuudelivery/main.go; duplicado aqui para não criar dependência entre
// módulos Go). gorilla/fasthttp ws não permite WriteMessage concorrente no
// mesmo conn.
type safeConn struct {
	conn wsMessageWriter
	mu   sync.Mutex
}

// wsMessageWriter é o subconjunto de *websocket.Conn usado pelo safeConn —
// existe para os testes injetarem um writer falso sob -race.
type wsMessageWriter interface {
	WriteMessage(messageType int, data []byte) error
}

func (s *safeConn) WriteMessage(messageType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(messageType, data)
}

var (
	rooms   = make(map[string]*Room)
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
	empty := len(room.Clients) == 0
	room.Mu.Unlock()

	// Clean up empty rooms to prevent memory leak.
	if empty {
		roomsMu.Lock()
		delete(rooms, orderID)
		roomsMu.Unlock()
	}
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
			if err := room.Clients[client].conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Erro ao enviar mensagem para cliente: %v", err)
				client.Close()
				delete(room.Clients, client)
			}
		}
	}
}

func HandleChatWebSocket(c *websocket.Conn) {
	orderID := c.Params("orderId")
	userIDStr := c.Params("userId")
	userType := c.Params("userType")

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		log.Printf("Erro ao parsear userID: %v", err)
		return
	}

	room := getOrCreateRoom(orderID)

	sc := &safeConn{conn: c}
	room.Mu.Lock()
	room.Clients[c] = &ClientInfo{
		UserID:   userID,
		UserType: userType,
		conn:     sc,
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
			sc.WriteMessage(websocket.TextMessage, responseBytes)

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
	now := time.Now()
	msg := models.ChatMessage{
		OrderID:     req.OrderID,
		SenderID:    req.SenderID,
		SenderType:  req.SenderType,
		SenderName:  req.SenderName,
		Message:     req.Message,
		MessageType: req.MessageType,
		ImageURL:    req.ImageURL,
		CreatedAt:   now,
	}

	if msg.MessageType == "" {
		msg.MessageType = "text"
	}

	// ── CORTE 2 (banco-único): escrita PRIMÁRIA em Postgres ─────────────
	// O ID volta preenchido pelo BIGSERIAL e é o que vai para o cliente.
	if models.DB == nil {
		return nil, fmt.Errorf("Postgres indisponível (chat_api models.DB nulo)")
	}
	if err := models.DB.Create(&msg).Error; err != nil {
		return nil, err
	}

	return &msg, nil
}

func GetMessages(c *fiber.Ctx) error {
	orderID := c.Params("orderId")
	if orderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "orderId é obrigatório"})
	}

	// CORTE 2: leitura 100% Postgres, ordenada por created_at (índice da tabela).
	if models.DB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Banco indisponível"})
	}

	var messages []models.ChatMessage
	if err := models.DB.
		Where("order_id = ?", orderID).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao buscar mensagens"})
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
	userIDStr := c.Params("userId")

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "userId inválido"})
	}

	now := time.Now()

	// CORTE 2: marca como lidas em Postgres todas as mensagens do pedido
	// que NÃO são deste usuário e ainda não foram lidas.
	if models.DB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Banco indisponível"})
	}
	if err := models.DB.Model(&models.ChatMessage{}).
		Where("order_id = ? AND sender_id <> ? AND read_at IS NULL", orderID, userID).
		Update("read_at", now).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao marcar como lido"})
	}

	return c.JSON(fiber.Map{"message": "Mensagens marcadas como lidas"})
}
