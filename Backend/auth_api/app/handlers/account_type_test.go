package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Clientes, usuários de loja e entregadores têm ids de tabelas diferentes: o
// cliente 5, o usuário 5 e o entregador 5 são três pessoas. Estes testes
// tentam cada ataque de "mesmo número, outra conta" e exigem que seja barrado.

func acctReq(t *testing.T, h fiber.Handler, method, route, path, token, body string) int {
	t.Helper()
	app := fiber.New()
	app.Add(method, route, h)
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

func TestAccountTypeFromClaims(t *testing.T) {
	casos := []struct {
		nome   string
		claims jwt.MapClaims
		quer   string
	}{
		{"claim explícito vence o role", jwt.MapClaims{"account_type": "deliveryman", "role": "user"}, middlewares.AccountDeliveryMan},
		{"token legado de cliente", jwt.MapClaims{"role": "client"}, middlewares.AccountClient},
		{"token legado de entregador (sem role)", jwt.MapClaims{}, middlewares.AccountDeliveryMan},
		{"token legado de loja", jwt.MapClaims{"role": "user", "establishment_id": float64(3)}, middlewares.AccountUser},
		{"admin é conta de users", jwt.MapClaims{"role": "admin"}, middlewares.AccountUser},
		{"usuário de users com role vazio, token novo", jwt.MapClaims{"role": "", "account_type": "user"}, middlewares.AccountUser},
		{"account_type desconhecido cai no role", jwt.MapClaims{"account_type": "root", "role": "client"}, middlewares.AccountClient},
	}
	for _, c := range casos {
		if got := middlewares.AccountTypeFromClaims(c.claims); got != c.quer {
			t.Errorf("%s: got %q, want %q", c.nome, got, c.quer)
		}
	}
}

// Os três emissores de token gravam o tipo da conta.
func TestTokensNovosTrazemAccountType(t *testing.T) {
	t.Setenv("JWT_SECRET", hoursTestSecret)
	parse := func(s string) jwt.MapClaims {
		tok, err := jwt.Parse(s, func(*jwt.Token) (interface{}, error) { return []byte(hoursTestSecret), nil })
		if err != nil {
			t.Fatal(err)
		}
		return tok.Claims.(jwt.MapClaims)
	}
	u, err := middlewares.GenerateJWT(&models.User{ID: 5, Role: "user"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	d, err := middlewares.GenerateJWTDeliveryMan(&models.DeliveryMan{ID: 5})
	if err != nil {
		t.Fatal(err)
	}
	cl, err := generateClientJWT(&models.Client{ID: 5, Phone: "+5511900000000"})
	if err != nil {
		t.Fatal(err)
	}
	for quer, tok := range map[string]string{middlewares.AccountUser: u, middlewares.AccountDeliveryMan: d, middlewares.AccountClient: cl} {
		if got := parse(tok)["account_type"]; got != quer {
			t.Errorf("token de %s traz account_type=%v", quer, got)
		}
	}
}

// Cliente 5 e entregador 5 não agem sobre o usuário de loja 5.
func TestRotasDeUsuario_OutroTipoDeContaComMesmoID(t *testing.T) {
	t.Setenv("JWT_SECRET", hoursTestSecret)
	intrusos := map[string]string{
		"cliente 5 (token novo)":    hoursToken(t, jwt.MapClaims{"id": 5, "role": "client", "account_type": "client"}),
		"cliente 5 (token legado)":  hoursToken(t, jwt.MapClaims{"id": 5, "role": "client"}),
		"entregador 5 (token novo)": hoursToken(t, jwt.MapClaims{"id": 5, "account_type": "deliveryman"}),
		"entregador 5 (legado)":     hoursToken(t, jwt.MapClaims{"id": 5}),
	}
	for nome, tok := range intrusos {
		for _, r := range []struct {
			h            fiber.Handler
			method, rota string
			path, body   string
		}{
			{GetUser, "GET", "/users/:id", "/users/5", ""},
			{UpdateUser, "PUT", "/users/:id", "/users/5", `{"email":"atacante@x.com"}`},
			{DeleteUser, "DELETE", "/users/:id", "/users/5", ""},
			{ChangePassword, "PUT", "/users/:id/password", "/users/5/password", `{"current_password":"a","new_password":"b"}`},
		} {
			if got := acctReq(t, r.h, r.method, r.rota, r.path, tok, r.body); got != 403 {
				t.Errorf("%s em %s %s: got %d, want 403", nome, r.method, r.path, got)
			}
		}
		if got := acctReq(t, SessionMe, "GET", "/auth/session", "/auth/session", tok, ""); got != 401 {
			t.Errorf("%s em GET /auth/session: got %d, want 401 (não pode ver o perfil do usuário 5)", nome, got)
		}
	}
}

// Cliente 5 e usuário de loja 5 não trocam a carteira de recebimento do
// entregador 5.
func TestCarteiraDoEntregador_OutroTipoDeContaComMesmoID(t *testing.T) {
	t.Setenv("JWT_SECRET", hoursTestSecret)
	body := `{"payment_wallet_id":"carteira-do-atacante"}`
	for nome, tok := range map[string]string{
		"cliente 5": hoursToken(t, jwt.MapClaims{"id": 5, "role": "client", "account_type": "client"}),
		"usuário 5": hoursToken(t, jwt.MapClaims{"id": 5, "role": "user", "account_type": "user"}),
	} {
		if got := acctReq(t, UpdateDeliveryManWallet, "PUT", "/delivery-man/:id/wallet", "/delivery-man/5/wallet", tok, body); got != 403 {
			t.Errorf("%s: got %d, want 403", nome, got)
		}
	}
}

// IsOwnAccount exige tipo E id.
func TestIsOwnAccount(t *testing.T) {
	t.Setenv("JWT_SECRET", hoursTestSecret)
	tok := hoursToken(t, jwt.MapClaims{"id": 5, "role": "user", "account_type": "user"})
	var dono, outroTipo, outroID bool
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		dono = middlewares.IsOwnAccount(c, middlewares.AccountUser, 5)
		outroTipo = middlewares.IsOwnAccount(c, middlewares.AccountClient, 5)
		outroID = middlewares.IsOwnAccount(c, middlewares.AccountUser, 6)
		return nil
	})
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}
	if !dono || outroTipo || outroID {
		t.Fatalf("dono=%v outroTipo=%v outroID=%v; want true,false,false", dono, outroTipo, outroID)
	}
}

// Assinatura é do cliente: o entregador 5 e o usuário de loja 5 não leem,
// criam nem cancelam a do cliente 5. A recusa vem antes do banco (DB nulo).
func TestAssinatura_SoContaDeCliente(t *testing.T) {
	prev := models.DB
	models.DB = nil
	t.Cleanup(func() { models.DB = prev })
	t.Setenv("JWT_SECRET", hoursTestSecret)
	for nome, tok := range map[string]string{
		"entregador 5 (novo)":   hoursToken(t, jwt.MapClaims{"id": 5, "account_type": "deliveryman"}),
		"entregador 5 (legado)": hoursToken(t, jwt.MapClaims{"id": 5}),
		"usuário de loja 5":     hoursToken(t, jwt.MapClaims{"id": 5, "role": "user", "account_type": "user", "establishment_id": 2}),
	} {
		for _, r := range []struct {
			h            fiber.Handler
			method, path string
		}{
			{GetUserSubscription, "GET", "/subscriptions/me"},
			{CreateSubscription, "POST", "/subscriptions"},
			{CancelSubscription, "POST", "/subscriptions/cancel"},
		} {
			if got := acctReq(t, r.h, r.method, r.path, r.path, tok, `{"plan":"premium"}`); got != 403 {
				t.Errorf("%s em %s %s: got %d, want 403", nome, r.method, r.path, got)
			}
		}
	}
}
