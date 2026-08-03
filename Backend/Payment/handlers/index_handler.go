// Package handlers — index_handler.go
// Rota raiz (GET /) — índice público da API do Payment Service.
// Retorna identidade do serviço, status atual e a lista de endpoints
// organizados por recurso, para auto-descoberta e monitoramento.
package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// HealthPayload monta o payload de status usado tanto em GET /health
// quanto na raiz (GET /). Mantém a resposta do /health idêntica à atual.
func HealthPayload() fiber.Map {
	return fiber.Map{
		"status":  "ok",
		"service": "payment",
	}
}

// endpointsIndex é a lista canônica de endpoints do serviço.
// Mantida próxima do registro real de rotas em main.go para não divergir.
var endpointsIndex = map[string]fiber.Map{
	"health": {
		"GET /health": "Health check (público)",
	},
	"auth": {
		"POST /api/auth/login": "Login (público)",
	},
	"payments": {
		"GET /api/payments/":               "Listar pagamentos",
		"GET /api/payments/stats":          "Estatísticas de pagamentos",
		"GET /api/payments/:id":            "Buscar pagamento por ID",
		"POST /api/payments/":              "Criar pagamento",
		"POST /api/payments/:id/approve":   "Aprovar pagamento",
		"POST /api/payments/:id/reject":    "Rejeitar pagamento",
	},
	"approvals": {
		"GET /api/approvals/queue":            "Fila de aprovação",
		"GET /api/approvals/auto-approved":    "Pagamentos aprovados automaticamente",
		"GET /api/approvals/rules":            "Regras de risco",
		"PUT /api/approvals/rules":            "Atualizar regras de risco",
	},
	"chargebacks": {
		"GET /api/chargebacks/":               "Listar chargebacks",
		"GET /api/chargebacks/stats":          "Estatísticas de chargebacks",
		"GET /api/chargebacks/:id":            "Buscar chargeback por ID",
		"POST /api/chargebacks/":              "Criar chargeback",
		"POST /api/chargebacks/:id/approve":   "Aprovar chargeback",
		"POST /api/chargebacks/:id/reject":    "Rejeitar chargeback",
		"POST /api/chargebacks/:id/evidence":  "Adicionar evidência",
		"GET /api/chargebacks/:id/evidence":   "Listar evidências",
	},
	"wallets": {
		"GET /api/wallets/:user_id":               "Consultar saldo da carteira",
		"GET /api/wallets/:user_id/transactions":  "Transações da carteira",
		"POST /api/wallets/:user_id/credit":       "Creditar carteira",
		"POST /api/wallets/:user_id/debit":        "Debitar carteira",
		"POST /api/wallets/:user_id/withdraw":     "Saque da carteira",
		"GET /api/wallets/:user_id/get-or-create": "Obter ou criar carteira",
	},
	"reports": {
		"GET /api/reports/establishment/:id": "Relatório do estabelecimento",
	},
	"users": {
		"GET /api/users/":    "Listar usuários (admin)",
		"GET /api/users/:id": "Buscar usuário por ID (admin)",
		"POST /api/users/":   "Criar usuário (admin)",
	},
}

// Index é o handler da rota raiz (GET /).
// Retorna 200 com o índice da API — útil para health checks de "página
// inicial" e para quem acessar a URL no navegador.
func Index(c *fiber.Ctx) error {
	payload := HealthPayload()
	payload["name"] = "FuuPayment Service"
	payload["version"] = "1.0.0"
	payload["time"] = time.Now().UTC()
	payload["docs"] = "https://github.com/MarcosPavanBR/fuudelivery"
	payload["endpoints"] = endpointsIndex

	return c.JSON(payload)
}
