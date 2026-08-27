package asaas

import "log"

// ═══════════════════════════════════════════════════════════════
// VALIDAÇÃO DE WEBHOOK
// ═══════════════════════════════════════════════════════════════

// ValidateWebhookToken valida o token de autenticação do webhook Asaas.
//
// O Asaas envia o token via header "access_token" ou query param.
// O token deve ser comparado com ASAAS_WEBHOOK_TOKEN.
//
// Parameters:
//   - token: valor recebido no header/query do webhook
//   - expectedToken: valor de ASAAS_WEBHOOK_TOKEN
//
// Retorna true se o token for válido.
func ValidateWebhookToken(token, expectedToken string) bool {
	if expectedToken == "" {
		log.Println("[ASAAS] WARNING: Webhook token not configured. Skipping validation.")
		return true
	}

	if token == "" {
		log.Println("[ASAAS] Webhook validation failed: missing token")
		return false
	}

	return token == expectedToken
}
