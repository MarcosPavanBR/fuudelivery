package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

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
		// None: frontend e API vivem em subdomínios .onrender.com diferentes
		// (cross-site) — mesmo motivo do fix em session_handler.go.
		SameSite: "none",
		Path:     "/",
		// Igual ao refresh_token (30 dias). Antes eram 24h: o cookie de CSRF
		// morria muito antes da sessão e, como o middleware liberava quando
		// ele faltava, a proteção se desligava sozinha um dia após o login.
		// O middleware agora rejeita nesse caso, então uma validade menor que
		// a da sessão só geraria 403 espúrio.
		MaxAge: int(30 * 24 * time.Hour.Seconds()),
	})

	return c.JSON(fiber.Map{
		"csrf_token": csrfToken,
	})
}
