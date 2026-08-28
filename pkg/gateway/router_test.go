package gateway

import (
	"testing"
)

func TestRouter_StrategyOrdered(t *testing.T) {
	// Test that StrategyOrdered is a valid strategy
	router := &Router{}
	router.SetStrategy(StrategyOrdered)
	
	if router.strategy != StrategyOrdered {
		t.Errorf("expected strategy to be StrategyOrdered")
	}
}

func TestRouter_StrategyFallback(t *testing.T) {
	router := &Router{}
	router.SetStrategy(StrategyFallback)
	
	if router.strategy != StrategyFallback {
		t.Errorf("expected strategy to be StrategyFallback")
	}
}

func TestRouter_AddGateway(t *testing.T) {
	router := &Router{}
	
	// Add nil gateway should not panic
	router.AddGateway(nil)
	
	if len(router.gateways) != 1 {
		t.Errorf("expected 1 gateway (nil), got %d", len(router.gateways))
	}
}

func TestTransactionRequest_Validation(t *testing.T) {
	req := &TransactionRequest{
		Amount:   1000,
		Currency: "BRL",
		Method:   MethodPIX,
	}
	
	if req.Amount != 1000 {
		t.Errorf("expected amount 1000, got %d", req.Amount)
	}
	if req.Currency != "BRL" {
		t.Errorf("expected currency BRL, got %s", req.Currency)
	}
	if req.Method != MethodPIX {
		t.Errorf("expected method PIX, got %s", req.Method)
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
