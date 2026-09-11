package main

import "crypto/subtle"

// Autorização do GET /metrics.
//
// A lógica mora numa função PURA, separada do handler, porque os testes de rota
// deste pacote (routes_auth_test.go) registram handlers de mentira num
// fiber.App novo em vez de exercitar os reais — `TestMetricsEndpoint` monta um
// `/metrics` stub e confere 200. Testes assim não conseguem falhar quando o
// código de produção quebra. Com a regra isolada aqui, o teste cobre
// exatamente o que roda em produção, sem precisar de harness HTTP.

// motivo explica por que o acesso foi negado, para o log de quem opera.
type metricsDenial string

const (
	metricsAllowed        metricsDenial = ""
	metricsDeniedNoToken  metricsDenial = "METRICS_TOKEN não configurado em produção"
	metricsDeniedBadToken metricsDenial = "token inválido ou ausente"
)

// metricsAuthorized decide se a requisição pode ler o /metrics.
//
//	GO_ENV=production + token vazio -> NEGA (403)
//	GO_ENV=production + token        -> exige Authorization: Bearer <token>
//	outro ambiente   + token vazio  -> libera (dev local, sem atrito)
//	outro ambiente   + token        -> exige o Bearer
//
// Antes, token vazio significava "sem checagem" em QUALQUER ambiente: o
// endpoint ficava público em produção, servindo volume de pedidos, backlog de
// pagamentos e profundidade da DLQ a quem pedisse. E como METRICS_TOKEN vem
// vazio no .env.example e no render.yaml, o caminho provável era exatamente
// esse. Isso não é credencial nem dado pessoal, mas é reconhecimento de graça
// — e a regra do projeto é falhar fechado, como nos ValidateWebhook dos
// adapters de pagamento.
func metricsAuthorized(goEnv, token, authHeader string) (bool, metricsDenial) {
	producao := goEnv == "production"

	if token == "" {
		if producao {
			return false, metricsDeniedNoToken
		}
		return true, metricsAllowed
	}

	// Comparação em tempo constante: o token é um segredo, e comparar com ==
	// vaza o tamanho do prefixo correto por tempo de resposta. É o mesmo
	// cuidado já aplicado na validação de webhook (hmac.Equal).
	esperado := "Bearer " + token
	if subtle.ConstantTimeCompare([]byte(authHeader), []byte(esperado)) != 1 {
		return false, metricsDeniedBadToken
	}

	return true, metricsAllowed
}
