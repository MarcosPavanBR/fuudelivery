package gateway

import (
	"context"
	"testing"
)

// mockGateway implements the Gateway interface for testing.
type mockGateway struct {
	name string
}

func (m *mockGateway) Name() string                      { return m.name }
func (m *mockGateway) SupportsMethod(PaymentMethod) bool { return true }
func (m *mockGateway) SupportsSplit() bool               { return false }
func (m *mockGateway) SupportsPreAuth() bool             { return false }
func (m *mockGateway) Supports3DS() bool                 { return false }
func (m *mockGateway) SupportsEscrow() bool              { return false }
func (m *mockGateway) MaxSplitRecipients() int           { return 0 }
func (m *mockGateway) CreateTransaction(context.Context, *TransactionRequest) (*TransactionResponse, error) {
	return &TransactionResponse{Gateway: m.name, Status: StatusPaid}, nil
}
func (m *mockGateway) CaptureTransaction(context.Context, string, int64) error { return nil }
func (m *mockGateway) RefundTransaction(context.Context, string, int64) (*RefundResponse, error) {
	return nil, nil
}
func (m *mockGateway) VoidTransaction(context.Context, string) error { return nil }
func (m *mockGateway) GetTransactionStatus(context.Context, string) (TransactionStatus, error) {
	return StatusPaid, nil
}
func (m *mockGateway) CreateRecipient(context.Context, *RecipientRequest) (*RecipientResponse, error) {
	return nil, nil
}
func (m *mockGateway) UpdateRecipient(context.Context, string, *RecipientRequest) error { return nil }
func (m *mockGateway) GetRecipientBalance(context.Context, string) (int64, int64, error) {
	return 0, 0, nil
}
func (m *mockGateway) ValidateWebhook([]byte, map[string]string) bool { return true }
func (m *mockGateway) ParseWebhook([]byte) (*WebhookEvent, error) {
	return &WebhookEvent{Gateway: m.name, EventType: "payment.paid", Status: StatusPaid}, nil
}

func TestRouter_StrategyOrdered(t *testing.T) {
	gw := &mockGateway{name: "mock"}
	router := NewRouter(gw)
	router.SetStrategy(StrategyOrdered)

	if router.strategy != StrategyOrdered {
		t.Errorf("expected strategy to be StrategyOrdered")
	}
}

func TestTransactionRequest_Fields(t *testing.T) {
	req := &TransactionRequest{
		OrderID:       12345,
		Amount:        1000,
		Currency:      "BRL",
		PaymentMethod: MethodPIX,
	}

	if req.Amount != 1000 {
		t.Errorf("expected amount 1000, got %d", req.Amount)
	}
	if req.Currency != "BRL" {
		t.Errorf("expected currency BRL, got %s", req.Currency)
	}
	if req.PaymentMethod != MethodPIX {
		t.Errorf("expected method PIX, got %s", req.PaymentMethod)
	}
}

func TestTransactionResponse_DefaultValues(t *testing.T) {
	resp := &TransactionResponse{}

	if resp.Status != "" {
		t.Errorf("expected empty status, got %s", resp.Status)
	}
	if resp.GatewayID != "" {
		t.Errorf("expected empty GatewayID, got %s", resp.GatewayID)
	}
	if resp.Gateway != "" {
		t.Errorf("expected empty Gateway, got %s", resp.Gateway)
	}
}

func TestRouter_NewWithMultipleGateways(t *testing.T) {
	gw1 := &mockGateway{name: "primary"}
	gw2 := &mockGateway{name: "fallback"}
	router := NewRouter(gw1, gw2)

	if len(router.gateways) != 2 {
		t.Errorf("expected 2 gateways, got %d", len(router.gateways))
	}
}
