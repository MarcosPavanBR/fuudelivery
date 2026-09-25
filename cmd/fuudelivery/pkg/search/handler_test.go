package search

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// A rota é registrada antes de o banco conectar: o handler precisa ler o
// banco a cada requisição. Com o banco ainda nil, 503 — não panic/500.
func TestNewHandler_BancoAindaNaoConectado(t *testing.T) {
	app := fiber.New()
	app.Get("/search", NewHandler(func() *gorm.DB { return nil }))
	resp, err := app.Test(httptest.NewRequest("GET", "/search?q=pizza", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", resp.StatusCode)
	}
}
