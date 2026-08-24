package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// newDebugIPApp monta um app Fiber com EXATAMENTE a mesma config de proxy do
// monolith (main.go: TrustedProxies 0.0.0.0/0 + ::/0, ProxyHeader
// X-Forwarded-For) e registra o handler real debugIPHandler na rota /debug-ip.
// Nao precisa de banco nem JWT: testa o handler puro, focado no diagnostico.
func newDebugIPApp() *fiber.App {
	app := fiber.New(fiber.Config{
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"0.0.0.0/0", "::/0"},
		ProxyHeader:             "X-Forwarded-For",
	})
	app.Get("/debug-ip", debugIPHandler)
	return app
}

func TestDebugIPEndpoint(t *testing.T) {
	app := newDebugIPApp()

	t.Run("SemHeadersDeProxy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/debug-ip", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		var payload map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))

		// Comportamento REAL do Fiber v2 com TrustedProxies aberto: o socket
		// (0.0.0.0:0 no httptest) e sempre "proxy confiavel", entao c.IP()
		// devolve o XFF — que aqui esta vazio — em vez de cair para o IP do
		// socket. Ou seja: c.IP() == "" num request sem X-Forwarded-For.
		// Em producao (Render injeta XFF) isso vira a cadeia inteira crua.
		require.Equal(t, "", payload["c_ip"], "c.IP() = XFF vazio quando o socket e confiavel e nao ha header")
		require.Equal(t, "", payload["x_forwarded_for"])
		require.Equal(t, "", payload["x_real_ip"])
	})

	t.Run("ComXForwardedFor", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/debug-ip", nil)
		// Render encaminha a cadeia: cliente real, proxy Render.
		req.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.1")
		req.Header.Set("X-Real-IP", "203.0.113.42")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		var payload map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))

		// Comportamento REAL do Fiber v2 com EnableTrustedProxyCheck=true e
		// TrustedProxies aberto (0.0.0.0/0): o socket e sempre "proxy
		// confiavel", entao c.IP() devolve o header X-Forwarded-For CRU
		// (a cadeia inteira), NAO o primeiro endereco. Isso e exatamente o
		// problema que o endpoint diagnostica: o rate limit Redis chaveia por
		// c.IP() = cadeia inteira, forjavel e inconsistente por request.
		require.Equal(t, "203.0.113.42, 10.0.0.1", payload["c_ip"], "Fiber v2 devolve o XFF cru inteiro")
		require.Equal(t, "203.0.113.42, 10.0.0.1", payload["x_forwarded_for"], "header cru preservado")
		require.Equal(t, "203.0.113.42", payload["x_real_ip"], "X-Real-IP cru preservado")
	})

	t.Run("XForwardedForComEspacos", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/debug-ip", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.7")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		var payload map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))

		// Header com um unico IP: c.IP() == XFF (sem virgula para desambiguar).
		require.Equal(t, "198.51.100.7", payload["c_ip"])
		require.Equal(t, "198.51.100.7", payload["x_forwarded_for"])
	})
}
