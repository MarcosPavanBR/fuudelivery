package asaas

import (
	"testing"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// ═══════════════════════════════════════════════════════════════
// TESTES — NOME
// ═══════════════════════════════════════════════════════════════

func TestAsaasGateway_Name(t *testing.T) {
	g := &AsaasGateway{}
	if g.Name() != "asaas" {
		t.Errorf("expected 'asaas', got %q", g.Name())
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES — CAPACIDADES
// ═══════════════════════════════════════════════════════════════

func TestAsaasGateway_SupportsMethod(t *testing.T) {
	g := &AsaasGateway{}

	tests := []struct {
		method   gateway.PaymentMethod
		expected bool
	}{
		{gateway.MethodPIX, true},
		{gateway.MethodCreditCard, true},
		{gateway.MethodDebitCard, true},
		{gateway.PaymentMethod("boleto"), false},
	}

	for _, tt := range tests {
		result := g.SupportsMethod(tt.method)
		if result != tt.expected {
			t.Errorf("SupportsMethod(%q) = %v, want %v", tt.method, result, tt.expected)
		}
	}
}

func TestAsaasGateway_SupportsSplit(t *testing.T) {
	g := &AsaasGateway{}
	if !g.SupportsSplit() {
		t.Error("expected SupportsSplit() = true")
	}
}

func TestAsaasGateway_SupportsPreAuth(t *testing.T) {
	g := &AsaasGateway{}
	if !g.SupportsPreAuth() {
		t.Error("expected SupportsPreAuth() = true")
	}
}

func TestAsaasGateway_Supports3DS(t *testing.T) {
	g := &AsaasGateway{}
	if !g.Supports3DS() {
		t.Error("expected Supports3DS() = true")
	}
}

func TestAsaasGateway_SupportsEscrow(t *testing.T) {
	g := &AsaasGateway{}
	if !g.SupportsEscrow() {
		t.Error("expected SupportsEscrow() = true")
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES — MAPEAMENTO DE STATUS
// ═══════════════════════════════════════════════════════════════

func TestMapAsaasStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected gateway.TransactionStatus
	}{
		{"PENDING", gateway.StatusPending},
		{"RECEIVED", gateway.StatusPaid},
		{"CONFIRMED", gateway.StatusPaid},
		{"OVERDUE", gateway.StatusExpired},
		{"REFUNDED", gateway.StatusRefunded},
		{"CHARGEBACK_REQUESTED", gateway.StatusChargeback},
		{"UNKNOWN", gateway.StatusPending},
	}

	for _, tt := range tests {
		result := mapAsaasStatus(tt.input)
		if result != tt.expected {
			t.Errorf("mapAsaasStatus(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMapAsaasEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PAYMENT_RECEIVED", "paid"},
		{"PAYMENT_CREDITED", "paid"},
		{"PAYMENT_OVERDUE", "expired"},
		{"PAYMENT_REFUNDED", "refunded"},
		{"PAYMENT_SPLIT_DONE", "split_done"},
		{"PAYMENT_SPLIT_DIVERGENCE_BLOCK", "split_block"},
		{"CHARGEBACK_REQUESTED", "chargeback"},
		{"UNKNOWN_EVENT", "UNKNOWN_EVENT"},
	}

	for _, tt := range tests {
		result := mapAsaasEventType(tt.input)
		if result != tt.expected {
			t.Errorf("mapAsaasEventType(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMapAsaasSplitStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected gateway.SplitStatus
	}{
		{"PENDING", gateway.SplitPending},
		{"CREDITED", gateway.SplitPaid},
		{"DONE", gateway.SplitPaid},
		{"REFUSED", gateway.SplitFailed},
		{"REFUNDED", gateway.SplitRefunded},
		{"BLOCKED", gateway.SplitBlocked},
		{"UNKNOWN", gateway.SplitPending},
	}

	for _, tt := range tests {
		result := mapAsaasSplitStatus(tt.input)
		if result != tt.expected {
			t.Errorf("mapAsaasSplitStatus(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES — WEBHOOK VALIDATION
// ═══════════════════════════════════════════════════════════════

func TestValidateWebhookToken_Valid(t *testing.T) {
	if !ValidateWebhookToken("my-secret-token", "my-secret-token") {
		t.Error("expected valid token to pass")
	}
}

func TestValidateWebhookToken_Invalid(t *testing.T) {
	if ValidateWebhookToken("wrong-token", "my-secret-token") {
		t.Error("expected invalid token to fail")
	}
}

func TestValidateWebhookToken_Empty(t *testing.T) {
	if ValidateWebhookToken("", "my-secret-token") {
		t.Error("expected empty token to fail")
	}
}

func TestValidateWebhookToken_NoSecret(t *testing.T) {
	// When expected token is empty, validation is skipped
	if !ValidateWebhookToken("any-token", "") {
		t.Error("expected validation to be skipped when secret is empty")
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES — PARSE WEBHOOK
// ═══════════════════════════════════════════════════════════════

func TestParseWebhook_PixPaid(t *testing.T) {
	g := &AsaasGateway{}

	body := []byte(`{
		"event": "PAYMENT_RECEIVED",
		"payment": {
			"id": "pay_123",
			"billingType": "PIX",
			"status": "RECEIVED",
			"value": 50.00,
			"externalReference": "999",
			"pixCopyPaste": "00020126580014BR.GOV.BCB.PIX..."
		}
	}`)

	event, err := g.ParseWebhook(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Gateway != "asaas" {
		t.Errorf("expected gateway 'asaas', got %q", event.Gateway)
	}
	if event.EventType != "paid" {
		t.Errorf("expected event type 'paid', got %q", event.EventType)
	}
	if event.Amount != 5000 {
		t.Errorf("expected amount 5000 cents, got %d", event.Amount)
	}
	if event.OrderID != "999" {
		t.Errorf("expected order_id '999', got %q", event.OrderID)
	}
}

func TestParseWebhook_CardConfirmed(t *testing.T) {
	g := &AsaasGateway{}

	body := []byte(`{
		"event": "PAYMENT_RECEIVED",
		"payment": {
			"id": "pay_456",
			"billingType": "CREDIT_CARD",
			"status": "CONFIRMED",
			"value": 75.50,
			"externalReference": "888"
		}
	}`)

	event, err := g.ParseWebhook(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Status != gateway.StatusPaid {
		t.Errorf("expected status 'paid', got %q", event.Status)
	}
	if event.PaymentMethod != gateway.MethodCreditCard {
		t.Errorf("expected method credit_card, got %q", event.PaymentMethod)
	}
}

func TestParseWebhook_WithSplit(t *testing.T) {
	g := &AsaasGateway{}

	body := []byte(`{
		"event": "PAYMENT_SPLIT_DONE",
		"payment": {
			"id": "pay_789",
			"billingType": "PIX",
			"status": "RECEIVED",
			"value": 100.00,
			"externalReference": "777",
			"split": [
				{"walletId": "rest_01", "amount": 75.00, "status": "CREDITED"},
				{"walletId": "driver_01", "amount": 15.00, "status": "CREDITED"}
			]
		}
	}`)

	event, err := g.ParseWebhook(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(event.SplitDetails) != 2 {
		t.Fatalf("expected 2 split details, got %d", len(event.SplitDetails))
	}

	if event.SplitDetails[0].RecipientID != "rest_01" {
		t.Errorf("expected recipient 'rest_01', got %q", event.SplitDetails[0].RecipientID)
	}
	if event.SplitDetails[0].Amount != 7500 {
		t.Errorf("expected amount 7500 cents, got %d", event.SplitDetails[0].Amount)
	}
}

func TestParseWebhook_InvalidJSON(t *testing.T) {
	g := &AsaasGateway{}

	body := []byte(`{invalid json`)

	_, err := g.ParseWebhook(body)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseWebhook_MissingPayment(t *testing.T) {
	g := &AsaasGateway{}

	body := []byte(`{"event": "PAYMENT_RECEIVED"}`)

	_, err := g.ParseWebhook(body)
	if err == nil {
		t.Error("expected error for missing payment data")
	}
}
