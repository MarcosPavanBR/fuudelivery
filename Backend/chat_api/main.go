package main

import (
	"log"

	"github.com/carloshomar/vercardapio/app/models"
	"github.com/carloshomar/vercardapio/app/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Erro ao carregar arquivo .env")
	}

	models.ConnectMongoDatabase()

	app := fiber.New()

	routes.SetupRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
