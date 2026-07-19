package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/streadway/amqp"

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
	startQueueListener()

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

			if err = c.WriteMessage(mt, msg); err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	routes.SetupRoutes(app)

	app.Listen(":3000")
}

func startQueueListener() {
	dsn := os.Getenv("RABBIT_CONNECTION")
	queueName := os.Getenv("RABBIT_PAYMENT_QUEUE")

	var conn *amqp.Connection
	var err error
	for {
		conn, err = amqp.Dial(dsn)
		if err == nil {
			break
		}
		log.Printf("Erro ao conectar ao servidor de mensagens: %s. Tentando novamente em 5 segundos...", err)
		time.Sleep(5 * time.Second)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Erro ao abrir canal: %s", err)
	}
	defer ch.Close()

	queue, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Erro ao declarar a fila: %s", err)
	}

	for {
		msgs, err := ch.Consume(
			queue.Name,
			"",
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Fatalf("Erro ao registrar o consumidor: %s", err)
		}

		for msg := range msgs {
			var paymentMsg map[string]interface{}
			if err := json.Unmarshal(msg.Body, &paymentMsg); err != nil {
				log.Printf("Erro ao decodificar mensagem: %s", err)
				continue
			}

			log.Printf("Mensagem recebida da fila %s: %s", queueName, string(msg.Body))

			// Notify connected WebSocket clients about payment status update
			clientsMu.Lock()
			for _, client := range clients {
				client.WriteMessage(websocket.TextMessage, msg.Body)
			}
			clientsMu.Unlock()
		}
	}
}
