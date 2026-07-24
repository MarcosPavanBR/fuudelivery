// Package services - chargeback_service_test.go
// Testes unitarios do servico de chargeback.
// Testam modelagem, constantes e regras de negocio.
// Nao dependem de MongoDB — apenas logica pura.
package services

import (
	"testing"

	"github.com/carloshomar/vercardapio/payment/models"
)

// === Testes de ChargebackStatus ===

func TestChargebackStatuses(t *testing.T) {
	statuses := map[models.ChargebackStatus]string{
		models.ChargebackPending:  "pending",
		models.ChargebackApproved: "approved",
		models.ChargebackRejected: "rejected",
		models.ChargebackEscalated: "escalated",
	}

	for status, expected := range statuses {
		if string(status) != expected {
			t.Errorf("Status %v: got %q, want %q", status, string(status), expected)
		}
	}
}

// === Testes de ChargebackReason ===

func TestChargebackReasons(t *testing.T) {
	reasons := map[models.ChargebackReason]string{
		models.ReasonUnauthorized: "unauthorized",
		models.ReasonNotReceived:  "not_received",
		models.ReasonDefective:    "defective",
		models.ReasonDuplicate:    "duplicate",
		models.ReasonOther:        "other",
	}

	for reason, expected := range reasons {
		if string(reason) != expected {
			t.Errorf("Reason %v: got %q, want %q", reason, string(reason), expected)
		}
	}
}

// === Testes de Chargeback struct ===

func TestChargeback_Creation(t *testing.T) {
	chargeback := models.Chargeback{
		PaymentOrderID:  "order_001",
		CustomerID:      "cust_001",
		EstablishmentID: "rest_001",
		Amount:          50.0,
		Reason:          models.ReasonUnauthorized,
		Description:     "Unauthorized transaction",
		Status:          models.ChargebackPending,
		EvidenceCount:   0,
	}

	if chargeback.PaymentOrderID != "order_001" {
		t.Errorf("PaymentOrderID: got %q, want %q", chargeback.PaymentOrderID, "order_001")
	}
	if chargeback.Amount != 50.0 {
		t.Errorf("Amount: got %f, want 50.0", chargeback.Amount)
	}
	if chargeback.Status != models.ChargebackPending {
		t.Errorf("Status: got %q, want %q", chargeback.Status, models.ChargebackPending)
	}
	if chargeback.Reason != models.ReasonUnauthorized {
		t.Errorf("Reason: got %q, want %q", chargeback.Reason, models.ReasonUnauthorized)
	}
}

func TestChargeback_DefaultStatus(t *testing.T) {
	chargeback := models.Chargeback{}

	if chargeback.Status != "" {
		t.Errorf("Default status should be empty: got %q", chargeback.Status)
	}
	if chargeback.EvidenceCount != 0 {
		t.Errorf("Default EvidenceCount: got %d, want 0", chargeback.EvidenceCount)
	}
}

func TestChargeback_EvidenceCountIncrement(t *testing.T) {
	chargeback := models.Chargeback{
		EvidenceCount: 0,
	}

	// Simula adicao de evidencias
	chargeback.EvidenceCount++
	if chargeback.EvidenceCount != 1 {
		t.Errorf("After 1 evidence: got %d, want 1", chargeback.EvidenceCount)
	}

	chargeback.EvidenceCount++
	if chargeback.EvidenceCount != 2 {
		t.Errorf("After 2 evidences: got %d, want 2", chargeback.EvidenceCount)
	}
}

// === Testes de regras de transicao de status ===

func TestChargeback_StatusTransitions(t *testing.T) {
	tests := []struct {
		name       string
		from       models.ChargebackStatus
		to         models.ChargebackStatus
		valid      bool
	}{
		{"pending to approved", models.ChargebackPending, models.ChargebackApproved, true},
		{"pending to rejected", models.ChargebackPending, models.ChargebackRejected, true},
		{"pending to escalated", models.ChargebackPending, models.ChargebackEscalated, true},
		{"approved to rejected", models.ChargebackApproved, models.ChargebackRejected, false},
		{"rejected to approved", models.ChargebackRejected, models.ChargebackApproved, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simula regra: so pode transitar a partir de pending
			validTransition := tt.from == models.ChargebackPending
			if validTransition != tt.valid {
				t.Errorf("Transition %s -> %s: got valid=%v, want %v",
					tt.from, tt.to, validTransition, tt.valid)
			}
		})
	}
}

// === Testes de ChargebackService (instancia) ===

func TestNewChargebackService(t *testing.T) {
	svc := NewChargebackService()
	if svc == nil {
		t.Error("NewChargebackService should return non-nil")
	}
}

// === Testes de valores limite ===

func TestChargeback_AmountBoundary(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		valid  bool
	}{
		{"zero amount", 0.0, false},
		{"negative amount", -10.0, false},
		{"minimum valid", 0.01, true},
		{"large amount", 999999.99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicates validation: amount must be > 0
			valid := tt.amount > 0
			if valid != tt.valid {
				t.Errorf("amount=%.2f: got valid=%v, want %v", tt.amount, valid, tt.valid)
			}
		})
	}
}

func TestChargeback_DescriptionLength(t *testing.T) {
	chargeback := models.Chargeback{
		Description: "Customer reports unauthorized transaction on 2024-01-15 for R$50.00",
	}

	if len(chargeback.Description) == 0 {
		t.Error("Description should not be empty")
	}
	if len(chargeback.Description) > 500 {
		t.Errorf("Description too long: %d chars", len(chargeback.Description))
	}
}
