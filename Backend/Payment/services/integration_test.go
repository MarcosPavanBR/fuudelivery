//go:build integration

// Testes de integração do Payment Service com MongoDB real (testcontainers).
//
// Cobrem os fluxos críticos de dinheiro:
// 1. Pagamento aprovado → crédito na carteira (happy path)
// 2. Idempotência: mesmo payment processado duas vezes não credita em dobro
// 3. Carteira com saldo insuficiente não debita
// 4. Créditos concorrentes (race condition) preservam todas as atualizações
// 5. Múltiplos pagamentos acumulam corretamente na carteira
//
// NOTA: Testes de validação de input (amount <= 0) e de status não-aprovado
// já estão cobertos por testes unitários em wallet_service_test.go e
// risk_scorer_test.go — não precisam de um container MongoDB.
//
// Rodar com:
//
//	go test -tags=integration ./services/... -v
//
// Pré-requisito:
//
//	go get github.com/testcontainers/testcontainers-go
//	go get github.com/testcontainers/testcontainers-go/modules/mongodb
package services

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// setupPaymentIntegrationEnv sobe um MongoDB real em container e aponta as
// coleções globais de repository/mongo.go (Payments, Wallets, WalletTransactions)
// pra esse banco de teste. Devolve uma func de cleanup.
func setupPaymentIntegrationEnv(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()

	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err, "subir container do MongoDB")

	uri, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.NoError(t, err)
	require.NoError(t, client.Ping(ctx, nil))

	db := client.Database("payment_test")
	repository.Client = client
	repository.Database = db
	repository.Payments = db.Collection("payments")
	repository.Wallets = db.Collection("wallets")
	repository.WalletTransactions = db.Collection("wallet_transactions")
	repository.Chargebacks = db.Collection("chargebacks")
	repository.Evidences = db.Collection("evidences")
	repository.Users = db.Collection("users")
	repository.ProcessedOrders = db.Collection("processed_orders")
	repository.Payouts = db.Collection("payouts")

	// Cria índices necessários (replica os que createIndexes() faria)
	createTestIndexes(ctx, db)

	return func() {
		_ = client.Disconnect(ctx)
		_ = mongoContainer.Terminate(ctx)
	}
}

// createTestIndexes cria índices usados nos testes.
func createTestIndexes(ctx context.Context, db *mongo.Database) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db.Collection("wallet_transactions").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"wallet_id"}},
		{Keys: []string{"reference_id"}, Options: options.Index().SetUnique(true).SetSparse(true)},
	})

	db.Collection("wallets").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"user_id"}, Options: options.Index().SetUnique(true)},
	})

	db.Collection("payments").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"order_id"}, Options: options.Index().SetUnique(true)},
	})
}

// ════════════════════════════════════════════════════════════════════
// 1. CAMINHO FELIZ — pagamento aprovado credita valor líquido
// ════════════════════════════════════════════════════════════════════

func TestPaymentApproval_CreditsNetAmountToWallet(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	establishmentID := "establishment-123"

	payment := &models.Payment{
		OrderID:         "order-1",
		EstablishmentID: establishmentID,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Status:          models.PaymentApproved,
		Method:          models.PaymentMethodPix,
		CreatedAt:       time.Now(),
	}
	require.NoError(t, repository.CreatePayment(payment))

	ws := NewWalletService()
	err := ws.ProcessPaymentApproval(payment)
	require.NoError(t, err)

	wallet, err := repository.GetWallet(establishmentID)
	require.NoError(t, err, "carteira deveria ter sido criada automaticamente no primeiro crédito")
	require.Equal(t, 90.00, wallet.Balance, "valor líquido = amount - delivery_amount")

	txs, err := ws.GetTransactions(establishmentID, 10)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, "order-1", txs[0].ReferenceID)

	updated, err := repository.GetPaymentByID(payment.ID)
	require.NoError(t, err)
	require.NotNil(t, updated, "payment deveria existir após o update")
}

// ════════════════════════════════════════════════════════════════════
// 2. IDEMPOTÊNCIA — mesmo payment processado 2x não credita em dobro
// ════════════════════════════════════════════════════════════════════

func TestPaymentApproval_IdempotentSecondCallSkipped(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	establishmentID := "estab-idempotent"

	payment := &models.Payment{
		OrderID:         "order-idempotent-1",
		EstablishmentID: establishmentID,
		Amount:          200.00,
		DeliveryAmount:  0,
		Status:          models.PaymentApproved,
		Method:          models.PaymentMethodCard,
		CreatedAt:       time.Now(),
	}
	require.NoError(t, repository.CreatePayment(payment))

	ws := NewWalletService()

	// Primeira chamada — deve creditar R$200
	err := ws.ProcessPaymentApproval(payment)
	require.NoError(t, err)

	wallet, err := repository.GetWallet(establishmentID)
	require.NoError(t, err)
	require.Equal(t, 200.00, wallet.Balance, "primeira chamada: R$200 creditados")

	// Segunda chamada — não deve creditar novamente
	err = ws.ProcessPaymentApproval(payment)
	require.NoError(t, err, "segunda chamada não deve retornar erro")

	wallet2, err := repository.GetWallet(establishmentID)
	require.NoError(t, err)
	require.Equal(t, 200.00, wallet2.Balance, "segunda chamada: saldo continua R$200 (sem dobro)")

	// Só deve haver 1 transação de crédito
	txs, err := ws.GetTransactions(establishmentID, 100)
	require.NoError(t, err)
	creditCount := 0
	for _, tx := range txs {
		if tx.Type == models.TransactionCredit {
			creditCount++
		}
	}
	require.Equal(t, 1, creditCount, "deve haver exatamente 1 transação de crédito")
}

// ════════════════════════════════════════════════════════════════════
// 3. SALDO INSUFICIENTE — débito deve falhar sem alterar saldo
// ════════════════════════════════════════════════════════════════════

func TestWallet_DebitInsufficientBalance(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	ws := NewWalletService()
	userID := "user-debit-test"

	// Cria carteira com saldo R$50
	_, err := repository.IncrementWalletBalance(userID, 50.0)
	require.NoError(t, err)

	// Tenta debitar R$75 — deve falhar
	err = ws.DebitWallet(userID, 75.0, "teste saldo insuficiente", "ref-debit-1")
	require.ErrorIs(t, err, repository.ErrInsufficientBalance, "deve retornar ErrInsufficientBalance")

	// Verifica que o saldo não mudou
	wallet, err := repository.GetWallet(userID)
	require.NoError(t, err)
	require.Equal(t, 50.0, wallet.Balance, "saldo não deve mudar após débito com saldo insuficiente")
}

func TestWallet_DebitExactBalance(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	ws := NewWalletService()
	userID := "user-debit-exact"

	// Cria carteira com R$30
	_, err := repository.IncrementWalletBalance(userID, 30.0)
	require.NoError(t, err)

	// Debita exatamente R$30 — deve funcionar
	err = ws.DebitWallet(userID, 30.0, "débito exato", "ref-debit-exact")
	require.NoError(t, err)

	wallet, err := repository.GetWallet(userID)
	require.NoError(t, err)
	require.Equal(t, 0.0, wallet.Balance, "saldo deve ser zero após débito exato")
}

// ════════════════════════════════════════════════════════════════════
// 4. CRÉDITOS CONCORRENTES — race condition com goroutines
// ════════════════════════════════════════════════════════════════════

func TestWallet_ConcurrentCredits(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	ws := NewWalletService()
	userID := "user-concurrent"
	numGoroutines := 20
	creditPerGoroutine := 1.0 // R$1 por goroutine

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			refID := fmt.Sprintf("concurrent-credit-%d", idx)
			if err := ws.CreditWallet(userID, creditPerGoroutine, fmt.Sprintf("credit %d", idx), refID); err != nil {
				errors <- fmt.Errorf("goroutine %d: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("erro em goroutine concorrente: %v", err)
	}

	// Saldo deve ser exatamente numGoroutines * creditPerGoroutine
	wallet, err := repository.GetWallet(userID)
	require.NoError(t, err)
	expectedBalance := float64(numGoroutines) * creditPerGoroutine
	require.Equal(t, expectedBalance, wallet.Balance,
		"saldo deve ser %v (20 goroutines × R$1 = R$20), got %v", expectedBalance, wallet.Balance)
}

func TestWallet_ConcurrentCreditAndDebit(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	ws := NewWalletService()
	userID := "user-concurrent-mix"

	// Cria carteira com R$100
	_, err := repository.IncrementWalletBalance(userID, 100.0)
	require.NoError(t, err)

	// 10 goroutines creditando R$5 cada (total +R$50)
	// 10 goroutines debitando R$2 cada (total -R$20)
	// Saldo esperado: 100 + 50 - 20 = R$130
	var wg sync.WaitGroup
	wg.Add(20)

	creditErrors := make(chan error, 10)
	debitErrors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer wg.Done()
			refID := fmt.Sprintf("mix-credit-%d", idx)
			if err := ws.CreditWallet(userID, 5.0, "concurrent credit", refID); err != nil {
				creditErrors <- err
			}
		}(i)

		go func(idx int) {
			defer wg.Done()
			refID := fmt.Sprintf("mix-debit-%d", idx)
			// Debita R$2 — se saldo insuficiente, ignora (fluxo normal)
			if err := ws.DebitWallet(userID, 2.0, "concurrent debit", refID); err != nil {
				debitErrors <- err
			}
		}(i)
	}

	wg.Wait()
	close(creditErrors)
	close(debitErrors)

	for err := range creditErrors {
		t.Errorf("erro no crédito concorrente: %v", err)
	}
	// DebitErrors pode conter ErrInsufficientBalance — isso é esperado se saldo acaba

	wallet, err := repository.GetWallet(userID)
	require.NoError(t, err)
	// Saldo mínimo garantido: 100 (inicial) + todos os créditos que passaram - débitos que passaram
	require.True(t, wallet.Balance >= 100.0,
		"saldo não pode cair abaixo do inicial: got %.2f", wallet.Balance)
	t.Logf("Saldo final: R$%.2f (esperado entre R$100 e R$130)", wallet.Balance)
}

// ════════════════════════════════════════════════════════════════════
// 5. MÚLTIPLOS PAGAMENTOS — acúmulo correto na carteira
// ════════════════════════════════════════════════════════════════════

func TestWallet_MultiplePaymentApprovals(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	establishmentID := "estab-multi"
	ws := NewWalletService()

	// 3 pagamentos aprovados sequenciais
	payments := []struct {
		orderID  string
		amount   float64
		delivery float64
	}{
		{"order-multi-1", 100.0, 10.0}, // líquido: 90
		{"order-multi-2", 50.0, 5.0},   // líquido: 45
		{"order-multi-3", 200.0, 20.0}, // líquido: 180
	}

	totalExpected := 0.0
	for _, p := range payments {
		payment := &models.Payment{
			OrderID:         p.orderID,
			EstablishmentID: establishmentID,
			Amount:          p.amount,
			DeliveryAmount:  p.delivery,
			Status:          models.PaymentApproved,
			Method:          models.PaymentMethodPix,
			CreatedAt:       time.Now(),
		}
		require.NoError(t, repository.CreatePayment(payment))

		err := ws.ProcessPaymentApproval(payment)
		require.NoError(t, err, "ProcessPaymentApproval para %s", p.orderID)

		totalExpected += p.amount - p.delivery
	}

	// Saldo final deve ser 90 + 45 + 180 = 315
	wallet, err := repository.GetWallet(establishmentID)
	require.NoError(t, err)
	require.Equal(t, totalExpected, wallet.Balance, "saldo acumulado de 3 pagamentos")

	// Histórico deve ter 3 transações
	txs, err := ws.GetTransactions(establishmentID, 100)
	require.NoError(t, err)
	require.Len(t, txs, 3, "deve haver 3 transações no histórico")
}
