package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// GetCSRFToken gera um novo token CSRF e define o cookie.
// Usado pelo frontend antes de mutações (POST/PUT/DELETE).
func GetCSRFToken(c *fiber.Ctx) error {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate csrf token"})
	}
	csrfToken := hex.EncodeToString(token)

	c.Cookie(&fiber.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		HTTPOnly: false, // frontend precisa ler via JS
		Secure:   true,  // HTTPS only em produção
		SameSite: "strict",
		Path:     "/",
		MaxAge:   86400, // 24h
	})

	return c.JSON(fiber.Map{
		"csrf_token": csrfToken,
	})
}
