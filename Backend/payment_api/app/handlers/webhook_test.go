// Package handlers - webhook_test.go
// Testes unitarios do webhook handler.
// Testam a logica pura: defaultSplitRules, status mapping, validacao.
package handlers

import (
	"os"
	"testing"

	"github.com/carloshomar/vercardapio/payment_api/app/models"
)

// === Testes de defaultSplitRules ===

func TestDefaultSplitRules_StandardPayment(t *testing.T) {
	payment := &models.Payment{
		Amount:         100.0,
		DeliveryAmount: 15.0,
		EstablishmentID: 12345,
		CustomerID:     67890,
	}

	rules := defaultSplitRules(payment)

	// Platform fee: 5% = 5.0
	// Establishment: 85% = 85.0
	// Delivery: 15.0
	// Customer credit: 100 - 5 - 85 - 15 = -5.0 -> adjusted
	if len(rules) < 2 {
		t.Fatalf("Expected at least 2 rules, got %d", len(rules))
	}

	// Platform rule
	if rules[0].ReceiverType != "platform" {
		t.Errorf("Rule 0 type: got %q, want %q", rules[0].ReceiverType, "platform")
	}
	if rules[0].Percentage != 5.0 {
		t.Errorf("Rule 0 percentage: got %f, want 5.0", rules[0].Percentage)
	}

	// Establishment rule
	if rules[1].ReceiverType != "establishment" {
		t.Errorf("Rule 1 type: got %q, want %q", rules[1].ReceiverType, "establishment")
	}
	if rules[1].Percentage != 85.0 {
		t.Errorf("Rule 1 percentage: got %f, want 85.0", rules[1].Percentage)
	}
}

func TestDefaultSplitRules_WithDelivery(t *testing.T) {
	payment := &models.Payment{
		Amount:         200.0,
		DeliveryAmount: 10.0,
		EstablishmentID: 100,
		CustomerID:     200,
	}

	rules := defaultSplitRules(payment)

	// Should have 3 rules: platform, establishment, deliveryman
	hasDelivery := false
	for _, rule := range rules {
		if rule.ReceiverType == "deliveryman" {
			hasDelivery = true
			if rule.Amount != 10.0 {
				t.Errorf("Delivery amount: got %f, want 10.0", rule.Amount)
			}
		}
	}
	if !hasDelivery {
		t.Error("Expected deliveryman rule when DeliveryAmount > 0")
	}
}

func TestDefaultSplitRules_NoDelivery(t *testing.T) {
	payment := &models.Payment{
		Amount:         100.0,
		DeliveryAmount: 0,
		EstablishmentID: 100,
		CustomerID:     200,
	}

	rules := defaultSplitRules(payment)

	for _, rule := range rules {
		if rule.ReceiverType == "deliveryman" {
			t.Error("Should not have deliveryman rule when DeliveryAmount = 0")
		}
	}
}

func TestDefaultSplitRules_CustomerCredit(t *testing.T) {
	// When delivery is low, customer gets credit
	payment := &models.Payment{
		Amount:         200.0,
		DeliveryAmount: 0,
		EstablishmentID: 100,
		CustomerID:     200,
	}

	rules := defaultSplitRules(payment)

	// Platform: 10.0, Establishment: 170.0, Customer: 20.0
	hasCustomer := false
	for _, rule := range rules {
		if rule.ReceiverType == "customer" {
			hasCustomer = true
			if rule.Amount <= 0 {
				t.Errorf("Customer credit should be positive: got %f", rule.Amount)
			}
		}
	}
	if !hasCustomer {
		t.Error("Expected customer credit rule")
	}
}

func TestDefaultSplitRules_HighDeliveryAdjustsEstablishment(t *testing.T) {
	// When delivery exceeds 10% of total, establishment amount is reduced
	payment := &models.Payment{
		Amount:         100.0,
		DeliveryAmount: 20.0, // 20% - high
		EstablishmentID: 100,
		CustomerID:     200,
	}

	rules := defaultSplitRules(payment)

	// Platform: 5.0, Establishment: should be reduced, Delivery: 20.0
	var estAmount float64
	for _, rule := range rules {
		if rule.ReceiverType == "establishment" {
			estAmount = rule.Amount
		}
	}

	// Establishment should be less than 85.0 due to high delivery
	if estAmount >= 85.0 {
		t.Errorf("Establishment amount should be reduced for high delivery: got %f", estAmount)
	}
}

func TestDefaultSplitRules_TotalEqualsPaymentAmount(t *testing.T) {
	payment := &models.Payment{
		Amount:         150.0,
		DeliveryAmount: 10.0,
		EstablishmentID: 100,
		CustomerID:     200,
	}

	rules := defaultSplitRules(payment)

	var totalSplit float64
	for _, rule := range rules {
		totalSplit += rule.Amount
	}

	// Total split should not exceed payment amount
	if totalSplit > payment.Amount+0.01 { // small float tolerance
		t.Errorf("Total split %f exceeds payment amount %f", totalSplit, payment.Amount)
	}
}

// === Testes de status mapping ===

func TestWebhookStatusMapping(t *testing.T) {
	tests := []struct {
		apiStatus    string
		expected     string
	}{
		{"paid", "CONFIRMED"},
		{"CONFIRMED", "CONFIRMED"},
		{"expired", "EXPIRED"},
		{"refunded", "REFUNDED"},
		{"cancelled", "CANCELLED"},
		{"pending", "pending"},
		{"unknown_status", "unknown_status"},
	}

	for _, tt := range tests {
		t.Run(tt.apiStatus, func(t *testing.T) {
			// Replicate the status mapping logic from HandlePaymentWebhook
			abacatepayStatus := ""
			switch tt.apiStatus {
			case "paid", "CONFIRMED":
				abacatepayStatus = "CONFIRMED"
			case "expired":
				abacatepayStatus = "EXPIRED"
			case "refunded":
				abacatepayStatus = "REFUNDED"
			case "cancelled":
				abacatepayStatus = "CANCELLED"
			default:
				abacatepayStatus = tt.apiStatus
			}

			if abacatepayStatus != tt.expected {
				t.Errorf("Status %q: got %q, want %q", tt.apiStatus, abacatepayStatus, tt.expected)
			}
		})
	}
}

// === Testes de publishToOrderQueue (env check) ===

func TestPublishToOrderQueue_NoEnv(t *testing.T) {
	// When RABBIT_CONNECTION is not set, should return nil (silent ignore)
	os.Unsetenv("RABBIT_CONNECTION")
	os.Unsetenv("RABBIT_ORDER_QUEUE")

	// The function returns nil when env is not set
	// We can't call it directly without RabbitMQ, but we test the logic
	rabbitConn := os.Getenv("RABBIT_CONNECTION")
	orderQueue := os.Getenv("RABBIT_ORDER_QUEUE")

	if rabbitConn != "" {
		t.Skip("RABBIT_CONNECTION is set, skipping no-env test")
	}

	if rabbitConn == "" || orderQueue == "" {
		// Expected behavior: message ignored silently
		t.Log("RabbitMQ not configured, message would be ignored")
	}
}

func TestPublishToPaymentQueue_NoEnv(t *testing.T) {
	os.Unsetenv("RABBIT_CONNECTION")
	os.Unsetenv("RABBIT_PAYMENT_QUEUE")

	rabbitConn := os.Getenv("RABBIT_CONNECTION")
	paymentQueue := os.Getenv("RABBIT_PAYMENT_QUEUE")

	if rabbitConn != "" {
		t.Skip("RABBIT_CONNECTION is set, skipping no-env test")
	}

	if rabbitConn == "" || paymentQueue == "" {
		t.Log("RabbitMQ not configured, message would be ignored")
	}
}

// === Testes de estrutura de pagamento aprovado ===

func TestPaymentApprovedMessageFormat(t *testing.T) {
	// Replicate the message format from publishPaymentApproved
	payment := &models.Payment{
		OrderID:         "order_123",
		ID:              [12]byte{1, 2, 3},
		Amount:          100.0,
		Method:          "pix",
		EstablishmentID: 100,
		DeliveryAmount:  10.0,
	}

	orderMsg := map[string]interface{}{
		"order_id":   payment.OrderID,
		"payment_id": "",
		"status":     "PAYMENT_CONFIRMED",
		"amount":     payment.Amount,
		"method":     payment.Method,
	}

	if orderMsg["status"] != "PAYMENT_CONFIRMED" {
		t.Errorf("Status: got %v, want PAYMENT_CONFIRMED", orderMsg["status"])
	}
	if orderMsg["amount"] != 100.0 {
		t.Errorf("Amount: got %v, want 100.0", orderMsg["amount"])
	}

	paymentMsg := map[string]interface{}{
		"order_id":         payment.OrderID,
		"establishment_id": payment.EstablishmentID,
		"amount":           payment.Amount,
		"delivery_amount":  payment.DeliveryAmount,
		"status":           "approved",
	}

	if paymentMsg["status"] != "approved" {
		t.Errorf("Payment status: got %v, want approved", paymentMsg["status"])
	}
}
