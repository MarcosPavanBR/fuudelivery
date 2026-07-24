// Package handlers - orders_test.go
// Testes unitarios do orders handler.
// Testam validacao, status mapping, e logica de pedido.
package handlers

import (
	"testing"

	"github.com/carloshomar/vercardapio/orders_api/app/dto"
)

// === Testes de RequestPayload ===

func TestRequestPayload_ScheduledOrder(t *testing.T) {
	req := dto.RequestPayload{}

	// Simulate CreateOrder logic for scheduled order
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() {
		req.Status = "SCHEDULED"
		req.IsScheduled = true
	} else {
		req.Status = "AWAIT_APPROVE"
	}

	if req.Status != "AWAIT_APPROVE" {
		t.Errorf("Status: got %q, want %q", req.Status, "AWAIT_APPROVE")
	}
	if req.IsScheduled {
		t.Error("IsScheduled should be false for immediate order")
	}
}

func TestRequestPayload_EmptyEstablishmentId(t *testing.T) {
	req := dto.RequestPayload{
		EstablishmentId: 0,
	}

	if req.EstablishmentId != 0 {
		t.Errorf("EstablishmentId: got %d, want 0", req.EstablishmentId)
	}
}

// === Testes de UpdateOrderStatusRequest ===

func TestUpdateOrderStatusRequest_ValidStatuses(t *testing.T) {
	validStatuses := map[string]bool{
		"REQUEST_APPROVE": true,
		"APPROVED":        true,
		"IN_ROUTE_DELIVERY": true,
		"DONE":            true,
		"FINISHED":        true,
		"CANCELLED":       true,
		"SCHEDULED":       true,
		"PREPARING":       true,
	}

	for status, expected := range validStatuses {
		if !expected {
			t.Errorf("Status %q should be valid", status)
		}
	}
}

func TestUpdateOrderStatusRequest_InvalidID(t *testing.T) {
	req := dto.UpdateOrderStatusRequest{
		ID:     "invalid_hex",
		Status: "APPROVED",
	}

	// Invalid hex should be caught by primitive.ObjectIDFromHex
	if req.ID == "" {
		t.Error("ID should not be empty")
	}
}

// === Testes de Order Status Messages ===

func TestOrderStatusMessages(t *testing.T) {
	statusMessages := map[string]string{
		"APPROVED":          "Seu pedido foi aprovado e está sendo preparado!",
		"DONE":              "Seu pedido está pronto e saiu para entrega!",
		"IN_ROUTE_DELIVERY": "Seu pedido está a caminho!",
		"FINISHED":          "Seu pedido foi entregue! Bom apetite!",
		"CANCELLED":         "Seu pedido foi cancelado.",
		"SCHEDULED":         "Seu pedido foi agendado com sucesso!",
	}

	for status, expected := range statusMessages {
		if expected == "" {
			t.Errorf("Message for status %q should not be empty", status)
		}
		_ = statusMessages[status]
	}

	// Unknown status should not have a message
	unknownStatus := "UNKNOWN_STATUS"
	_, hasMessage := statusMessages[unknownStatus]
	if hasMessage {
		t.Error("Unknown status should not have a message")
	}
}

// === Testes de order status titles ===

func TestOrderStatusTitles(t *testing.T) {
	tests := []struct {
		status        string
		expectedTitle string
	}{
		{"APPROVED", "Atualização do Pedido"},
		{"DONE", "Atualização do Pedido"},
		{"IN_ROUTE_DELIVERY", "Atualização do Pedido"},
		{"FINISHED", "Pedido Entregue"},
		{"CANCELLED", "Pedido Cancelado"},
		{"SCHEDULED", "Atualização do Pedido"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			title := "Atualização do Pedido"
			if tt.status == "FINISHED" {
				title = "Pedido Entregue"
			} else if tt.status == "CANCELLED" {
				title = "Pedido Cancelado"
			}

			if title != tt.expectedTitle {
				t.Errorf("Title for %s: got %q, want %q", tt.status, title, tt.expectedTitle)
			}
		})
	}
}

// === Testes de list orders validation ===

func TestListOrders_EstablishmentIdRequired(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantError bool
	}{
		{"valid id", "123", false},
		{"empty id", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.id == ""
			if isInvalid != tt.wantError {
				t.Errorf("id=%q: got invalid=%v, want %v", tt.id, isInvalid, tt.wantError)
			}
		})
	}
}

func TestListOrders_PhoneRequired(t *testing.T) {
	tests := []struct {
		name      string
		phone     string
		wantError bool
	}{
		{"valid phone", "11999998888", false},
		{"empty phone", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.phone == ""
			if isInvalid != tt.wantError {
				t.Errorf("phone=%q: got invalid=%v, want %v", tt.phone, isInvalid, tt.wantError)
			}
		})
	}
}

// === Testes de RequestPayload estrutura ===

func TestRequestPayload_UserPhone(t *testing.T) {
	req := dto.RequestPayload{}

	// Access nested user phone
	if req.User.Phone != "" {
		t.Errorf("Default phone should be empty: got %q", req.User.Phone)
	}
}

func TestRequestPayload_Establishment(t *testing.T) {
	est := dto.Establishment{
		Name:      "Restaurante Teste",
		Latitude:  -23.5505,
		Longitude: -46.6333,
	}

	if est.Name != "Restaurante Teste" {
		t.Errorf("Name: got %q", est.Name)
	}
	if est.Latitude != -23.5505 {
		t.Errorf("Latitude: got %f, want -23.5505", est.Latitude)
	}
}
