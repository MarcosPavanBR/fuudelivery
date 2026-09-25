package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// A carteira "customer" é do cliente: o entregador 5 e o usuário de loja 5
// não leem, recarregam nem debitam a carteira do cliente 5. A recusa vem
// antes do banco (DB nulo aqui).
func TestCarteiraDoCliente_OutroTipoDeContaComMesmoID(t *testing.T) {
	prev := models.DB
	models.DB = nil
	t.Cleanup(func() { models.DB = prev })
	const secret = "carteira-tipo-de-conta"
	t.Setenv("JWT_SECRET", secret)
	sign := func(claims jwt.MapClaims) string {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	rotas := []struct {
		method, route, path, body string
		h                         fiber.Handler
	}{
		{"GET", "/wallets/:user_id", "/wallets/5", "", GetBalance},
		{"POST", "/wallets/topup", "/wallets/topup", `{"user_id":5,"amount":10,"payment_id":"pay-1"}`, TopUp},
		{"POST", "/wallets/deduct", "/wallets/deduct", `{"user_id":5,"amount":10,"order_id":"o-1"}`, DeductFromWallet},
	}
	for nome, tok := range map[string]string{
		"entregador 5 (novo)":   sign(jwt.MapClaims{"id": 5, "account_type": "deliveryman", "phone": "+5511"}),
		"entregador 5 (legado)": sign(jwt.MapClaims{"id": 5, "phone": "+5511"}),
		"usuário de loja 5":     sign(jwt.MapClaims{"id": 5, "role": "user", "account_type": "user", "establishment_id": 5}),
	} {
		for _, r := range rotas {
			app := fiber.New()
			app.Add(r.method, r.route, r.h)
			req := httptest.NewRequest(r.method, r.path, bytes.NewReader([]byte(r.body)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tok)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != 403 {
				t.Errorf("%s em %s %s: got %d, want 403", nome, r.method, r.path, resp.StatusCode)
			}
		}
	}
}
