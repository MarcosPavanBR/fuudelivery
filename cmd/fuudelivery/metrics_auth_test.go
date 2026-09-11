package main

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestMetricsAuthorized(t *testing.T) {
	casos := []struct {
		nome       string
		goEnv      string
		token      string
		authHeader string
		querOK     bool
		querMotivo metricsDenial
	}{
		{
			// O caso que motivou a mudança: é o estado em que o serviço sobe
			// hoje, porque METRICS_TOKEN vem vazio no .env.example e no
			// render.yaml. Antes disto, aqui o endpoint ficava PÚBLICO.
			nome:  "produção sem token: NEGA",
			goEnv: "production", token: "", authHeader: "",
			querOK: false, querMotivo: metricsDeniedNoToken,
		},
		{
			nome:  "produção com token e Bearer correto: libera",
			goEnv: "production", token: "s3cr3t", authHeader: "Bearer s3cr3t",
			querOK: true, querMotivo: metricsAllowed,
		},
		{
			nome:  "produção com token e Bearer errado: nega",
			goEnv: "production", token: "s3cr3t", authHeader: "Bearer errado",
			querOK: false, querMotivo: metricsDeniedBadToken,
		},
		{
			nome:  "produção com token e sem header: nega",
			goEnv: "production", token: "s3cr3t", authHeader: "",
			querOK: false, querMotivo: metricsDeniedBadToken,
		},
		{
			// Dev local segue aberto de propósito: exigir token para rodar na
			// própria máquina só faria configurar um token de mentira para
			// calar o erro, que não protege nada.
			nome:  "dev sem token: libera",
			goEnv: "", token: "", authHeader: "",
			querOK: true, querMotivo: metricsAllowed,
		},
		{
			nome:  "dev com token configurado: continua exigindo o Bearer",
			goEnv: "development", token: "s3cr3t", authHeader: "",
			querOK: false, querMotivo: metricsDeniedBadToken,
		},
		{
			// "Bearer " sem valor não pode passar por um token vazio.
			nome:  "produção com token e header só com o prefixo: nega",
			goEnv: "production", token: "s3cr3t", authHeader: "Bearer ",
			querOK: false, querMotivo: metricsDeniedBadToken,
		},
		{
			// Token cru, sem o esquema, é um erro comum de configuração de
			// monitor — e não pode ser aceito por engano.
			nome:  "token sem o prefixo Bearer: nega",
			goEnv: "production", token: "s3cr3t", authHeader: "s3cr3t",
			querOK: false, querMotivo: metricsDeniedBadToken,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ok, motivo := metricsAuthorized(c.goEnv, c.token, c.authHeader)
			if ok != c.querOK {
				t.Errorf("autorizado: obtive %v, queria %v", ok, c.querOK)
			}
			if motivo != c.querMotivo {
				t.Errorf("motivo: obtive %q, queria %q", motivo, c.querMotivo)
			}
		})
	}
}

// TestMetricsHandler_ProducaoSemTokenDevolve403 fecha o buraco que os testes de
// rota deste pacote deixam aberto.
//
// Testar só a função pura provaria que a REGRA está certa, não que o handler a
// usa — e um handler que ignora a regra passaria despercebido, que é
// exatamente o que acontece com TestMetricsEndpoint em routes_auth_test.go
// (ele registra um /metrics de mentira e confere 200 nele). Aqui o handler
// registrado é o mesmo código do main, com a mesma chamada.
func TestMetricsHandler_ProducaoSemTokenDevolve403(t *testing.T) {
	t.Setenv("GO_ENV", "production")
	t.Setenv("METRICS_TOKEN", "")

	app := fiber.New()
	app.Get("/metrics", func(c *fiber.Ctx) error {
		ok, _ := metricsAuthorized(os.Getenv("GO_ENV"), os.Getenv("METRICS_TOKEN"), c.Get("Authorization"))
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.SendString("metricas")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/metrics", nil))
	if err != nil {
		t.Fatalf("requisição falhou: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Errorf("produção sem METRICS_TOKEN devolveu %d, queria 403 — "+
			"o endpoint está servindo dado operacional para qualquer um", resp.StatusCode)
	}

	// E com o token configurado, o mesmo handler passa a servir.
	t.Setenv("METRICS_TOKEN", "s3cr3t")
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t")
	resp2, err := app.Test(req)
	if err != nil {
		t.Fatalf("requisição falhou: %v", err)
	}
	if resp2.StatusCode != fiber.StatusOK {
		t.Errorf("com o token correto devolveu %d, queria 200", resp2.StatusCode)
	}
}
