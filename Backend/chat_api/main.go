package main

import (
	"log"

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

	routes.SetupRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
