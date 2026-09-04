// security_audit_test.go — regressões reais para os achados da auditoria de
// segurança (nov/2026). O que era testável de forma pura (sem DB/Postgres)
// fica aqui, no monolito:
//
//   - JWT_SECRET placeholder (achado #4 da auditoria) → isKnownJWTSecretPlaceholder
//
// As regressões de autorização por posse (achados #1/#2/#3 — Asaas admin-only,
// GetUserByEstablishment, HandlerEstablishmentStatus, GetPaymentByOrder)
// vivem junto das decisões que elas protegem, nos pacotes dos handlers:
//   - auth_api/app/handlers:   canManageEstablishment (establishment_handler_test.go)
//   - payment_api/app/handlers: canViewOrderPayment    (order_total_test.go)
package main

import "testing"

func TestIsKnownJWTSecretPlaceholder(t *testing.T) {
	placeholders := []string{
		"change-this-to-a-random-64-char-string", // .env.example
		"change-me",
		"secret",
		"your-secret-key",
		"super-secret",
		"123456",
	}
	for _, p := range placeholders {
		if !isKnownJWTSecretPlaceholder(p) {
			t.Errorf("placeholder %q deveria ser rejeitado", p)
		}
	}

	strong := []string{
		"",
		"a9f2c8e1b7d34f6a9f2c8e1b7d34f6a9f2c8e1b7d34f6a",
		"openssl rand -hex 32 output here",
		"63a0b1c2d3e4f5061728394a5b6c7d8e9f0a1b2c3d4e5f60718293a4b5c6d7e8f",
	}
	for _, s := range strong {
		if isKnownJWTSecretPlaceholder(s) {
			t.Errorf("segredo forte %q nao deveria ser tratado como placeholder", s)
		}
	}
}

func TestIsKnownJWTSecretPlaceholder_EmptyIsNotPlaceholder(t *testing.T) {
	// O vazio é tratado à parte pelo validateRequiredEnv (falha em produção);
	// o helper de placeholder não deve rotulá-lo como placeholder conhecido.
	if isKnownJWTSecretPlaceholder("") {
		t.Error("secret vazio não é placeholder conhecido — validação é separada")
	}
}
