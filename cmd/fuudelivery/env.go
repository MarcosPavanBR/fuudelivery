package main

// Validação das variáveis de ambiente obrigatórias na subida.

import (
	"log"
	"os"
	// Models (database initialization)
	// Handlers
	// Middleware
	// Dispatch engine
	// Batch expiry
	// Queue + Health + Upload + Metrics + Search
)

// isKnownJWTSecretPlaceholder retorna true se o segredo for um dos placeholders
// públicos/conhecidos (ex.: copiado do .env.example). Aceitar um desses como
// chave HMAC permitiria forjar qualquer token — inclusive role=admin.
func isKnownJWTSecretPlaceholder(secret string) bool {
	knownPlaceholders := []string{
		"change-this-to-a-random-64-char-string",
		"change-me",
		"secret",
		"your-secret-key",
		"super-secret",
		"123456",
	}
	for _, placeholder := range knownPlaceholders {
		if secret == placeholder {
			return true
		}
	}
	return false
}

// validateRequiredEnv verifica se as variaveis de ambiente essenciais estao presentes.
// Em producao, falha rapidamente (exit) se algo critico estiver faltando:
// com JWT_SECRET vazio os tokens passam a ser assinados/validados com chave
// HMAC vazia, o que permite forjar qualquer identidade — inclusive admin.
func validateRequiredEnv() {
	critical := []string{"JWT_SECRET", "DB_CONNECTION_STRING"}
	missing := false
	for _, key := range critical {
		if os.Getenv(key) == "" {
			log.Printf("[ENV] CRITICAL: %s nao configurado", key)
			missing = true
		}
	}
	if missing {
		if os.Getenv("GO_ENV") == "production" {
			log.Fatalf("[ENV] Encerrando: variaveis criticas ausentes em producao (%v)", critical)
		}
		log.Println("[ENV] Configuracao incompleta — seguindo em modo dev.")
	}

	// Reject known JWT_SECRET placeholders — they allow token forgery
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret != "" {
		if isKnownJWTSecretPlaceholder(jwtSecret) {
			log.Fatalf("[ENV] CRITICAL: JWT_SECRET is a known placeholder (%q). Generate a real secret: openssl rand -hex 32", jwtSecret)
		}
		if len(jwtSecret) < 32 {
			log.Printf("[ENV] WARNING: JWT_SECRET is only %d chars — recommended minimum is 32", len(jwtSecret))
		}
	}

	// Em producao, valida tambem as de pagamento
	if os.Getenv("GO_ENV") == "production" {
		prodRequired := []string{"ABACATE_PAY_API_KEY", "REDIS_URL"}
		for _, key := range prodRequired {
			if os.Getenv(key) == "" {
				log.Printf("[ENV] WARNING: %s nao configurado — funcionalidade limitada", key)
			}
		}
	}
}
