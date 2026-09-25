package main

// Middlewares de autenticação/autorização das rotas e a regra de CORS
// para origens locais de desenvolvimento.

import (
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"

	// Models (database initialization)

	// Handlers

	// Middleware
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	// Dispatch engine
	// Batch expiry
	// Queue + Health + Upload + Metrics + Search
)

func protectedRoute(c *fiber.Ctx) error {
	_, err := middlewares.ValidateJWT(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	return c.Next()
}

func adminRequired(c *fiber.Ctx) error {
	_, err := middlewares.ValidateJWT(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	role, err := middlewares.GetUserRoleFromToken(c)
	if err != nil || role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}
	return c.Next()
}

// isLocalDevOrigin libera origens de desenvolvimento local
// (localhost / 127.0.0.1 / ::1 em qualquer porta). O Fiber não suporta
// wildcard de porta no AllowOrigins, então o check é programático;
// quando retorna true o middleware ecoa o origin na resposta
// (compatível com AllowCredentials: true).
func isLocalDevOrigin(origin string) bool {
	// Em PRODUÇÃO devolve sempre false: localhost-any-port com credentials
	// permitia qualquer app local do usuário autenticar contra a API.
	if origin == "" {
		return false
	}
	if os.Getenv("GO_ENV") == "production" {
		return false
	}
	if os.Getenv("GO_ENV") == "" {
		log.Println("[CORS] WARNING: GO_ENV não definido — assumindo development (localhost permitido)")
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
