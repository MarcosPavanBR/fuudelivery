package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// computeHMAC gera a assinatura HMAC-SHA256 de um body com uma secret.
func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// TestValidateWebhookSignature_Correta: assinatura HMAC válida deve retornar true.
func TestValidateWebhookSignature_Correta(t *testing.T) {
	os.Setenv("ABACATE_PAY_WEBHOOK_SECRET", "test-secret-key-12345")
	defer os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	body := []byte(`{"event":"billing.paid","charge":{"id":"abc123"}}`)
	sig := computeHMAC(body, "test-secret-key-12345")

	if !ValidateWebhookSignature(body, sig) {
		t.Fatal("esperava true para assinatura HMAC válida")
	}
}

// TestValidateWebhookSignature_Incorreta: assinatura HMAC inválida deve retornar false.
func TestValidateWebhookSignature_Incorreta(t *testing.T) {
	os.Setenv("ABACATE_PAY_WEBHOOK_SECRET", "test-secret-key-12345")
	defer os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	body := []byte(`{"event":"billing.paid","charge":{"id":"abc123"}}`)
	wrongSig := computeHMAC(body, "wrong-secret")

	if ValidateWebhookSignature(body, wrongSig) {
		t.Fatal("esperava false para assinatura HMAC incorreta")
	}
}

// TestValidateWebhookSignature_SemHeader: header vazio deve retornar false.
func TestValidateWebhookSignature_SemHeader(t *testing.T) {
	os.Setenv("ABACATE_PAY_WEBHOOK_SECRET", "test-secret-key-12345")
	defer os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	body := []byte(`{"event":"billing.paid"}`)

	if ValidateWebhookSignature(body, "") {
		t.Fatal("esperava false para header de assinatura vazio")
	}
}

// TestValidateWebhookSignature_SemSecret: quando a secret não está configurada,
// o HMAC é bypassado (fallback) — retorna true.
func TestValidateWebhookSignature_SemSecret(t *testing.T) {
	os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	body := []byte(`{"event":"billing.paid"}`)

	if !ValidateWebhookSignature(body, "qualquer-coisa") {
		t.Fatal("esperava true (fallback) quando ABACATE_PAY_WEBHOOK_SECRET não está definida")
	}
}

// TestValidateWebhookSignature_TamperDetection: alterar 1 byte do body deve
// invalidar a assinatura.
func TestValidateWebhookSignature_TamperDetection(t *testing.T) {
	os.Setenv("ABACATE_PAY_WEBHOOK_SECRET", "test-secret-key-12345")
	defer os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	original := []byte(`{"event":"billing.paid","charge":{"id":"abc123"}}`)
	sig := computeHMAC(original, "test-secret-key-12345")

	// Altera o body (simula tampering)
	tampered := []byte(`{"event":"billing.fraud","charge":{"id":"abc123"}}`)

	if ValidateWebhookSignature(tampered, sig) {
		t.Fatal("esperava false — assinatura não deve validar body alterado")
	}
}
