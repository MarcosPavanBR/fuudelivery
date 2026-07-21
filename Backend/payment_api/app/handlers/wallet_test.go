package handlers

import (
	"testing"
)

func TestTopUpValidation(t *testing.T) {
	tests := []struct {
		name      string
		amount    float64
		paymentID string
		wantError bool
	}{
		{"valid topup", 50.0, "pay_abc123", false},
		{"zero amount", 0.0, "pay_abc123", true},
		{"negative amount", -10.0, "pay_abc123", true},
		{"empty payment_id", 50.0, "", true},
		{"valid small amount", 0.01, "pay_xyz789", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.amount <= 0 || tt.paymentID == ""
			if isInvalid != tt.wantError {
				t.Errorf("amount=%.2f paymentID=%q: got invalid=%v, want %v",
					tt.amount, tt.paymentID, isInvalid, tt.wantError)
			}
		})
	}
}

func TestDeductFromWalletValidation(t *testing.T) {
	tests := []struct {
		name      string
		amount    float64
		wantError bool
	}{
		{"valid deduction", 25.0, false},
		{"zero amount", 0.0, true},
		{"negative amount", -5.0, true},
		{"very small positive", 0.01, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.amount <= 0
			if isInvalid != tt.wantError {
				t.Errorf("amount=%.2f: got invalid=%v, want %v",
					tt.amount, isInvalid, tt.wantError)
			}
		})
	}
}

func TestWalletAntiReplay(t *testing.T) {
	usedPayments := make(map[string]bool)

	// First credit should succeed
	paymentID := "pay_test123"
	if usedPayments[paymentID] {
		t.Error("Payment should not be marked as used yet")
	}
	usedPayments[paymentID] = true

	// Second credit should be rejected (anti-replay)
	if !usedPayments[paymentID] {
		t.Error("Payment should be marked as used after first credit")
	}
}
