// Package services - responsibility_chain_test.go
// Testes unitarios do padrao Chain of Responsibility.
// Testam a logica pura de validacao, aprovacao, e encadeamento.
// RiskCheckHandler nao e testado aqui porque depende de MongoDB
// (via checkFrequency/checkEstablishmentHistory).
package services

import (
	"testing"

	"github.com/carloshomar/vercardapio/payment/models"
)

// === Testes do ValidationHandler ===

func TestValidationHandler_EmptyOrderID(t *testing.T) {
	handler := &ValidationHandler{}
	payment := &models.Payment{
		OrderID: "",
		Amount:  100.0,
	}

	err := handler.Handle(payment)
	if err != nil {
		t.Errorf("ValidationHandler empty OrderID: expected nil, got %v", err)
	}
	if payment.Status != "" {
		t.Errorf("Status should not be set for invalid payment: got %q", payment.Status)
	}
}

func TestValidationHandler_ZeroAmount(t *testing.T) {
	handler := &ValidationHandler{}
	payment := &models.Payment{
		OrderID: "order_001",
		Amount:  0,
	}

	err := handler.Handle(payment)
	if err != nil {
		t.Errorf("ValidationHandler zero amount: expected nil, got %v", err)
	}
}

func TestValidationHandler_NegativeAmount(t *testing.T) {
	handler := &ValidationHandler{}
	payment := &models.Payment{
		OrderID: "order_001",
		Amount:  -50.0,
	}

	err := handler.Handle(payment)
	if err != nil {
		t.Errorf("ValidationHandler negative amount: expected nil, got %v", err)
	}
}

func TestValidationHandler_EmptyCustomerID(t *testing.T) {
	handler := &ValidationHandler{}
	payment := &models.Payment{
		OrderID:    "order_001",
		Amount:     100.0,
		CustomerID: "",
	}

	err := handler.Handle(payment)
	if err != nil {
		t.Errorf("ValidationHandler empty CustomerID: expected nil, got %v", err)
	}
}

func TestValidationHandler_PassesValidToNext(t *testing.T) {
	handler := &ValidationHandler{}
	nextHandler := &mockHandler{called: false}
	handler.SetNext(nextHandler)

	payment := &models.Payment{
		OrderID:    "order_001",
		Amount:     100.0,
		CustomerID: "cust_001",
	}

	err := handler.Handle(payment)
	if err != nil {
		t.Errorf("ValidationHandler valid: expected nil, got %v", err)
	}
	if !nextHandler.called {
		t.Error("ValidationHandler should pass valid payment to next handler")
	}
}

func TestValidationHandler_InvalidDoesNotPassToNext(t *testing.T) {
	handler := &ValidationHandler{}
	nextHandler := &mockHandler{called: false}
	handler.SetNext(nextHandler)

	payment := &models.Payment{
		OrderID: "",
		Amount:  100.0,
	}

	handler.Handle(payment)
	if nextHandler.called {
		t.Error("Invalid payment should NOT pass to next handler")
	}
}

// === Testes do ApprovalHandler ===

func TestApprovalHandler_RequiresApproval(t *testing.T) {
	handler := &ApprovalHandler{}
	payment := &models.Payment{
		RequiresApproval: true,
	}

	err := handler.Handle(payment)
	if err != nil {
		t.Errorf("ApprovalHandler requires approval: expected nil, got %v", err)
	}
	if payment.Status != models.PaymentPending {
		t.Errorf("Status: got %q, want %q", payment.Status, models.PaymentPending)
	}
}

func TestApprovalHandler_AutoApproved(t *testing.T) {
	handler := &ApprovalHandler{}
	payment := &models.Payment{
		RequiresApproval: false,
	}

	err := handler.Handle(payment)
	if err != nil {
		t.Errorf("ApprovalHandler auto approve: expected nil, got %v", err)
	}
	if payment.Status != models.PaymentApproved {
		t.Errorf("Status: got %q, want %q", payment.Status, models.PaymentApproved)
	}
}

func TestApprovalHandler_AutoApprovedPassesToNext(t *testing.T) {
	handler := &ApprovalHandler{}
	nextHandler := &mockHandler{called: false}
	handler.SetNext(nextHandler)

	payment := &models.Payment{
		RequiresApproval: false,
	}

	handler.Handle(payment)
	if !nextHandler.called {
		t.Error("Auto-approved payment should pass to next handler")
	}
}

func TestApprovalHandler_PendingDoesNotPassToNext(t *testing.T) {
	handler := &ApprovalHandler{}
	nextHandler := &mockHandler{called: false}
	handler.SetNext(nextHandler)

	payment := &models.Payment{
		RequiresApproval: true,
	}

	handler.Handle(payment)
	if nextHandler.called {
		t.Error("Pending payment should NOT pass to next handler")
	}
}

// === Testes do NotificationHandler ===

func TestNotificationHandler_PassesToNext(t *testing.T) {
	handler := &NotificationHandler{}
	nextHandler := &mockHandler{called: false}
	handler.SetNext(nextHandler)

	payment := &models.Payment{OrderID: "order_001"}
	handler.Handle(payment)

	if !nextHandler.called {
		t.Error("NotificationHandler should pass to next handler")
	}
}

func TestNotificationHandler_NoNextHandler(t *testing.T) {
	handler := &NotificationHandler{}
	payment := &models.Payment{OrderID: "order_001"}

	err := handler.Handle(payment)
	if err != nil {
		t.Errorf("NotificationHandler without next: expected nil, got %v", err)
	}
}

// === Testes de SetNext (encadeamento) ===

func TestBaseHandler_SetNext(t *testing.T) {
	h1 := &ValidationHandler{}
	h2 := &ApprovalHandler{}

	result := h1.SetNext(h2)
	if result != h2 {
		t.Error("SetNext should return the handler it was given")
	}
}

func TestBaseHandler_SetNextFluid(t *testing.T) {
	h1 := &ValidationHandler{}
	h2 := &ApprovalHandler{}
	h3 := &NotificationHandler{}

	h1.SetNext(h2).SetNext(h3)

	nextHandler := &mockHandler{called: false}
	h3.SetNext(nextHandler)

	payment := &models.Payment{
		OrderID:    "order_001",
		Amount:     100.0,
		CustomerID: "cust_001",
	}

	h1.Handle(payment)
	if !nextHandler.called {
		t.Error("Full chain should reach the end")
	}
}

// === Testes de BuildChain ===

func TestBuildChain_ReturnsHandler(t *testing.T) {
	chain := BuildChain()
	if chain == nil {
		t.Fatal("BuildChain should return a non-nil handler")
	}
}

// BuildChain cannot be tested without MongoDB because RiskCheckHandler
// calls checkFrequency/checkEstablishmentHistory which access the DB.
// Instead, test the chain logic manually without RiskCheckHandler.

func TestChain_ValidationToApproval(t *testing.T) {
	validation := &ValidationHandler{}
	approval := &ApprovalHandler{}
	notification := &NotificationHandler{}

	validation.SetNext(approval).SetNext(notification)

	payment := &models.Payment{
		OrderID:          "order_001",
		Amount:           50.0,
		CustomerID:       "cust_001",
		RequiresApproval: false,
	}

	err := validation.Handle(payment)
	if err != nil {
		t.Errorf("Chain valid payment: expected nil, got %v", err)
	}
	if payment.Status != models.PaymentApproved {
		t.Errorf("Status: got %q, want %q", payment.Status, models.PaymentApproved)
	}
}

func TestChain_ValidationRejectsInvalid(t *testing.T) {
	validation := &ValidationHandler{}
	approval := &ApprovalHandler{}

	validation.SetNext(approval)

	payment := &models.Payment{
		OrderID: "",
		Amount:  100.0,
	}

	err := validation.Handle(payment)
	if err != nil {
		t.Errorf("Chain invalid: expected nil, got %v", err)
	}
	if payment.Status != "" {
		t.Errorf("Invalid payment should not have status: got %q", payment.Status)
	}
}

func TestChain_PendingPayment(t *testing.T) {
	validation := &ValidationHandler{}
	approval := &ApprovalHandler{}
	notification := &NotificationHandler{}

	validation.SetNext(approval).SetNext(notification)

	payment := &models.Payment{
		OrderID:          "order_001",
		Amount:           50.0,
		CustomerID:       "cust_001",
		RequiresApproval: true,
	}

	err := validation.Handle(payment)
	if err != nil {
		t.Errorf("Chain pending: expected nil, got %v", err)
	}
	if payment.Status != models.PaymentPending {
		t.Errorf("Status: got %q, want %q", payment.Status, models.PaymentPending)
	}
}

// === Mock para testes ===

type mockHandler struct {
	called bool
}

func (m *mockHandler) Handle(payment *models.Payment) error {
	m.called = true
	return nil
}

func (m *mockHandler) SetNext(handler Handler) Handler {
	return handler
}
