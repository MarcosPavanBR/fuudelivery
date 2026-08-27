package abacatepay

import (
	"encoding/json"
	"testing"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// ═══════════════════════════════════════════════════════════════
// TESTES BÁSICOS DO GATEWAY
// ═══════════════════════════════════════════════════════════════

func TestGatewayName(t *testing.T) {
	gw := &AbacatePayGateway{client: &Client{}}
	if gw.Name() != "abacatepay" {
		t.Errorf("esperado 'abacatepay', recebido '%s'", gw.Name())
	}
}

func TestSupportsMethod(t *testing.T) {
	gw := &AbacatePayGateway{client: &Client{}}

	tests := []struct {
		method   gateway.PaymentMethod
		expected bool
	}{
		{gateway.MethodPIX, true},
		{gateway.MethodCreditCard, false},
		{gateway.MethodDebitCard, false},
		{"invalido", false},
	}

	for _, tt := range tests {
		if got := gw.SupportsMethod(tt.method); got != tt.expected {
			t.Errorf("SupportsMethod(%s) = %v, esperado %v", tt.method, got, tt.expected)
		}
	}
}

func TestSupportsCapabilities(t *testing.T) {
	gw := &AbacatePayGateway{client: &Client{}}

	if gw.SupportsSplit() {
		t.Error("AbacatePay NÃO suporta split")
	}
	if gw.SupportsPreAuth() {
		t.Error("AbacatePay não suporta pré-autorização")
	}
	if gw.Supports3DS() {
		t.Error("AbacatePay não suporta 3DS")
	}
	if gw.SupportsEscrow() {
		t.Error("AbacatePay não suporta escrow")
	}
	if gw.MaxSplitRecipients() != 0 {
		t.Errorf("esperado MaxSplit=0, recebido %d", gw.MaxSplitRecipients())
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES DE WEBHOOK
// ═══════════════════════════════════════════════════════════════

func TestValidateWebhookSemSecret(t *testing.T) {
	gw := &AbacatePayGateway{client: &Client{}}
	if !gw.ValidateWebhook([]byte("body"), map[string]string{}) {
		t.Error("deveria aceitar sem secret (modo dev)")
	}
}

func TestParseWebhookPaid(t *testing.T) {
	gw := &AbacatePayGateway{client: &Client{}}

	payload := WebhookPayload{
		ID:     "abt_123",
		Status: "paid",
	}

	body, _ := json.Marshal(payload)
	event, err := gw.ParseWebhook(body)
	if err != nil {
		t.Fatalf("erro ao parsear: %v", err)
	}

	if event.Type != gateway.WebhookPaymentApproved {
		t.Errorf("tipo esperado payment_approved, recebido %s", event.Type)
	}
	if event.PaymentExternalID != "abt_123" {
		t.Errorf("ID esperado abt_123, recebido %s", event.PaymentExternalID)
	}
}

func TestParseWebhookExpired(t *testing.T) {
	gw := &AbacatePayGateway{client: &Client{}}

	payload := WebhookPayload{
		ID:     "abt_456",
		Status: "expired",
	}

	body, _ := json.Marshal(payload)
	event, err := gw.ParseWebhook(body)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	if event.Type != gateway.WebhookPaymentFailed {
		t.Errorf("tipo esperado payment_failed, recebido %s", event.Type)
	}
}

func TestParseWebhookDesconhecido(t *testing.T) {
	gw := &AbacatePayGateway{client: &Client{}}

	payload := WebhookPayload{
		ID:     "abt_789",
		Status: "unknown_status",
	}

	body, _ := json.Marshal(payload)
	event, err := gw.ParseWebhook(body)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	if event.Type != gateway.WebhookPaymentPending {
		t.Errorf("tipo esperado payment_pending, recebido %s", event.Type)
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES DE ERROS NÃO IMPLEMENTADOS
// ═══════════════════════════════════════════════════════════════

func TestCreateTransactionNaoSuportado(t *testing.T) {
	gw := &AbacatePayGateway{client: &Client{}}
	_, err := gw.CreateTransaction(nil, &gateway.TransactionRequest{})
	if err == nil {
		t.Error("esperado erro (PIX não suporta CreateTransaction)")
	}
}
