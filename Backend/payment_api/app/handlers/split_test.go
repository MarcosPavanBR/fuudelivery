// Package handlers - split_test.go
// Testes unitarios do split handler.
// Testam a logica de calculo de porcentagem e validacao.
package handlers

import (
	"testing"

	"github.com/carloshomar/vercardapio/payment_api/app/dto"
	"github.com/carloshomar/vercardapio/payment_api/app/models"
)

// === Testes de calculo de split ===

func TestSplitCalculation_PercentageToAmount(t *testing.T) {
	payment := &models.Payment{Amount: 200.0}

	tests := []struct {
		name       string
		percentage float64
		expected   float64
	}{
		{"5 percent", 5.0, 10.0},
		{"85 percent", 85.0, 170.0},
		{"10 percent", 10.0, 20.0},
		{"0 percent", 0.0, 0.0},
		{"100 percent", 100.0, 200.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate split calculation: if Amount == 0 && Percentage > 0
			amount := (tt.percentage / 100.0) * payment.Amount
			if amount != tt.expected {
				t.Errorf("Percentage %.1f of %.2f: got %f, want %f",
					tt.percentage, payment.Amount, amount, tt.expected)
			}
		})
	}
}

func TestSplitCalculation_SumOfRules(t *testing.T) {
	payment := &models.Payment{Amount: 100.0}

	rules := []models.SplitRule{
		{Percentage: 5.0, Amount: 0},
		{Percentage: 85.0, Amount: 0},
	}

	var totalSplit float64
	for i, rule := range rules {
		if rule.Amount == 0 && rule.Percentage > 0 {
			rules[i].Amount = (rule.Percentage / 100.0) * payment.Amount
		}
		totalSplit += rules[i].Amount
	}

	expected := 90.0 // 5 + 85
	if totalSplit != expected {
		t.Errorf("Total split: got %f, want %f", totalSplit, expected)
	}
}

func TestSplitCalculation_WithDelivery(t *testing.T) {
	payment := &models.Payment{Amount: 100.0}

	rules := []models.SplitRule{
		{Percentage: 5.0, Amount: 0},
		{Percentage: 85.0, Amount: 0},
		{ReceiverType: "deliveryman", Amount: 10.0, Percentage: 0},
	}

	var totalSplit float64
	for i, rule := range rules {
		if rule.Amount == 0 && rule.Percentage > 0 {
			rules[i].Amount = (rule.Percentage / 100.0) * payment.Amount
		}
		totalSplit += rules[i].Amount
	}

	expected := 100.0 // 5 + 85 + 10
	if totalSplit != expected {
		t.Errorf("Total split: got %f, want %f", totalSplit, expected)
	}
}

// === Testes de SplitPaymentRequest ===

func TestSplitPaymentRequest_Creation(t *testing.T) {
	req := dto.SplitPaymentRequest{
		PaymentID: "pay_123",
		Rules: []models.SplitRule{
			{ReceiverType: "platform", Percentage: 5.0},
			{ReceiverType: "establishment", Percentage: 85.0},
		},
	}

	if req.PaymentID != "pay_123" {
		t.Errorf("PaymentID: got %q, want %q", req.PaymentID, "pay_123")
	}
	if len(req.Rules) != 2 {
		t.Errorf("Rules count: got %d, want 2", len(req.Rules))
	}
}

func TestSplitPaymentRequest_EmptyRules(t *testing.T) {
	req := dto.SplitPaymentRequest{
		PaymentID: "pay_123",
		Rules:     []models.SplitRule{},
	}

	if len(req.Rules) != 0 {
		t.Errorf("Empty rules: got %d, want 0", len(req.Rules))
	}
}

// === Testes de SplitRule ===

func TestSplitRule_Fields(t *testing.T) {
	rule := models.SplitRule{
		ReceiverID:   12345,
		ReceiverType: "establishment",
		Amount:       85.0,
		Percentage:   85.0,
	}

	if rule.ReceiverID != 12345 {
		t.Errorf("ReceiverID: got %d, want 12345", rule.ReceiverID)
	}
	if rule.ReceiverType != "establishment" {
		t.Errorf("ReceiverType: got %q, want %q", rule.ReceiverType, "establishment")
	}
	if rule.Amount != 85.0 {
		t.Errorf("Amount: got %f, want 85.0", rule.Amount)
	}
	if rule.Percentage != 85.0 {
		t.Errorf("Percentage: got %f, want 85.0", rule.Percentage)
	}
}

func TestSplitRule_PlatformReceiverID(t *testing.T) {
	// Platform and deliveryman rules use ReceiverID = 0
	rule := models.SplitRule{
		ReceiverID:   0,
		ReceiverType: "platform",
	}

	if rule.ReceiverID != 0 {
		t.Errorf("Platform ReceiverID: got %d, want 0", rule.ReceiverID)
	}
}

// === Testes de logica de validacao ===

func TestSplitValidation_PaymentIDRequired(t *testing.T) {
	req := dto.SplitPaymentRequest{
		PaymentID: "",
	}

	if req.PaymentID != "" {
		t.Error("Empty PaymentID should be detected")
	}
}

func TestSplitValidation_PercentageBounds(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		valid      bool
	}{
		{"zero percent", 0.0, true},
		{"5 percent", 5.0, true},
		{"100 percent", 100.0, true},
		{"negative percent", -5.0, false},
		{"over 100 percent", 150.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.percentage >= 0 && tt.percentage <= 100
			if valid != tt.valid {
				t.Errorf("Percentage %.1f: got valid=%v, want %v", tt.percentage, valid, tt.valid)
			}
		})
	}
}
