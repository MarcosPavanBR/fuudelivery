package pagarme

import (
	"testing"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// ═══════════════════════════════════════════════════════════════
// TESTES — NOME
// ═══════════════════════════════════════════════════════════════

func TestPagarMeGateway_Name(t *testing.T) {
	g := &PagarMeGateway{}
	if g.Name() != "pagarme" {
		t.Errorf("expected 'pagarme', got %q", g.Name())
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES — CAPACIDADES
// ═══════════════════════════════════════════════════════════════

func TestPagarMeGateway_SupportsMethod(t *testing.T) {
	g := &PagarMeGateway{}

	tests := []struct {
		method   gateway.PaymentMethod
		expected bool
	}{
		{gateway.MethodPIX, true},
		{gateway.MethodCreditCard, true},
		{gateway.MethodDebitCard, true},
		{gateway.PaymentMethod("boleto"), false},
		{gateway.PaymentMethod(""), false},
	}

	for _, tt := range tests {
		result := g.SupportsMethod(tt.method)
		if result != tt.expected {
			t.Errorf("SupportsMethod(%q) = %v, want %v", tt.method, result, tt.expected)
		}
	}
}

func TestPagarMeGateway_SupportsSplit(t *testing.T) {
	g := &PagarMeGateway{}
	if !g.SupportsSplit() {
		t.Error("expected SupportsSplit() = true")
	}
}

func TestPagarMeGateway_SupportsPreAuth(t *testing.T) {
	g := &PagarMeGateway{}
	if !g.SupportsPreAuth() {
		t.Error("expected SupportsPreAuth() = true")
	}
}

func TestPagarMeGateway_Supports3DS(t *testing.T) {
	g := &PagarMeGateway{}
	if !g.Supports3DS() {
		t.Error("expected Supports3DS() = true")
	}
}

func TestPagarMeGateway_SupportsEscrow(t *testing.T) {
	g := &PagarMeGateway{}
	if !g.SupportsEscrow() {
		t.Error("expected SupportsEscrow() = true")
	}
}

func TestPagarMeGateway_MaxSplitRecipients(t *testing.T) {
	g := &PagarMeGateway{}
	if g.MaxSplitRecipients() != 10 {
		t.Errorf("expected 10, got %d", g.MaxSplitRecipients())
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES — MAPEAMENTO DE STATUS
// ═══════════════════════════════════════════════════════════════

func TestMapStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected gateway.TransactionStatus
	}{
		{"waiting", gateway.StatusWaiting},
		{"authorized", gateway.StatusAuthorized},
		{"paid", gateway.StatusPaid},
		{"captured", gateway.StatusCaptured},
		{"refunded", gateway.StatusRefunded},
		{"voided", gateway.StatusVoided},
		{"canceled", gateway.StatusVoided},
		{"refused", gateway.StatusFailed},
		{"expired", gateway.StatusExpired},
		{"pending", gateway.StatusPending},
		{"unknown", gateway.StatusPending},
	}

	for _, tt := range tests {
		result := mapStatus(tt.input)
		if result != tt.expected {
			t.Errorf("mapStatus(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMapEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"paid", "paid"},
		{"refused", "failed"},
		{"refunded", "refunded"},
		{"authorized", "authorized"},
		{"captured", "captured"},
		{"voided", "voided"},
		{"canceled", "voided"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := mapEventType(tt.input)
		if result != tt.expected {
			t.Errorf("mapEventType(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES — WEBHOOK HMAC
// ═══════════════════════════════════════════════════════════════

func TestValidateHMAC_Valid(t *testing.T) {
	body := []byte(`{"id":123,"status":"paid"}`)
	secret := "test-secret-key"

	// Calcular assinatura correta
	signature := ComputeHMAC(body, secret)

	// Validar
	if !ValidateHMAC(body, signature, secret) {
		t.Error("expected valid HMAC to pass")
	}
}

func TestValidateHMAC_Invalid(t *testing.T) {
	body := []byte(`{"id":123,"status":"paid"}`)
	secret := "test-secret-key"
	invalidSignature := "invalid-signature"

	if ValidateHMAC(body, invalidSignature, secret) {
		t.Error("expected invalid HMAC to fail")
	}
}

func TestValidateHMAC_DifferentBody(t *testing.T) {
	body1 := []byte(`{"id":123,"status":"paid"}`)
	body2 := []byte(`{"id":123,"status":"refused"}`)
	secret := "test-secret-key"

	signature := ComputeHMAC(body1, secret)

	// Assinatura do body1 não deve ser válida para body2
	if ValidateHMAC(body2, signature, secret) {
		t.Error("expected HMAC mismatch for different body")
	}
}

func TestComputeHMAC_Deterministic(t *testing.T) {
	body := []byte(`{"test":"data"}`)
	secret := "my-secret"

	hmac1 := ComputeHMAC(body, secret)
	hmac2 := ComputeHMAC(body, secret)

	if hmac1 != hmac2 {
		t.Errorf("ComputeHMAC should be deterministic: %q != %q", hmac1, hmac2)
	}

	if len(hmac1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("expected 64 char hex string, got %d", len(hmac1))
	}
}

// ═══════════════════════════════════════════════════════════════
// TESTES — PARSE WEBHOOK
// ═══════════════════════════════════════════════════════════════

func TestParseWebhook_PixPaid(t *testing.T) {
	g := &PagarMeGateway{}

	body := []byte(`{
		"id": 12345,
		"object": "transaction",
		"status": "paid",
		"payment_method": "pix",
		"amount": 5000,
		"external_reference": "999",
		"pix_payload": "00020126580014BR.GOV.BCB.PIX0136...",
		"metadata": {"order_id": "999"}
	}`)

	event, err := g.ParseWebhook(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Gateway != "pagarme" {
		t.Errorf("expected gateway 'pagarme', got %q", event.Gateway)
	}
	if event.EventType != "paid" {
		t.Errorf("expected event type 'paid', got %q", event.EventType)
	}
	if event.Amount != 5000 {
		t.Errorf("expected amount 5000, got %d", event.Amount)
	}
	if event.OrderID != "999" {
		t.Errorf("expected order_id '999', got %q", event.OrderID)
	}
	if event.PaymentMethod != gateway.MethodPIX {
		t.Errorf("expected method PIX, got %q", event.PaymentMethod)
	}
}

func TestParseWebhook_CardAuthorized(t *testing.T) {
	g := &PagarMeGateway{}

	body := []byte(`{
		"id": 67890,
		"object": "transaction",
		"status": "authorized",
		"payment_method": "credit_card",
		"amount": 7500,
		"external_reference": "888",
		"card_brand": "visa",
		"card_last_four": "1234"
	}`)

	event, err := g.ParseWebhook(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Status != gateway.StatusAuthorized {
		t.Errorf("expected status 'authorized', got %q", event.Status)
	}
	if event.PaymentMethod != gateway.MethodCreditCard {
		t.Errorf("expected method credit_card, got %q", event.PaymentMethod)
	}
	if event.CardBrand != "visa" {
		t.Errorf("expected card_brand 'visa', got %q", event.CardBrand)
	}
	if event.CardLast4 != "1234" {
		t.Errorf("expected card_last4 '1234', got %q", event.CardLast4)
	}
}

func TestParseWebhook_InvalidJSON(t *testing.T) {
	g := &PagarMeGateway{}

	body := []byte(`{invalid json`)

	_, err := g.ParseWebhook(body)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseWebhook_WithSplit(t *testing.T) {
	g := &PagarMeGateway{}

	body := []byte(`{
		"id": 11111,
		"status": "paid",
		"payment_method": "pix",
		"amount": 10000,
		"split_rules": [
			{"recipient_id": "rest_01", "percentage": 75.0, "amount": 7500, "status": "paid"},
			{"recipient_id": "driver_01", "percentage": 15.0, "amount": 1500, "status": "paid"}
		]
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
		t.Errorf("expected amount 7500, got %d", event.SplitDetails[0].Amount)
	}
}
