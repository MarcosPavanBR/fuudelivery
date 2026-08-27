package mercadopago

import (
	"encoding/json"
	"testing"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// ═══════════════════════════════════════════════════════════════
// TESTES BÁSICOS DO GATEWAY
// ═══════════════════════════════════════════════════════════════

func TestGatewayName(t *testing.T) {
	gw := &MercadoPagoGateway{client: &Client{}}
	if gw.Name() != "mercadopago" {
		t.Errorf("esperado 'mercadopago', recebido '%s'", gw.Name())
	}
}

func TestSupportsMethod(t *testing.T) {
	gw := &MercadoPagoGateway{client: &Client{}}

	tests := []struct {
		method   gateway.PaymentMethod
		expected bool
	}{
		{gateway.MethodPIX, true},
		{gateway.MethodCreditCard, true},
		{gateway.MethodDebitCard, true},
		{"invalido", false},
	}

	for _, tt := range tests {
		if got := gw.SupportsMethod(tt.method); got != tt.expected {
			t.Errorf("SupportsMethod(%s) = %v, esperado %v", tt.method, got, tt.expected)
		}
	}
}

func TestSupportsCapabilities(t *testing.T) {
	gw := &MercadoPagoGateway{client: &Client{}}

	if !gw.SupportsSplit() {
		t.Error("esperado SupportsSplit() = true")
	}
	if gw.SupportsPreAuth() {
		t.Error("MP não suporta pré-autorização")
	}
	if !gw.Supports3DS() {
		t.Error("MP suporta 3DS via API")
	}
	if gw.SupportsEscrow() {
		t.Error("MP não suporta escrow")
	}
	if gw.MaxSplitRecipients() != 1 {
		t.Errorf("esperado MaxSplit=1, recebido %d", gw.MaxSplitRecipients())
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES DE MAPEAMENTO DE STATUS
// ═══════════════════════════════════════════════════════════════

func TestMapMPStatusToInternal(t *testing.T) {
	tests := []struct {
		mpStatus string
		expected gateway.TransactionStatus
	}{
		{"approved", gateway.StatusPaid},
		{"authorized", gateway.StatusAuthorized},
		{"pending", gateway.StatusPending},
		{"in_process", gateway.StatusPending},
		{"in_mediation", gateway.StatusPending},
		{"rejected", gateway.StatusFailed},
		{"cancelled", gateway.StatusVoided},
		{"refunded", gateway.StatusRefunded},
		{"charged_back", gateway.StatusChargeback},
		{"status_desconhecido", gateway.StatusPending}, // fallback
	}

	for _, tt := range tests {
		got := mapMPStatus(tt.mpStatus)
		if got != tt.expected {
			t.Errorf("mapMPStatus(%s) = %s, esperado %s", tt.mpStatus, got, tt.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES DE WEBHOOK
// ═══════════════════════════════════════════════════════════════

func TestValidateWebhookSemSecret(t *testing.T) {
	gw := &MercadoPagoGateway{client: &Client{}}
	if !gw.ValidateWebhook([]byte("body"), map[string]string{}) {
		t.Error("deveria aceitar sem secret (modo dev)")
	}
}

func TestValidateWebhookComSecret(t *testing.T) {
	gw := &MercadoPagoGateway{
		client:        &Client{},
		webhookSecret: "test_secret_123",
	}

	body := []byte(`{"action":"payment.updated","data":{"id":"123"}}`)

	// Sem header → deve falhar
	if gw.ValidateWebhook(body, map[string]string{}) {
		t.Error("deveria rejeitar sem header x-signature")
	}

	// Com header inválido → deve falhar
	if gw.ValidateWebhook(body, map[string]string{"x-signature": "invalido"}) {
		t.Error("deveria rejeitar assinatura inválida")
	}
}

func TestParseWebhookPaymentUpdated(t *testing.T) {
	gw := &MercadoPagoGateway{client: &Client{}}

	payload := WebhookPayload{
		Action: "payment.updated",
		Data: &WebhookData{ID: "mp_123456"},
	}

	body, _ := json.Marshal(payload)
	event, err := gw.ParseWebhook(body)
	if err != nil {
		t.Fatalf("erro ao parsear: %v", err)
	}

	if event.Type != gateway.WebhookPaymentPending {
		t.Errorf("tipo esperado payment_pending (fallback sem HTTP), recebido %s", event.Type)
	}
	if event.PaymentExternalID != "mp_123456" {
		t.Errorf("ID esperado mp_123456, recebido %s", event.PaymentExternalID)
	}
}

func TestParseWebhookDesconhecido(t *testing.T) {
	gw := &MercadoPagoGateway{client: &Client{}}

	payload := WebhookPayload{
		Action: "unknown.action",
		Data: &WebhookData{ID: "abc"},
	}

	body, _ := json.Marshal(payload)
	event, err := gw.ParseWebhook(body)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	if event.Type != gateway.WebhookPaymentPending {
		t.Errorf("tipo esperado payment_pending (fallback), recebido %s", event.Type)
	}
}
