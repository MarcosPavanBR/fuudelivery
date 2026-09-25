package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const courierTestSecret = "segredo-de-teste-do-entregador"

func courierToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	claims["exp"] = time.Now().Add(time.Hour).Unix()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(courierTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Rotas do entregador barram quem não é entregador ANTES de tocar no banco
// (os testes rodam sem DB): o cliente 5 e a loja 5 não agem como o
// entregador 5. O entregador passa da autorização (o 400 é da validação que
// vem depois, sem corpo/parâmetros).
func TestRotasDoEntregador_ExigemContaDeEntregador(t *testing.T) {
	t.Setenv("JWT_SECRET", courierTestSecret)
	h := newTestDispatchHandler()
	rotas := []struct {
		method, path string
		handler      fiber.Handler
		body         string
		okStatus     int // o que o entregador recebe (passou da autorização)
	}{
		{"GET", "/solicitation-orders", GetApprovedSolicitations, "", 400},
		{"PUT", "/solicitation-orders/hand-shake", HandShakeDeliveryman, "bad", 400},
		{"POST", "/deliveryman/status", func(c *fiber.Ctx) error { return UpdateOrderStatusByDeliverymanID(c, nil) }, `{"order_id":"x","status":"NOPE"}`, 400},
		{"POST", "/dispatch/location", h.UpdateLocation, `{"lat":0,"lng":0}`, 400},
		{"POST", "/dispatch/status", h.SetCourierStatus, `{"status":"voando"}`, 400},
	}
	intrusos := map[string]string{
		"cliente 5 (novo)":   courierToken(t, jwt.MapClaims{"id": 5, "role": "client", "account_type": "client"}),
		"cliente 5 (legado)": courierToken(t, jwt.MapClaims{"id": 5, "role": "client"}),
		"loja 5":             courierToken(t, jwt.MapClaims{"id": 5, "role": "user", "account_type": "user", "establishment_id": 5}),
		"admin":              courierToken(t, jwt.MapClaims{"id": 1, "role": "admin", "account_type": "user"}),
	}
	entregadores := map[string]string{
		"entregador (novo)":   courierToken(t, jwt.MapClaims{"id": 5, "account_type": "deliveryman"}),
		"entregador (legado)": courierToken(t, jwt.MapClaims{"id": 5}),
	}
	do := func(method, path string, h fiber.Handler, body, token string) int {
		app := fiber.New()
		app.Add(method, path, h)
		req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	for _, r := range rotas {
		if got := do(r.method, r.path, r.handler, r.body, ""); got != 401 {
			t.Errorf("sem token em %s %s: got %d, want 401", r.method, r.path, got)
		}
		for nome, tok := range intrusos {
			if got := do(r.method, r.path, r.handler, r.body, tok); got != 403 {
				t.Errorf("%s em %s %s: got %d, want 403", nome, r.method, r.path, got)
			}
		}
		for nome, tok := range entregadores {
			if got := do(r.method, r.path, r.handler, r.body, tok); got != r.okStatus {
				t.Errorf("%s em %s %s: got %d, want %d", nome, r.method, r.path, got, r.okStatus)
			}
		}
	}
}

func TestValidCourierTransition(t *testing.T) {
	casos := []struct {
		from, to string
		ok       bool
	}{
		{"", "IN_ROUTE_COLECT", true},
		{"IN_ROUTE_COLECT", "AWAIT_COLECT", true},
		{"AWAIT_COLECT", "IN_ROUTE_DELIVERY", true},
		{"IN_ROUTE_DELIVERY", "FINISHED", true},
		{"AWAIT_COLECT", "AWAIT_COLECT", true}, // repetição (resposta perdida)
		{"IN_ROUTE_COLECT", "FINISHED", false}, // pular etapas
		{"IN_ROUTE_COLECT", "IN_ROUTE_DELIVERY", false},
		{"FINISHED", "IN_ROUTE_COLECT", false}, // entregue não volta
		{"AWAIT_COLECT", "", false},            // status vazio (o bug antigo)
		{"AWAIT_COLECT", "HACKED", false},
		{"LEGADO", "IN_ROUTE_COLECT", true}, // status desconhecido recomeça, como o app
		{"LEGADO", "FINISHED", false},
	}
	for _, c := range casos {
		if got := validCourierTransition(c.from, c.to); got != c.ok {
			t.Errorf("%q → %q: got %v, want %v", c.from, c.to, got, c.ok)
		}
	}
}
