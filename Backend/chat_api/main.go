// Package main e o ponto de entrada da Chat API.
//
// Chat em tempo real entre cliente, restaurante e entregador.
// Persistencia em MongoDB, WebSocket para mensagens.
//
// Porta padrao: 3000
// Endpoints: GET /chat/messages/:orderId, POST /chat/message
package main

import (
	"log"

	"github.com/carloshomar/fuudelivery/pkg/health"
	"github.com/carloshomar/vercardapio/chat_api/app/models"
	"github.com/carloshomar/vercardapio/chat_api/app/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	models.ConnectMongoDatabase()

	app := fiber.New()

	app.Get("/health", func(c *fiber.Ctx) error {
		mongoCheck := health.MongoCheck(models.MongoClient)
		return c.JSON(fiber.Map{
			"status":  mongoCheck.Status,
			"service": "chat_api",
			"checks": fiber.Map{
				"mongodb": mongoCheck,
			},
		})
	})

	routes.SetupRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
