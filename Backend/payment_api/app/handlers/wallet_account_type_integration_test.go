//go:build integration

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// A carteira é (user_id, user_type). A loja 42 e o cliente 42 são pessoas
// diferentes com o mesmo user_id — e o código adivinhava o tipo pela primeira
// carteira achada (walletTypeForUser) e checava idempotência só pelo user_id.

func countLedgerOf(t *testing.T, userID int64, userType, txnType, refID string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, models.DB.Model(&models.WalletTxn{}).
		Joins("JOIN wallets ON wallets.id = wallet_transactions.wallet_id").
		Where("wallets.user_id = ? AND wallets.user_type = ? AND wallet_transactions.type = ? AND wallet_transactions.reference_id = ?",
			userID, userType, txnType, refID).
		Count(&n).Error)
	return n
}

// Estorno de um pagamento da loja 42 feito pelo cliente 42 como top-up: a
// fatia da loja sai da carteira da LOJA e o top-up da carteira do CLIENTE,
// uma vez cada — mesmo com o webhook de estorno chegando duas vezes.
//
// Antes: a fatia da loja era debitada da carteira do cliente (a primeira
// achada), e o débito do top-up era pulado porque "já existia débito para o
// user_id 42 nesta referência".
func TestEstorno_LojaEClienteComMesmoID(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	applyLedgerIdempotencyIndexes(t)

	// A carteira do cliente nasce primeiro: é a que walletTypeForUser achava.
	seedWallet(t, 42, "customer", 100) // o top-up já creditado
	seedWallet(t, 42, "establishment", 0)

	creditado := time.Now().Add(-time.Minute)
	p := pagamentoNaoLiquidado("order-mesmo-id", "charge-mesmo-id", time.Minute)
	p.CustomerID = 42 // mesmo número da loja
	p.WalletCreditedAt = &creditado
	seedPayment(t, &p)
	require.NoError(t, settlePaymentApproved(&p))
	share := currentBalance(t, 42, "establishment")
	require.Greater(t, share, 0.0, "pré-condição: o settle creditou a fatia da loja")

	processPaymentRefund(p.AbacatePayID)

	require.InDelta(t, 0.0, currentBalance(t, 42, "establishment"), 0.001, "a fatia sai da carteira da loja")
	require.InDelta(t, 0.0, currentBalance(t, 42, "customer"), 0.001, "o top-up sai da carteira do cliente")
	require.Equal(t, int64(1), countLedgerOf(t, 42, "establishment", "debit", "charge-mesmo-id"))
	require.Equal(t, int64(1), countLedgerOf(t, 42, "customer", "debit", "charge-mesmo-id"))

	// Segundo webhook que leu o pagamento antes de ele virar REFUNDED: a
	// idempotência do ledger (por carteira) segura — nada é debitado de novo.
	require.NoError(t, models.DB.Model(&models.Payment{}).
		Where("abacatepay_id = ?", p.AbacatePayID).Update("status", "CONFIRMED").Error)
	seedWalletTopUp := func(userType string, amount float64) {
		require.NoError(t, models.DB.Model(&models.Wallet{}).
			Where("user_id = ? AND user_type = ?", 42, userType).
			Update("balance", amount).Error)
	}
	seedWalletTopUp("establishment", 500) // saldo de outros pedidos
	seedWalletTopUp("customer", 500)
	processPaymentRefund(p.AbacatePayID)
	require.InDelta(t, 500.0, currentBalance(t, 42, "establishment"), 0.001, "replay não debita a loja de novo")
	require.InDelta(t, 500.0, currentBalance(t, 42, "customer"), 0.001, "replay não debita o cliente de novo")
	require.Equal(t, int64(1), countLedgerOf(t, 42, "establishment", "debit", "charge-mesmo-id"))
	require.Equal(t, int64(1), countLedgerOf(t, 42, "customer", "debit", "charge-mesmo-id"))
}

// O cliente 5 paga o pedido com a carteira DELE, mesmo com a loja 5 tendo
// carteira (criada antes) — e o replay do mesmo pedido não debita de novo.
func TestDeduct_ClienteNaoPagaComCarteiraDaLoja(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	applyLedgerIdempotencyIndexes(t)
	createOrderDocumentsTable(t)

	os.Setenv("JWT_SECRET", "deduct-mesmo-id-secret")
	defer os.Unsetenv("JWT_SECRET")

	const phone = "+5511900000005"
	seedWallet(t, 5, "establishment", 100)
	seedWallet(t, 5, "customer", 30)
	require.NoError(t, models.DB.Exec(
		`INSERT INTO order_documents (legacy_id, establishment_id, user_phone, payload)
		 VALUES ('order-cliente-5', 9, ?, jsonb_build_object('order_total', 25.0::float8))`, phone).Error)

	app := fiber.New()
	app.Post("/wallets/deduct", DeductFromWallet)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": float64(5), "phone": phone, "role": "client", "account_type": "client",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("deduct-mesmo-id-secret"))
	require.NoError(t, err)
	post := func() (int, map[string]interface{}) {
		req := httptest.NewRequest(http.MethodPost, "/wallets/deduct",
			bytesReader([]byte(`{"user_id":5,"amount":25.0,"order_id":"order-cliente-5"}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, rErr := app.Test(req, -1)
		require.NoError(t, rErr)
		var out map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	code, out := post()
	require.Equal(t, 200, code, "%v", out)
	require.InDelta(t, 5.0, currentBalance(t, 5, "customer"), 0.001, "o pedido sai da carteira do cliente")
	require.InDelta(t, 100.0, currentBalance(t, 5, "establishment"), 0.001, "a carteira da loja 5 não é tocada")

	code, out = post()
	require.Equal(t, 200, code, "replay idempotente: %v", out)
	require.Equal(t, true, out["idempotent"])
	require.InDelta(t, 5.0, currentBalance(t, 5, "customer"), 0.001)
	require.InDelta(t, 100.0, currentBalance(t, 5, "establishment"), 0.001)
}

// Sacar o saldo inteiro e repetir com a mesma Idempotency-Key: o replay
// batia na guarda de saldo (0 < valor) antes do índice único e voltava 400
// "Saldo insuficiente" — o dono via falha num saque que aconteceu.
func TestWithdraw_ReplayDoSaldoInteiro(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	applyLedgerIdempotencyIndexes(t)

	os.Setenv("JWT_SECRET", "withdraw-inteiro-secret")
	defer os.Unsetenv("JWT_SECRET")
	const estID int64 = 4242

	app := fiber.New()
	app.Post("/wallet/establishment/withdraw", EstablishmentWithdraw)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": 999, "establishment_id": float64(estID), "role": "restaurant", "account_type": "user",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("withdraw-inteiro-secret"))
	require.NoError(t, err)
	post := func() (int, map[string]interface{}) {
		req := httptest.NewRequest(http.MethodPost, "/wallet/establishment/withdraw",
			bytesReader([]byte(`{"amount":80.0,"destination":"pix@example.com","method":"PIX"}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Idempotency-Key", "saque-inteiro")
		resp, rErr := app.Test(req, -1)
		require.NoError(t, rErr)
		var out map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}

	seedWallet(t, estID, "establishment", 80.0)
	code, out := post()
	require.Equal(t, 200, code, "%v", out)
	code, out = post()
	require.Equal(t, 200, code, "replay do saque do saldo inteiro: %v", out)
	require.Equal(t, true, out["idempotent"])
	require.Equal(t, int64(1), countDebits(t, estID))
	require.InDelta(t, 0.0, currentBalance(t, estID, "establishment"), 0.001)
}

// O cliente paga um pedido por PIX e depois pede para usar o mesmo pagamento
// como recarga da carteira. Antes: 200 e o valor inteiro virava saldo — o
// pedido estava pago, a loja creditada, e o saldo pagava o próximo pedido.
func TestTopUp_PagamentoDePedidoNaoViraSaldo(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	applyLedgerIdempotencyIndexes(t)

	os.Setenv("JWT_SECRET", "topup-pedido-secret")
	defer os.Unsetenv("JWT_SECRET")

	seedWallet(t, 42, "establishment", 0)
	seedWallet(t, 100, "customer", 0)
	p := pagamentoNaoLiquidado("order-topup-duplo", "charge-topup-duplo", time.Minute)
	seedPayment(t, &p)
	require.NoError(t, settlePaymentApproved(&p))

	app := fiber.New()
	app.Post("/wallets/topup", TopUp)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": float64(100), "phone": p.CustomerPhone, "role": "client", "account_type": "client",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("topup-pedido-secret"))
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/wallets/topup",
		bytesReader([]byte(`{"user_id":100,"amount":100,"payment_id":"charge-topup-duplo"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	require.Equal(t, 409, resp.StatusCode)
	require.InDelta(t, 0.0, currentBalance(t, 100, "customer"), 0.001, "o pagamento do pedido não vira saldo")
	// Na custódia o índice único de crédito já barrava o crédito (a loja tem
	// crédito com a mesma referência), mas o claim ficava marcado — e o
	// estorno depois debitava do cliente um top-up que nunca entrou.
	require.Nil(t, findPaymentByAbacate(t, "charge-topup-duplo").WalletCreditedAt)

	// Split na origem: a loja recebeu direto no gateway, não há crédito
	// interno com a referência — aqui o valor virava saldo de verdade.
	p2 := pagamentoNaoLiquidado("order-topup-origem", "charge-topup-origem", time.Minute)
	p2.SplitAtOrigin = true
	seedPayment(t, &p2)
	require.NoError(t, settlePaymentApproved(&p2))
	req = httptest.NewRequest(http.MethodPost, "/wallets/topup",
		bytesReader([]byte(`{"user_id":100,"amount":100,"payment_id":"charge-topup-origem"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 409, resp.StatusCode)
	require.InDelta(t, 0.0, currentBalance(t, 100, "customer"), 0.001, "split na origem: o pedido pago não vira saldo")
}
