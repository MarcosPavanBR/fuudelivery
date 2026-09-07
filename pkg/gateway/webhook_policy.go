package gateway

import (
	"log"
	"os"
)

// AllowUnsignedWebhook decide o que fazer quando um adapter não tem secret de
// webhook configurada.
//
// Regra (espelha ValidateWebhookSignature em payment_api/handlers/webhook.go):
// em produção, secret ausente REJEITA o webhook (fail-closed). Fora de
// produção, permite, para não travar desenvolvimento local.
//
// Motivo de existir: os quatro adapters retornavam `true` com secret vazia,
// sem olhar o ambiente. Hoje nenhuma rota chama esses ValidateWebhook (o
// endpoint em produção usa o validador do payment_api), mas qualquer rota
// nova herdaria a falha aberta — um erro de configuração em produção
// desligaria a verificação de assinatura silenciosamente, que é exatamente o
// que a regra 3 de rules/common/security.md proíbe.
//
// gatewayName e secretEnv entram só no log, para o operador saber qual
// variável configurar.
func AllowUnsignedWebhook(gatewayName, secretEnv string) bool {
	if os.Getenv("GO_ENV") == "production" {
		log.Printf("[%s] %s ausente em produção — rejeitando webhook (fail-closed)",
			gatewayName, secretEnv)
		return false
	}
	log.Printf("[%s] %s não configurada — validação de assinatura desligada (apenas dev)",
		gatewayName, secretEnv)
	return true
}
