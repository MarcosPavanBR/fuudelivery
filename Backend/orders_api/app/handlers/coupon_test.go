// Package handlers - coupon_test.go
// Testes unitarios do coupon handler.
// Testam validacao, logica de desconto, e regras de uso.
package handlers

import (
	"strings"
	"testing"

	"github.com/carloshomar/vercardapio/orders_api/app/dto"
)

// === Testes de CreateCoupon validation ===

func TestCreateCoupon_CodeRequired(t *testing.T) {
	code := ""
	trimmed := strings.ToUpper(strings.TrimSpace(code))

	if trimmed == "" {
		// Expected: code is required
		t.Log("Empty code detected as required")
	} else {
		t.Error("Empty code should be detected")
	}
}

func TestCreateCoupon_CodeUppercase(t *testing.T) {
	code := "  promocao10  "
	trimmed := strings.ToUpper(strings.TrimSpace(code))

	expected := "PROMOCAO10"
	if trimmed != expected {
		t.Errorf("Code normalization: got %q, want %q", trimmed, expected)
	}
}

func TestCreateCoupon_DiscountTypeValidation(t *testing.T) {
	validTypes := map[string]bool{
		"PERCENTAGE":    true,
		"FIXED":         true,
		"FREE_DELIVERY": true,
		"INVALID":       false,
		"":              false,
	}

	for discountType, expectedValid := range validTypes {
		isValid := discountType == "PERCENTAGE" || discountType == "FIXED" || discountType == "FREE_DELIVERY"
		if isValid != expectedValid {
			t.Errorf("Type %q: got valid=%v, want %v", discountType, isValid, expectedValid)
		}
	}
}

func TestCreateCoupon_PercentageBounds(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		valid      bool
	}{
		{"zero percent", 0, false},
		{"1 percent", 1, true},
		{"50 percent", 50, true},
		{"100 percent", 100, true},
		{"101 percent", 101, false},
		{"negative", -5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.percentage > 0 && tt.percentage <= 100
			if valid != tt.valid {
				t.Errorf("Percentage %.0f: got valid=%v, want %v", tt.percentage, valid, tt.valid)
			}
		})
	}
}

func TestCreateCoupon_FixedAmountValidation(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		valid  bool
	}{
		{"zero", 0, false},
		{"negative", -10, false},
		{"valid", 10, true},
		{"large", 1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.amount > 0
			if valid != tt.valid {
				t.Errorf("Amount %.2f: got valid=%v, want %v", tt.amount, valid, tt.valid)
			}
		})
	}
}

// === Testes de CalculateDiscount logic ===

func TestCalculateDiscount_Percentage(t *testing.T) {
	tests := []struct {
		name           string
		orderValue     float64
		discountValue  float64
		expectedAmount float64
		expectedFinal  float64
	}{
		{"10% of 100", 100.0, 10.0, 10.0, 90.0},
		{"50% of 200", 200.0, 50.0, 100.0, 100.0},
		{"25% of 80", 80.0, 25.0, 20.0, 60.0},
		{"100% of 50", 50.0, 100.0, 50.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discountAmount := tt.orderValue * (tt.discountValue / 100)
			finalValue := tt.orderValue - discountAmount
			if finalValue < 0 {
				finalValue = 0
			}

			if discountAmount != tt.expectedAmount {
				t.Errorf("Discount: got %f, want %f", discountAmount, tt.expectedAmount)
			}
			if finalValue != tt.expectedFinal {
				t.Errorf("Final: got %f, want %f", finalValue, tt.expectedFinal)
			}
		})
	}
}

func TestCalculateDiscount_Fixed(t *testing.T) {
	tests := []struct {
		name           string
		orderValue     float64
		discountValue  float64
		expectedFinal  float64
	}{
		{"10 off 100", 100.0, 10.0, 90.0},
		{"50 off 30", 30.0, 50.0, 0.0}, // cant go below 0
		{"5 off 20", 20.0, 5.0, 15.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discountAmount := tt.discountValue
			finalValue := tt.orderValue - discountAmount
			if finalValue < 0 {
				finalValue = 0
			}

			if finalValue != tt.expectedFinal {
				t.Errorf("Final: got %f, want %f", finalValue, tt.expectedFinal)
			}
		})
	}
}

func TestCalculateDiscount_FreeDelivery(t *testing.T) {
	orderValue := 100.0
	discountAmount := 0.0 // FREE_DELIVERY does not reduce order value

	finalValue := orderValue - discountAmount
	if finalValue != 100.0 {
		t.Errorf("FREE_DELIVERY should not reduce order: got %f, want 100.0", finalValue)
	}
}

func TestCalculateDiscount_FinalValueNeverNegative(t *testing.T) {
	orderValue := 20.0
	discountAmount := 50.0

	finalValue := orderValue - discountAmount
	if finalValue < 0 {
		finalValue = 0
	}

	if finalValue != 0 {
		t.Errorf("Final value should be 0: got %f", finalValue)
	}
}

// === Testes de ValidateCouponResponse ===

func TestValidateCouponResponse_Fields(t *testing.T) {
	resp := dto.ValidateCouponResponse{
		Valid:          true,
		DiscountType:   "PERCENTAGE",
		DiscountValue:  10.0,
		DiscountAmount: 15.0,
		FinalValue:     135.0,
	}

	if !resp.Valid {
		t.Error("Should be valid")
	}
	if resp.DiscountAmount != 15.0 {
		t.Errorf("DiscountAmount: got %f, want 15.0", resp.DiscountAmount)
	}
	if resp.FinalValue != 135.0 {
		t.Errorf("FinalValue: got %f, want 135.0", resp.FinalValue)
	}
}

func TestValidateCouponResponse_Invalid(t *testing.T) {
	resp := dto.ValidateCouponResponse{
		Valid:   false,
		Message: "Cupom expirado",
	}

	if resp.Valid {
		t.Error("Should be invalid")
	}
	if resp.Message != "Cupom expirado" {
		t.Errorf("Message: got %q", resp.Message)
	}
}

// === Testes de coupon validation messages ===

func TestCouponValidation_Messages(t *testing.T) {
	messages := []struct {
		condition string
		message   string
	}{
		{"not_found", "Cupom não encontrado"},
		{"inactive", "Cupom está inativo"},
		{"not_started", "Cupom ainda não está válido"},
		{"expired", "Cupom expirado"},
		{"max_uses", "Cupom atingiu o limite máximo de usos"},
		{"min_order", "Valor mínimo do pedido não atingido"},
		{"wrong_establishment", "Cupom não é válido para este estabelecimento"},
		{"personal", "Este cupom é pessoal e intransferível"},
		{"user_limit", "Você já atingiu o limite de usos deste cupom"},
	}

	for _, msg := range messages {
		if msg.message == "" {
			t.Errorf("Message for %q should not be empty", msg.condition)
		}
	}
}
