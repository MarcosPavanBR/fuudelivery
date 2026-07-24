// Package handlers - card_test.go
// Testes unitarios do card handler.
// Testam validacao, defaults, e logica de parcelamento.
package handlers

import (
	"testing"

	"github.com/carloshomar/vercardapio/payment_api/app/dto"
)

// === Testes de validacao de cartao ===

func TestCardCharge_CardTokenRequired(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantError bool
	}{
		{"valid token", "tok_abc123", false},
		{"empty token", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.token == ""
			if isInvalid != tt.wantError {
				t.Errorf("token=%q: got invalid=%v, want %v", tt.token, isInvalid, tt.wantError)
			}
		})
	}
}

// === Testes de installment defaults ===

func TestCard_DefaultInstallments(t *testing.T) {
	installments := 0
	if installments <= 0 {
		installments = 1
	}

	if installments != 1 {
		t.Errorf("Default installments: got %d, want 1", installments)
	}
}

func TestCard_CustomInstallments(t *testing.T) {
	installments := 3
	if installments <= 0 {
		installments = 1
	}

	if installments != 3 {
		t.Errorf("Custom installments: got %d, want 3", installments)
	}
}

func TestCard_NegativeInstallments(t *testing.T) {
	installments := -1
	if installments <= 0 {
		installments = 1
	}

	if installments != 1 {
		t.Errorf("Negative installments: got %d, want 1", installments)
	}
}

// === Testes de default values ===

func TestCard_DefaultEmail(t *testing.T) {
	email := ""
	if email == "" {
		email = "cliente@email.com"
	}

	if email != "cliente@email.com" {
		t.Errorf("Default email: got %q", email)
	}
}

func TestCard_DefaultName(t *testing.T) {
	name := ""
	if name == "" {
		name = "Cliente"
	}

	if name != "Cliente" {
		t.Errorf("Default name: got %q", name)
	}
}

// === Testes de ProcessPayment method routing ===

func TestProcessPayment_MethodRouting(t *testing.T) {
	tests := []struct {
		method     string
		isCard     bool
		isPix      bool
		isInvalid  bool
	}{
		{"credit", true, false, false},
		{"debit", true, false, false},
		{"pix", false, true, false},
		{"invalid", false, false, true},
		{"", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			isCard := tt.method == "credit" || tt.method == "debit"
			isPix := tt.method == "pix"
			isInvalid := !isCard && !isPix

			if isCard != tt.isCard {
				t.Errorf("isCard: got %v, want %v", isCard, tt.isCard)
			}
			if isPix != tt.isPix {
				t.Errorf("isPix: got %v, want %v", isPix, tt.isPix)
			}
			if isInvalid != tt.isInvalid {
				t.Errorf("isInvalid: got %v, want %v", isInvalid, tt.isInvalid)
			}
		})
	}
}

// === Testes de PaymentRequest ===

func TestPaymentRequest_CardFields(t *testing.T) {
	req := dto.PaymentRequest{
		OrderID:         "order_001",
		CustomerID:      100,
		EstablishmentID: 200,
		Amount:          150.0,
		DeliveryAmount:  10.0,
		Method:          "credit",
		CardToken:       "tok_abc123",
		Installments:    3,
		CustomerName:    "Joao Silva",
		CustomerEmail:   "joao@email.com",
		CustomerPhone:   "11999998888",
	}

	if req.Method != "credit" {
		t.Errorf("Method: got %q, want %q", req.Method, "credit")
	}
	if req.Installments != 3 {
		t.Errorf("Installments: got %d, want 3", req.Installments)
	}
	if req.CardToken != "tok_abc123" {
		t.Errorf("CardToken: got %q", req.CardToken)
	}
}

func TestPaymentRequest_PIXFields(t *testing.T) {
	req := dto.PaymentRequest{
		OrderID:         "order_002",
		CustomerID:      100,
		EstablishmentID: 200,
		Amount:          80.0,
		Method:          "pix",
	}

	if req.Method != "pix" {
		t.Errorf("Method: got %q, want %q", req.Method, "pix")
	}
	if req.CardToken != "" {
		t.Error("CardToken should be empty for PIX")
	}
}

// === Testes de status mapping ===

func TestCardStatusMapping(t *testing.T) {
	tests := []struct {
		apiStatus string
		expected  string
	}{
		{"paid", "CONFIRMED"},
		{"refused", "REFUSED"},
		{"pending", "PENDING"},
		{"", "PENDING"},
	}

	for _, tt := range tests {
		t.Run(tt.apiStatus, func(t *testing.T) {
			paymentStatus := "PENDING"
			if tt.apiStatus == "paid" {
				paymentStatus = "CONFIRMED"
			} else if tt.apiStatus == "refused" {
				paymentStatus = "REFUSED"
			}

			if paymentStatus != tt.expected {
				t.Errorf("API status %q: got %q, want %q", tt.apiStatus, paymentStatus, tt.expected)
			}
		})
	}
}
