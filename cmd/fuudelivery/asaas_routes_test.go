package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/gofiber/fiber/v2"
)

// As rotas Asaas legadas aceitam valor, destino e split livres do corpo:
// só admin (SEG-04). Antes, qualquer cliente logado criava cobrança e
// subconta na conta Asaas da plataforma.
func TestAsaasRoutes_SoAdmin(t *testing.T) {
	t.Setenv("JWT_SECRET", "segredo-teste-asaas")
	t.Setenv("ASAAS_API_KEY", "")
	app := fiber.New()
	setupPaymentRoutes(app, gateway.NewRouter())

	cliente := createTestJWT(t, map[string]interface{}{"id": float64(1), "role": "client"})
	for _, path := range []string{"/payments/asaas/payment/split", "/payments/asaas/wallet/create"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(
			`{"amount":100,"establishment_wallet_id":"w-atacante","establishment_split_pct":100,"name":"x","cpf_cnpj":"1","email":"a@b.c"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cliente)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 403 {
			t.Errorf("%s como cliente: got %d, want 403", path, resp.StatusCode)
		}
	}
}
