package main

import (
	"log"

	"github.com/carloshomar/vercardapio/auth_api/app/models"
	"github.com/carloshomar/vercardapio/auth_api/app/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	models.ConnectDatabase()

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://fuudelivery-web.onrender.com,https://fuudelivery-admin-lv7f.onrender.com,https://fuudelivery-payment-panel.onrender.com,http://localhost:3000,http://localhost:3001",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	routes.SetupRoutes(app)

	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
