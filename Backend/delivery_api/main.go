package main

import (
	"log"
	"strconv"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"

	"github.com/carloshomar/vercardapio/delivery_api/app/handlers"
	"github.com/carloshomar/vercardapio/delivery_api/app/models"
	"github.com/carloshomar/vercardapio/delivery_api/app/routes"
)

var clients = make(map[int64]*websocket.Conn)
var clientsMu sync.Mutex

func sendMessageToClient(clientID int64, message []byte) error {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if client, ok := clients[clientID]; ok {
		return client.WriteMessage(websocket.TextMessage, message)
	}
	log.Println("Enviando socket")
	log.Printf("Id: %d", clientID)
	log.Println(string(message))
	return nil
}

func main() {
	// Carregar variáveis de ambiente
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Erro ao carregar arquivo .env")
	}

	// Iniciar o servidor HTTP
	startHTTPServer()

	// NOTA: Fila RabbitMQ removida. O monolito gerencia filas via Redis.
	log.Println("[DELIVERY_API] RabbitMQ removido — usando HTTP direto via monolito")

	// Mantém a aplicação em execução indefinidamente
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
		log.Println(c.Locals("allowed"))
		log.Println(c.Params("id"))
		log.Println(c.Query("v"))
		log.Println(c.Cookies("session"))

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

	routes.SetupRoutes(app, sendMessageToClient)

	// Iniciar o servidor
	app.Listen(":3000")
}

// CreateSolicitation exposta para compatibilidade — delega ao handler real.
func CreateSolicitation(body string, sendFn func(int64, []byte) error) {
	handlers.CreateSolicitation(body, sendFn)
}
