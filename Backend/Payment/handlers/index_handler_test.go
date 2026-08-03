// Package handlers - index_handler_test.go
// Testes unitarios da rota raiz (GET /) e do health payload.
// Nao requerem MongoDB — testam apenas o handler publico.
package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// buildTestApp monta um app Fiber minimo com as rotas publicas.
// Espelha o registro real feito em main.go para as rotas testadas.
func buildTestApp() *fiber.App {
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(HealthPayload())
	})
	app.Get("/", Index)
	return app
}

// TestIndexHandler verifica que GET / retorna 200 com o indice completo.
func TestIndexHandler(t *testing.T) {
	app := buildTestApp()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req, -1) // -1 = sem timeout
	if err != nil {
		t.Fatalf("falha ao executar GET /: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / retornou status %d, esperado 200", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("resposta nao e JSON valido: %v", err)
	}

	// Campos obrigatorios de identidade
	if body["service"] != "payment" {
		t.Errorf("service = %v, esperado 'payment'", body["service"])
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v, esperado 'ok'", body["status"])
	}
	if body["name"] == nil || body["version"] == nil {
		t.Errorf("campos 'name' e 'version' devem estar presentes")
	}

	// Indice de endpoints deve conter os grupos principais
	endpoints, ok := body["endpoints"].(map[string]interface{})
	if !ok {
		t.Fatalf("campo 'endpoints' ausente ou mal formatado")
	}
	for _, group := range []string{"health", "auth", "payments", "chargebacks", "wallets"} {
		if _, exists := endpoints[group]; !exists {
			t.Errorf("grupo de endpoints '%s' ausente no indice", group)
		}
	}
}

// TestHealthPayload verifica que o payload de saude mantem o contrato
// atual do endpoint GET /health ({status, service}).
func TestHealthPayload(t *testing.T) {
	payload := HealthPayload()
	if payload["status"] != "ok" {
		t.Errorf("HealthPayload status = %v, esperado 'ok'", payload["status"])
	}
	if payload["service"] != "payment" {
		t.Errorf("HealthPayload service = %v, esperado 'payment'", payload["service"])
	}
}

// TestHealthEndpoint verifica que GET /health continua retornando 200
// com o contrato original apos a refatoracao para HealthPayload.
func TestHealthEndpoint(t *testing.T) {
	app := buildTestApp()

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("falha ao executar GET /health: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health retornou status %d, esperado 200", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("resposta nao e JSON valido: %v", err)
	}
	if body["status"] != "ok" || body["service"] != "payment" {
		t.Errorf("contrato do /health quebrado: %v", body)
	}
	if _, exists := body["endpoints"]; exists {
		t.Errorf("/health nao deve expor o indice de endpoints")
	}
}
