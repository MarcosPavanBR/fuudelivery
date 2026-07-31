// Package main e o ponto de entrada da Auth API.
//
// Autenticacao JWT, CRUD de usuarios, estabelecimentos,
// delivery-men, clientes e zonas.
//
// Porta padrao: 3000
// Endpoints: POST /users/login, GET /establishments, etc.
package main

import (
	"log"

	"github.com/carloshomar/fuudelivery/pkg/health"
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

	app.Get("/health", func(c *fiber.Ctx) error {
		postgresCheck := health.DatabaseCheck(models.DB)
		return c.JSON(fiber.Map{
			"status":  postgresCheck.Status,
			"service": "auth_api",
			"checks": fiber.Map{
				"postgres": postgresCheck,
			},
		})
	})

	routes.SetupRoutes(app)

	if err := app.Listen(":3000"); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
