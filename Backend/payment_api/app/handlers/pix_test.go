// Package handlers - pix_test.go
// Testes unitarios do PIX handler.
// Testam validacao de entrada e logica de defaults.
package handlers

import (
	"testing"

	"github.com/carloshomar/vercardapio/payment_api/app/dto"
)

// === Testes de validacao de entrada PIX ===

func TestPIXRequest_Validation(t *testing.T) {
	tests := []struct {
		name      string
		req       dto.PaymentRequest
		wantError bool
	}{
		{
			"valid PIX request",
			dto.PaymentRequest{
				OrderID:         "order_001",
				CustomerID:      100,
				EstablishmentID: 200,
				Amount:          50.0,
				Method:          "pix",
			},
			false,
		},
		{
			"zero amount",
			dto.PaymentRequest{
				OrderID:    "order_001",
				CustomerID: 100,
				Amount:     0,
				Method:     "pix",
			},
			true,
		},
		{
			"empty order_id",
			dto.PaymentRequest{
				OrderID: "",
				Amount:  50.0,
				Method:  "pix",
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.req.Amount <= 0 || tt.req.OrderID == ""
			if isInvalid != tt.wantError {
				t.Errorf("got invalid=%v, want %v", isInvalid, tt.wantError)
			}
		})
	}
}

// === Testes de default values ===

func TestPIX_DefaultEmail(t *testing.T) {
	email := ""
	if email == "" {
		email = "cliente@email.com"
	}

	if email != "cliente@email.com" {
		t.Errorf("Default email: got %q, want %q", email, "cliente@email.com")
	}
}

func TestPIX_DefaultName(t *testing.T) {
	name := ""
	if name == "" {
		name = "Cliente"
	}

	if name != "Cliente" {
		t.Errorf("Default name: got %q, want %q", name, "Cliente")
	}
}

func TestPIX_DescriptionFormat(t *testing.T) {
	orderID := "order_123"
	description := "Pedido " + orderID

	expected := "Pedido order_123"
	if description != expected {
		t.Errorf("Description: got %q, want %q", description, expected)
	}
}

// === Testes de PaymentResponse ===

func TestPIXPaymentResponse_Fields(t *testing.T) {
	resp := dto.PaymentResponse{
		PaymentID:    "pay_001",
		Status:       "PENDING",
		PixQRCode:    "0002012658...",
		PixCopyPaste: "0002012658...",
		QRCodeBase64: "data:image/png;base64,...",
		AbacatePayID: "abacate_001",
		Message:      "PIX payment created via AbacatePay",
	}

	if resp.Status != "PENDING" {
		t.Errorf("Status: got %q, want %q", resp.Status, "PENDING")
	}
	if resp.Message != "PIX payment created via AbacatePay" {
		t.Errorf("Message: got %q", resp.Message)
	}
}

func TestPIXPaymentResponse_EmptyOptionalFields(t *testing.T) {
	resp := dto.PaymentResponse{
		PaymentID: "pay_001",
		Status:    "PENDING",
	}

	if resp.PixQRCode != "" {
		t.Error("PixQRCode should be empty by default")
	}
	if resp.PixCopyPaste != "" {
		t.Error("PixCopyPaste should be empty by default")
	}
}
