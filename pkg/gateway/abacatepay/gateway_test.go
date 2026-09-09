package abacatepay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// ═══════════════════════════════════════════════════════════════
// STATUS MAPPING E GETCHARGEDETAILS
// ═══════════════════════════════════════════════════════════════

// A v2 mistura caixas: /transparents/create devolve "waiting" (lowercase),
// /transparents/check e webhooks devolvem "PAID" (uppercase). O webhook
// confirma pagamento a partir do segundo — mapear só lowercase deixa o
// PAID cair em StatusPending e o pagamento nunca é confirmado.
func TestMapAbacateStatus_CaseInsensitive(t *testing.T) {
	tests := []struct {
		in   string
		want gateway.TransactionStatus
	}{
		{"PAID", gateway.StatusPaid},
		{"paid", gateway.StatusPaid},
		{"Paid", gateway.StatusPaid},
		{"EXPIRED", gateway.StatusExpired},
		{"REFUNDED", gateway.StatusRefunded},
		{"CANCELED", gateway.StatusFailed},
		{"cancelled", gateway.StatusFailed},
		{"refused", gateway.StatusFailed},
		{"waiting", gateway.StatusWaiting},
		{"WAITING", gateway.StatusWaiting},
		{"pENDING", gateway.StatusPending},
		{"desconhecido", gateway.StatusPending},
		{"", gateway.StatusPending},
	}
	for _, tt := range tests {
		if got := mapAbacateStatus(tt.in); got != tt.want {
			t.Errorf("mapAbacateStatus(%q) = %q, esperado %q", tt.in, got, tt.want)
		}
	}
}

// GetChargeDetails deve desembrulhar o envelope v2 e devolver o objeto de
// dentro de data — paridade com GetCharge do client legado.
func TestGetChargeDetails_V2Envelope(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transparents/check" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "pix_char_123" {
			t.Errorf("query id = %q, esperado pix_char_123", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":"pix_char_123","status":"PAID","amount":8990},"error":null}`))
	}))
	defer mock.Close()

	t.Setenv("ABACATE_PAY_API_KEY", "test-key")
	t.Setenv("ABACATE_PAY_BASE_URL", mock.URL)

	gw, err := NewGateway()
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}

	charge, err := gw.GetChargeDetails("pix_char_123")
	if err != nil {
		t.Fatalf("GetChargeDetails: %v", err)
	}

	if got := charge["status"]; got != "PAID" {
		t.Errorf("status = %v, esperado PAID", got)
	}
	if got, ok := charge["amount"].(float64); !ok || got != 8990 {
		t.Errorf("amount = %v, esperado 8990", charge["amount"])
	}
}

// Envelope de erro v2 deve virar erro, não objeto com success=false dentro.
func TestGetChargeDetails_EnvelopeErro(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"data":null,"error":"charge not found"}`))
	}))
	defer mock.Close()

	t.Setenv("ABACATE_PAY_API_KEY", "test-key")
	t.Setenv("ABACATE_PAY_BASE_URL", mock.URL)

	gw, err := NewGateway()
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}

	if _, err := gw.GetChargeDetails("pix_char_inexistente"); err == nil {
		t.Fatal("esperava erro para envelope success=false, veio nil")
	}
}
