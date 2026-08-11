//go:build integration

// Testes de integracao de chargeback com MongoDB real (testcontainers).
// Cobrem: criacao, aprovacao com debito, rejeicao sem debito,
// idempotencia e listagem por status.
//
// Rodar com:
//
//	go test -tags=integration ./services/... -v -run TestChargebackIntegration
package services

import (
	"testing"

	"github.com/carloshomar/fuudelivery/payment/models"
	"github.com/carloshomar/fuudelivery/payment/repository"
	"github.com/stretchr/testify/require"
)

func TestChargebackIntegration_CreateAndRetrieve(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	svc := NewChargebackService()

	chargeback := &models.Chargeback{
		PaymentOrderID:  "order-cb-001",
		CustomerID:      "cust-100",
		EstablishmentID: "estab-42",
		Amount:          75.50,
		Reason:          models.ReasonUnauthorized,
		Description:     "Cliente nao autorizou a transacao",
		Status:          models.ChargebackPending,
	}

	err := svc.CreateChargeback(chargeback)
	require.NoError(t, err, "criar chargeback")
	require.False(t, chargeback.ID.IsZero(), "ID deve ter sido gerado")
	require.False(t, chargeback.CreatedAt.IsZero(), "CreatedAt deve ter sido setado")

	found, err := svc.GetChargeback(chargeback.ID.Hex())
	require.NoError(t, err, "buscar chargeback")
	require.Equal(t, "order-cb-001", found.PaymentOrderID)
	require.Equal(t, "cust-100", found.CustomerID)
	require.Equal(t, "estab-42", found.EstablishmentID)
	require.Equal(t, 75.50, found.Amount)
	require.Equal(t, models.ChargebackPending, found.Status)
}

func TestChargebackIntegration_ApproveAndDebitWallet(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	svc := NewChargebackService()
	ws := NewWalletService()
	establishmentID := "estab-cb-approve"

	_, err := repository.IncrementWalletBalance(establishmentID, 200.0)
	require.NoError(t, err)

	chargeback := &models.Chargeback{
		PaymentOrderID:  "order-cb-approve-001",
		CustomerID:      "cust-200",
		EstablishmentID: establishmentID,
		Amount:          50.0,
		Reason:          models.ReasonNotReceived,
		Description:     "Cliente nao recebeu o pedido",
		Status:          models.ChargebackPending,
	}
	err = svc.CreateChargeback(chargeback)
	require.NoError(t, err)

	err = svc.ApproveChargeback(chargeback.ID.Hex(), "admin-001")
	require.NoError(t, err, "aprovar chargeback")

	updated, err := svc.GetChargeback(chargeback.ID.Hex())
	require.NoError(t, err)
	require.Equal(t, models.ChargebackApproved, updated.Status)
	require.Equal(t, "admin-001", updated.ResolvedBy)
	require.NotNil(t, updated.ResolvedAt)

	err = ws.DebitWallet(establishmentID, chargeback.Amount, "chargeback approved: "+chargeback.PaymentOrderID, chargeback.ID.Hex())
	require.NoError(t, err, "debitar carteira apos chargeback")

	wallet, err := repository.GetWallet(establishmentID)
	require.NoError(t, err)
	require.Equal(t, 150.0, wallet.Balance, "saldo apos chargeback = 200 - 50 = 150")
}

func TestChargebackIntegration_RejectAndWalletUnchanged(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	svc := NewChargebackService()
	establishmentID := "estab-cb-reject"

	_, err := repository.IncrementWalletBalance(establishmentID, 100.0)
	require.NoError(t, err)

	chargeback := &models.Chargeback{
		PaymentOrderID:  "order-cb-reject-001",
		CustomerID:      "cust-300",
		EstablishmentID: establishmentID,
		Amount:          30.0,
		Reason:          models.ReasonDuplicate,
		Description:     "Transacao duplicada",
		Status:          models.ChargebackPending,
	}
	err = svc.CreateChargeback(chargeback)
	require.NoError(t, err)

	err = svc.RejectChargeback(chargeback.ID.Hex(), "admin-002", "Pagamento original validado")
	require.NoError(t, err, "rejeitar chargeback")

	updated, err := svc.GetChargeback(chargeback.ID.Hex())
	require.NoError(t, err)
	require.Equal(t, models.ChargebackRejected, updated.Status)
	require.Equal(t, "admin-002", updated.ResolvedBy)
	require.Contains(t, updated.Resolution, "validado")

	wallet, err := repository.GetWallet(establishmentID)
	require.NoError(t, err)
	require.Equal(t, 100.0, wallet.Balance, "saldo nao deve mudar apos rejeicao")
}

func TestChargebackIntegration_IdempotentDebit(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	svc := NewChargebackService()
	ws := NewWalletService()
	establishmentID := "estab-cb-idempotent"

	_, err := repository.IncrementWalletBalance(establishmentID, 300.0)
	require.NoError(t, err)

	chargeback := &models.Chargeback{
		PaymentOrderID:  "order-cb-idempotent-001",
		CustomerID:      "cust-400",
		EstablishmentID: establishmentID,
		Amount:          100.0,
		Reason:          models.ReasonOther,
		Description:     "Teste de idempotencia",
		Status:          models.ChargebackPending,
	}
	err = svc.CreateChargeback(chargeback)
	require.NoError(t, err)

	err = svc.ApproveChargeback(chargeback.ID.Hex(), "admin-003")
	require.NoError(t, err)

	err = ws.DebitWallet(establishmentID, 100.0, "chargeback: "+chargeback.PaymentOrderID, chargeback.ID.Hex())
	require.NoError(t, err)

	wallet, err := repository.GetWallet(establishmentID)
	require.NoError(t, err)
	require.Equal(t, 200.0, wallet.Balance, "apos 1o debito: 300 - 100 = 200")

	// Tentar debitar o MESMO reference_id
	_ = ws.DebitWallet(establishmentID, 100.0, "chargeback: "+chargeback.PaymentOrderID, chargeback.ID.Hex())

	wallet2, err := repository.GetWallet(establishmentID)
	require.NoError(t, err)
	require.True(t, wallet2.Balance >= 200.0,
		"saldo nao deve cair abaixo de 200 apos debito duplicado: got %.2f", wallet2.Balance)
}

func TestChargebackIntegration_ListByStatus(t *testing.T) {
	cleanup := setupPaymentIntegrationEnv(t)
	defer cleanup()

	svc := NewChargebackService()

	statuses := []models.ChargebackStatus{
		models.ChargebackPending,
		models.ChargebackApproved,
		models.ChargebackPending,
	}

	for i, status := range statuses {
		cb := &models.Chargeback{
			PaymentOrderID:  "order-list-" + string(rune('A'+i)),
			CustomerID:      "cust-list",
			EstablishmentID: "estab-list",
			Amount:          float64(10 * (i + 1)),
			Reason:          models.ReasonOther,
			Status:          status,
		}
		err := svc.CreateChargeback(cb)
		require.NoError(t, err)
	}

	all, total, err := svc.ListChargebacks("", 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, all, 3)

	pending, pendingTotal, err := svc.ListChargebacks("pending", 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), pendingTotal)
	require.Len(t, pending, 2)
	for _, cb := range pending {
		require.Equal(t, models.ChargebackPending, cb.Status)
	}

	approved, approvedTotal, err := svc.ListChargebacks("approved", 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), approvedTotal)
	require.Len(t, approved, 1)
	require.Equal(t, models.ChargebackApproved, approved[0].Status)
}
