package main

// Testes de regressão do middleware CSRF (double-submit cookie).
//
// Vulnerabilidade corrigida: a versão anterior aceitava o token do PRÓPRIO
// cookie quando o header X-CSRF-Token estava ausente — exatamente o que um
// ataque CSRF faz (o browser envia o cookie automaticamente em qualquer
// form/POST cross-site). Agora header×cookie precisam bater.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

const testCSRFToken = "0123456789abcdef0123456789abcdef" // 32 hex chars

func newCSRFTestApp() *fiber.App {
	app := fiber.New()
	// O middleware vem ANTES de toda rota, inclusive a do webhook. Se o
	// webhook fosse registrado primeiro, o Fiber o atenderia sem passar pelo
	// middleware e o teste de isenção passaria sem exercitar
	// IsCSRFExemptPath — verde por acidente, não por proteção.
	app.Use(csrfMiddleware)
	app.Post("/payments/webhook", func(c *fiber.Ctx) error { return c.SendString("wh-ok") })
	app.Post("/mutate", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Put("/mutate", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Delete("/mutate", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/read", func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

// csrfReq executa uma requisição contra o app de teste com headers e cookies arbitrários.
func csrfReq(t *testing.T, app *fiber.App, method, path string, headers map[string]string, cookies map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test(%s %s) failed: %v", method, path, err)
	}
	return resp
}

// ── Atacante cross-site: cookie enviado sozinho, sem header ──

func TestCSRF_CookieWithoutHeaderIsRejected(t *testing.T) {
	app := newCSRFTestApp()
	resp := csrfReq(t, app, "POST", "/mutate", nil, map[string]string{csrfCookieName: testCSRFToken})
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403 for POST with cookie but no header, got %d", resp.StatusCode)
	}
}

func TestCSRF_WrongHeaderIsRejected(t *testing.T) {
	app := newCSRFTestApp()
	resp := csrfReq(t, app, "POST", "/mutate",
		map[string]string{"X-CSRF-Token": "11112222333344445555666677778888"},
		map[string]string{csrfCookieName: testCSRFToken},
	)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403 for mismatched header×cookie, got %d", resp.StatusCode)
	}
}

// ── Cliente web legítimo: header confere com o cookie ──

func TestCSRF_MatchingHeaderCookieAccepted(t *testing.T) {
	app := newCSRFTestApp()
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		resp := csrfReq(t, app, method, "/mutate",
			map[string]string{"X-CSRF-Token": testCSRFToken},
			map[string]string{csrfCookieName: testCSRFToken},
		)
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200 for %s with matching header×cookie, got %d", method, resp.StatusCode)
		}
	}
}

// ── O furo real: sessão de browser SEM cookie de CSRF ──
//
// Era este o caso que a versão anterior liberava. O gate era
// `if csrf_token == "" { passa }`, então bastava a requisição chegar sem o
// cookie de CSRF para a proteção se desligar — e isso acontecia sozinho,
// porque o csrf_token expirava antes da sessão. Com SameSite=Strict o
// navegador nem mandava os cookies cross-site e o furo era teórico; com
// SameSite=None (necessário porque frontend e API vivem em subdomínios
// .onrender.com distintos) ele virou alcançável de verdade.
//
// O gatilho correto é o cookie de SESSÃO: se ele veio, o browser está
// mandando credencial ambiente e o double-submit é obrigatório.

func TestCSRF_SessionWithoutCsrfCookieIsRejected(t *testing.T) {
	for _, sessionCookie := range []string{accessCookieName, refreshCookieName} {
		t.Run(sessionCookie, func(t *testing.T) {
			app := newCSRFTestApp()
			resp := csrfReq(t, app, "POST", "/mutate", nil,
				map[string]string{sessionCookie: "jwt-de-sessao-valido"},
			)
			if resp.StatusCode != fiber.StatusForbidden {
				t.Fatalf("esperava 403 para mutação com cookie de sessão %s e sem cookie de CSRF, veio %d",
					sessionCookie, resp.StatusCode)
			}
		})
	}
}

// Mesmo com sessão, um header sozinho (sem o cookie de CSRF para comparar)
// não vale: o double-submit exige os dois lados.
func TestCSRF_SessionWithHeaderButNoCsrfCookieIsRejected(t *testing.T) {
	app := newCSRFTestApp()
	resp := csrfReq(t, app, "POST", "/mutate",
		map[string]string{"X-CSRF-Token": testCSRFToken},
		map[string]string{accessCookieName: "jwt-de-sessao-valido"},
	)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("esperava 403 para header de CSRF sem o cookie correspondente, veio %d", resp.StatusCode)
	}
}

// Contraprova: a sessão legítima do painel web, que carrega os dois cookies
// e reenvia o token no header, continua passando. Sem este teste o aperto
// acima poderia estar simplesmente rejeitando tudo.
func TestCSRF_SessionWithMatchingDoubleSubmitAccepted(t *testing.T) {
	app := newCSRFTestApp()
	resp := csrfReq(t, app, "POST", "/mutate",
		map[string]string{"X-CSRF-Token": testCSRFToken},
		map[string]string{
			accessCookieName: "jwt-de-sessao-valido",
			csrfCookieName:   testCSRFToken,
		},
	)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("esperava 200 para sessão web com double-submit correto, veio %d", resp.StatusCode)
	}
}

// ── Clientes Bearer-only (mobile): sem cookie de sessão web, CSRF não se aplica ──

func TestCSRF_NoCookieNoHeaderAccepted(t *testing.T) {
	app := newCSRFTestApp()
	resp := csrfReq(t, app, "POST", "/mutate", nil, nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for Bearer-only mutation without any cookie, got %d", resp.StatusCode)
	}
}

// ── GET/HEAD nunca exigem CSRF (OPTIONS é tratado pelo CORS do app real) ──

func TestCSRF_SafeMethodsPassThrough(t *testing.T) {
	app := newCSRFTestApp()
	for _, method := range []string{"GET", "HEAD"} {
		resp := csrfReq(t, app, method, "/read", nil, nil)
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", method, resp.StatusCode)
		}
	}
}

// ── Webhooks de gateway são isentos (autenticação própria: HMAC/token) ──

func TestCSRF_PaymentWebhookExempt(t *testing.T) {
	app := newCSRFTestApp()
	resp := csrfReq(t, app, "POST", "/payments/webhook",
		map[string]string{csrfCookieName: "does-not-matter"},
		nil,
	)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for payment webhook (gateway-authenticated), got %d", resp.StatusCode)
	}
}

// ── IsCSRFExemptPath unitário ──

func TestIsCSRFExemptPath(t *testing.T) {
	if !IsCSRFExemptPath("/payments/webhook") {
		t.Error("/payments/webhook deve ser isento de CSRF")
	}
	for _, p := range []string{"/payments/webhook/", "/payments/webhooks", "/users/login", "/auth/session"} {
		if IsCSRFExemptPath(p) {
			t.Errorf("%s não deveria ser isento de CSRF", p)
		}
	}
}
