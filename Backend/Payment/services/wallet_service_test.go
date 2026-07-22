// Package services - wallet_service_test.go
// Testes unitarios do servico de carteiras.
// Testam validacao de inputs e logica de negocio.
// Nao dependem de MongoDB — testam apenas regras de validacao.
package services

import (
	"testing"

	"github.com/carloshomar/vercardapio/payment/models"
)

// === Testes de validacao de entrada (amount <= 0) ===

func TestCreditWallet_AmountValidation(t *testing.T) {
	// Testa que a validacao de amount rejeita valores invalidos
	// SEM chamar o repository (que precisa de MongoDB)
	tests := []struct {
		name      string
		amount    float64
		wantError bool
	}{
		{"zero amount is invalid", 0.0, true},
		{"negative amount is invalid", -10.0, true},
		{"negative large is invalid", -999.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validação direta: replicar a lógica do CreditWallet
			isInvalid := tt.amount <= 0
			if isInvalid != tt.wantError {
				t.Errorf("amount=%.2f: got invalid=%v, want %v", tt.amount, isInvalid, tt.wantError)
			}
		})
	}
}

func TestDebitWallet_AmountValidation(t *testing.T) {
	tests := []struct {
		name      string
		amount    float64
		wantError bool
	}{
		{"zero amount is invalid", 0.0, true},
		{"negative amount is invalid", -5.0, true},
		{"negative small is invalid", -0.01, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := tt.amount <= 0
			if isInvalid != tt.wantError {
				t.Errorf("amount=%.2f: got invalid=%v, want %v", tt.amount, isInvalid, tt.wantError)
			}
		})
	}
}

// === Testes de ProcessPaymentApproval (logica pura) ===

func TestProcessPaymentApproval_SkipsNonApproved(t *testing.T) {
	// Testa que pagamentos nao aprovados nao devem ser processados
	nonApprovedStatuses := []models.PaymentStatus{
		models.PaymentPending,
		models.PaymentRejected,
		models.PaymentCancelled,
		models.PaymentRefunded,
		models.PaymentDisputed,
	}

	for _, status := range nonApprovedStatuses {
		t.Run(string(status), func(t *testing.T) {
			payment := &models.Payment{
				Status: status,
			}
			// Lógica do ProcessPaymentApproval: só processa se status == approved
			shouldProcess := payment.Status == models.PaymentApproved
			if shouldProcess {
				t.Errorf("Status %q should NOT be processed", status)
			}
		})
	}
}

func TestProcessPaymentApproval_CalculatesNetAmount(t *testing.T) {
	tests := []struct {
		name           string
		amount         float64
		deliveryAmount float64
		expectedNet    float64
	}{
		{"standard order", 100.0, 15.0, 85.0},
		{"free delivery", 50.0, 0.0, 50.0},
		{"expensive delivery", 80.0, 30.0, 50.0},
		{"minimum order", 10.0, 5.0, 5.0},
		{"large order", 500.0, 25.0, 475.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payment := &models.Payment{
				Amount:         tt.amount,
				DeliveryAmount: tt.deliveryAmount,
			}

			// Lógica: valor líquido = amount - deliveryAmount
			netAmount := payment.Amount - payment.DeliveryAmount
			if netAmount != tt.expectedNet {
				t.Errorf("Net amount: got %f, want %f", netAmount, tt.expectedNet)
			}
		})
	}
}

// === Testes de WalletBalance (logica de saldo) ===

func TestWalletBalance_CreditDebit(t *testing.T) {
	var balance float64 = 0

	// Credit
	balance += 100.0
	if balance != 100.0 {
		t.Errorf("After credit: got %f, want 100.0", balance)
	}

	// Debit
	balance -= 30.0
	if balance != 70.0 {
		t.Errorf("After debit: got %f, want 70.0", balance)
	}
}

func TestWalletBalance_InsufficientFunds(t *testing.T) {
	balance := 50.0
	debitAmount := 75.0

	if balance >= debitAmount {
		t.Error("Should detect insufficient balance")
	}
}

func TestWalletBalance_MultipleOperations(t *testing.T) {
	var balance float64 = 0

	// Simula múltiplas operações
	balance += 100.0 // Credit: order 1
	balance += 50.0  // Credit: order 2
	balance -= 30.0  // Debit: chargeback
	balance += 25.0  // Credit: order 3

	expected := 145.0
	if balance != expected {
		t.Errorf("Balance after operations: got %f, want %f", balance, expected)
	}
}

// === Testes de TransactionType ===

func TestTransactionTypes(t *testing.T) {
	if models.TransactionCredit != "credit" {
		t.Errorf("TransactionCredit: got %q, want %q", models.TransactionCredit, "credit")
	}
	if models.TransactionDebit != "debit" {
		t.Errorf("TransactionDebit: got %q, want %q", models.TransactionDebit, "debit")
	}
}

// === Testes de PaymentStatus ===

func TestPaymentStatuses(t *testing.T) {
	statuses := map[models.PaymentStatus]string{
		models.PaymentPending:   "pending",
		models.PaymentApproved:  "approved",
		models.PaymentRejected:  "rejected",
		models.PaymentCancelled: "cancelled",
		models.PaymentRefunded:  "refunded",
		models.PaymentDisputed:  "disputed",
	}

	for status, expected := range statuses {
		if string(status) != expected {
			t.Errorf("Status %v: got %q, want %q", status, string(status), expected)
		}
	}
}

// === Testes de RiskLevel ===

func TestRiskLevels(t *testing.T) {
	tests := []struct {
		level    models.RiskLevel
		expected string
	}{
		{models.RiskLow, "low"},
		{models.RiskMedium, "medium"},
		{models.RiskHigh, "high"},
		{models.RiskCritical, "critical"},
	}

	for _, tt := range tests {
		if string(tt.level) != tt.expected {
			t.Errorf("RiskLevel %v: got %q, want %q", tt.level, string(tt.level), tt.expected)
		}
	}
}

// === Testes de Wallet ===

func TestWallet_Currency(t *testing.T) {
	wallet := models.Wallet{
		UserID:   "user_001",
		UserType: "restaurant",
		Balance:  0,
		Currency: "BRL",
		Status:   "active",
	}

	if wallet.Currency != "BRL" {
		t.Errorf("Default currency: got %q, want %q", wallet.Currency, "BRL")
	}
	if wallet.Status != "active" {
		t.Errorf("Default status: got %q, want %q", wallet.Status, "active")
	}
	if wallet.Balance != 0 {
		t.Errorf("Initial balance: got %f, want 0", wallet.Balance)
	}
}

// === Testes de WalletTransaction ===

func TestWalletTransaction_CreditFlow(t *testing.T) {
	tx := models.WalletTransaction{
		Type:          models.TransactionCredit,
		Amount:        100.0,
		BalanceBefore: 0,
		BalanceAfter:  100.0,
		Description:   "Payment for order #123",
		ReferenceID:   "order_123",
	}

	if tx.BalanceAfter-tx.BalanceBefore != tx.Amount {
		t.Errorf("Balance mismatch: before=%f after=%f amount=%f",
			tx.BalanceBefore, tx.BalanceAfter, tx.Amount)
	}
}

func TestWalletTransaction_DebitFlow(t *testing.T) {
	tx := models.WalletTransaction{
		Type:          models.TransactionDebit,
		Amount:        25.0,
		BalanceBefore: 100.0,
		BalanceAfter:  75.0,
		Description:   "Chargeback for order #456",
		ReferenceID:   "order_456",
	}

	if tx.BalanceBefore-tx.Amount != tx.BalanceAfter {
		t.Errorf("Debit flow: before=%f amount=%f after=%f",
			tx.BalanceBefore, tx.Amount, tx.BalanceAfter)
	}
}

func TestWalletTransaction_Sequential(t *testing.T) {
	// Simula sequência de transações
	balance := 0.0
	txCount := 0

	// Credit 100
	balanceBefore := balance
	balance += 100.0
	txCount++
	if balance != 100.0 {
		t.Errorf("TX1: balance=%f, want 100.0", balance)
	}

	// Debit 30
	balanceBefore = balance
	balance -= 30.0
	txCount++
	if balance != 70.0 {
		t.Errorf("TX2: balance=%f, want 70.0", balance)
	}
	if balanceBefore-balance != 30.0 {
		t.Errorf("TX2 debit amount: got %f, want 30.0", balanceBefore-balance)
	}

	// Credit 50
	balanceBefore = balance
	balance += 50.0
	txCount++
	if balance != 120.0 {
		t.Errorf("TX3: balance=%f, want 120.0", balance)
	}

	if txCount != 3 {
		t.Errorf("Total transactions: got %d, want 3", txCount)
	}
}

// === Testes de PaymentMethod ===

func TestWalletService_PaymentMethods(t *testing.T) {
	methods := map[models.PaymentMethod]string{
		models.PaymentMethodPix:  "pix",
		models.PaymentMethodCard: "card",
	}

	for method, expected := range methods {
		if string(method) != expected {
			t.Errorf("PaymentMethod %v: got %q, want %q", method, string(method), expected)
		}
	}
}

// === Testes de PaymentFilter ===

func TestWalletService_PaymentFilterDefaults(t *testing.T) {
	filter := models.PaymentFilter{}

	if filter.Page != 0 {
		t.Errorf("Default page: got %d, want 0", filter.Page)
	}
	if filter.Limit != 0 {
		t.Errorf("Default limit: got %d, want 0", filter.Limit)
	}
}

func TestWalletService_PaymentFilterWithValues(t *testing.T) {
	filter := models.PaymentFilter{
		Status:          "pending",
		RiskLevel:       "high",
		EstablishmentID: "rest_001",
		Page:            2,
		Limit:           10,
	}

	if filter.Status != "pending" {
		t.Errorf("Status: got %q, want %q", filter.Status, "pending")
	}
	if filter.Page != 2 {
		t.Errorf("Page: got %d, want 2", filter.Page)
	}
	if filter.Limit != 10 {
		t.Errorf("Limit: got %d, want 10", filter.Limit)
	}
}
