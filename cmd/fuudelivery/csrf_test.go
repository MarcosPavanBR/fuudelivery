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
	app.Post("/payments/webhook", func(c *fiber.Ctx) error { return c.SendString("wh-ok") })
	app.Use(csrfMiddleware)
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
