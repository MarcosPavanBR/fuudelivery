// Package main e o ponto de entrada da Payment API.
//
// Gateway de pagamento que processa PIX e cartao via AbacatePay.
// Webhook de confirmacao, split de pagamento e fila Redis.
//
// Porta padrao: 3000
// Endpoints: POST /payments/pix/generate, POST /payments/webhook, etc.
package main

import (
	"encoding/json"
	"log"
	"strconv"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"

	"github.com/carloshomar/vercardapio/payment_api/app/models"
	"github.com/carloshomar/vercardapio/payment_api/app/routes"
)

var clients = make(map[int64]*websocket.Conn)
var clientsMu sync.Mutex

func sendMessageToClient(clientID int64, message []byte) error {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if client, ok := clients[clientID]; ok {
		return client.WriteMessage(websocket.TextMessage, message)
	}
	log.Printf("Enviando socket para clientID %d: %s", clientID, string(message))
	return nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Erro ao carregar arquivo .env")
	}

	go startHTTPServer()

	// NOTA: Fila RabbitMQ removida. O monolito gerencia filas via Redis.
	log.Println("[PAYMENT_API] RabbitMQ removido — usando HTTP direto via monolito")

	<-make(chan struct{})
}

func startHTTPServer() {
	app := fiber.New()
	models.ConnectMongoDatabase()

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
		clientIDStr := c.Params("id")
		clientID, _ := strconv.ParseInt(clientIDStr, 10, 64)

		clientsMu.Lock()
		clients[clientID] = c
		clientsMu.Unlock()

		defer func() {
			clientsMu.Lock()
			delete(clients, clientID)
			clientsMu.Unlock()
		}()

		var (
			mt  int
			msg []byte
			err error
		)

		for {
			if mt, msg, err = c.ReadMessage(); err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", msg)

			// Echo message back to client
			if err = c.WriteMessage(mt, msg); err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	app.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok", "service": "payment_api"}) })

	routes.SetupRoutes(app)

	app.Listen(":3000")
}

// processPaymentMessage — stub para compatibilidade com testes.
func processPaymentMessage(msgBody []byte) {
	var paymentMsg map[string]interface{}
	if err := json.Unmarshal(msgBody, &paymentMsg); err != nil {
		log.Printf("Erro ao decodificar mensagem: %s", err)
		return
	}

	log.Printf("Mensagem de pagamento processada (RabbitMQ removido): %s", string(msgBody))

	// Notify connected WebSocket clients
	clientsMu.Lock()
	for _, client := range clients {
		client.WriteMessage(websocket.TextMessage, msgBody)
	}
	clientsMu.Unlock()
}
