//go:build integration

package handlers

import (
	"encoding/json"
	"fmt"
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

// ============================================================================
// Idempotência financeira — exigida por esquadrao/rules/common/testing.md:
// "toda operação financeira tem teste de idempotência (chamar duas vezes,
// mesmo efeito)".
//
// Por que integração e não unitário: a idempotência NÃO é implementada em Go,
// e sim por índices PARCIAIS do Postgres (uq_wallet_txns_credit_ref em
// sql/11, uq_wallet_txns_debit_ref_wallet em sql/18 + sql/20). Um teste com
// mock passaria sem provar nada — o que segura o dinheiro é o banco.
//
// Como rodar:
//
//	go test -tags=integration -run 'TestWalletIdempotency' ./app/handlers/
// ============================================================================

// applyLedgerIdempotencyIndexes recria os índices parciais de idempotência no
// banco de teste.
//
// IMPORTANTE: setupCheckoutE2EEnv monta o schema com gormDB.AutoMigrate, que
// cria as tabelas a partir das structs Go. Os índices parciais vêm de SQL cru
// (sql/11 e sql/18+20) e NÃO têm tag de índice no struct WalletTxn — logo o
// AutoMigrate não os cria. Sem esta função, um teste de idempotência rodaria
// contra um schema SEM a constraint que garante idempotência em produção:
// o INSERT duplicado passaria e o teste daria falsa sensação de segurança.
//
// As definições abaixo espelham exatamente os dois arquivos SQL.
func applyLedgerIdempotencyIndexes(t *testing.T) {
	t.Helper()
	require.NoError(t, models.DB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_txns_credit_ref
		    ON wallet_transactions (reference_id)
		    WHERE type = 'credit' AND reference_id <> ''`).Error)
	require.NoError(t, models.DB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_txns_debit_ref_wallet
		    ON wallet_transactions (wallet_id, reference_id)
		    WHERE type = 'debit' AND reference_id <> ''`).Error)
}

// countDebits conta os débitos de um usuário. Fina camada sobre o countLedger
// já existente em checkout_e2e_test.go (assinatura: userID, txnType, kind,
// refID) só para deixar as asserções abaixo legíveis.
func countDebits(t *testing.T, userID int64) int64 {
	t.Helper()
	return countLedger(t, userID, "debit", "", "")
}

// currentBalance relê o saldo direto do banco.
func currentBalance(t *testing.T, userID int64, userType string) float64 {
	t.Helper()
	var w models.Wallet
	require.NoError(t, models.DB.Where("user_id = ? AND user_type = ?", userID, userType).First(&w).Error)
	return w.Balance
}

func TestWalletIdempotency_Withdraw(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	applyLedgerIdempotencyIndexes(t)

	os.Setenv("JWT_SECRET", "idem-withdraw-secret")
	defer os.Unsetenv("JWT_SECRET")

	const estID int64 = 4242

	app := fiber.New()
	app.Post("/wallet/establishment/withdraw", EstablishmentWithdraw)

	token := func() string {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"id":               999,
			"establishment_id": float64(estID),
			"role":             "restaurant",
			"exp":              time.Now().Add(time.Hour).Unix(),
		})
		s, err := tok.SignedString([]byte("idem-withdraw-secret"))
		require.NoError(t, err)
		return s
	}()

	post := func(body, idemKey string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/wallet/establishment/withdraw", bytesReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
		}
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		return resp
	}

	seedWallet(t, estID, "establishment", 500.0)
	body := `{"amount":50.0,"destination":"pix@example.com","method":"PIX"}`

	t.Run("mesma Idempotency-Key debita uma vez só", func(t *testing.T) {
		resp1 := post(body, "key-aaa")
		require.Equal(t, 200, resp1.StatusCode)

		// Segundo envio (duplo clique / retry automático do cliente).
		resp2 := post(body, "key-aaa")
		require.Equal(t, 200, resp2.StatusCode, "replay deve ser 200 idempotente, nunca 500")

		var out map[string]interface{}
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&out))
		require.Equal(t, true, out["idempotent"], "resposta deve marcar que foi replay")

		require.Equal(t, int64(1), countDebits(t, estID),
			"duas chamadas com a mesma chave devem gerar UM lançamento")
		require.InDelta(t, 450.0, currentBalance(t, estID, "establishment"), 0.001,
			"saldo debitado uma única vez (500 - 50)")
	})

	// A Idempotency-Key é escolhida pelo cliente e não carrega valor nem
	// destino: reusá-la com outro valor recebia "Saque solicitado com
	// sucesso" sem saque nenhum acontecer.
	t.Run("mesma chave com valor diferente é recusada", func(t *testing.T) {
		saldoAntes := currentBalance(t, estID, "establishment")
		debitosAntes := countDebits(t, estID)

		outro := `{"amount":90.0,"destination":"pix@example.com","method":"PIX"}`
		resp := post(outro, "key-aaa")
		require.Equal(t, 409, resp.StatusCode,
			"chave reusada com outro valor não pode responder sucesso")

		require.Equal(t, debitosAntes, countDebits(t, estID))
		require.InDelta(t, saldoAntes, currentBalance(t, estID, "establishment"), 0.001)
	})

	t.Run("chaves diferentes sacam duas vezes", func(t *testing.T) {
		// Saque legítimo do mesmo valor não pode ser bloqueado.
		resp := post(body, "key-bbb")
		require.Equal(t, 200, resp.StatusCode)

		require.Equal(t, int64(2), countDebits(t, estID))
		require.InDelta(t, 400.0, currentBalance(t, estID, "establishment"), 0.001)
	})
}

// TestWalletIdempotency_WithdrawSemChaveUsaFallback cobre o cliente antigo,
// que não manda Idempotency-Key: o fallback derivado (janela curta) tem que
// barrar o duplo clique mesmo assim.
func TestWalletIdempotency_WithdrawSemChaveUsaFallback(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	applyLedgerIdempotencyIndexes(t)

	os.Setenv("JWT_SECRET", "idem-fallback-secret")
	defer os.Unsetenv("JWT_SECRET")

	const estID int64 = 5151

	app := fiber.New()
	app.Post("/wallet/establishment/withdraw", EstablishmentWithdraw)

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":               888,
		"establishment_id": float64(estID),
		"role":             "restaurant",
		"exp":              time.Now().Add(time.Hour).Unix(),
	})
	token, err := tok.SignedString([]byte("idem-fallback-secret"))
	require.NoError(t, err)

	post := func() *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/wallet/establishment/withdraw",
			bytesReader([]byte(`{"amount":30.0,"destination":"cliente-antigo@example.com","method":"PIX"}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		// Sem Idempotency-Key de propósito.
		resp, rErr := app.Test(req, -1)
		require.NoError(t, rErr)
		return resp
	}

	seedWallet(t, estID, "establishment", 200.0)

	require.Equal(t, 200, post().StatusCode)
	require.Equal(t, 200, post().StatusCode, "replay sem chave também deve ser 200")

	require.Equal(t, int64(1), countDebits(t, estID),
		"fallback derivado deve barrar o segundo saque idêntico na mesma janela")
	require.InDelta(t, 170.0, currentBalance(t, estID, "establishment"), 0.001)
}

func TestWalletIdempotency_Deduct(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	applyLedgerIdempotencyIndexes(t)

	os.Setenv("JWT_SECRET", "idem-deduct-secret")
	defer os.Unsetenv("JWT_SECRET")

	const userID int64 = 7777

	app := fiber.New()
	app.Post("/wallets/deduct", DeductFromWallet)

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":   float64(userID),
		"role": "client",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	token, err := tok.SignedString([]byte("idem-deduct-secret"))
	require.NoError(t, err)

	post := func(body string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/wallets/deduct", bytesReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, rErr := app.Test(req, -1)
		require.NoError(t, rErr)
		return resp
	}

	walletType := walletTypeForUser(userID)
	seedWallet(t, userID, walletType, 100.0)

	t.Run("mesmo order_id debita uma vez só", func(t *testing.T) {
		body := fmt.Sprintf(`{"user_id":%d,"amount":25.0,"order_id":"order-idem-1"}`, userID)

		require.Equal(t, 200, post(body).StatusCode)
		resp2 := post(body)
		require.Equal(t, 200, resp2.StatusCode, "replay deve ser 200 idempotente, nunca 500")

		var out map[string]interface{}
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&out))
		require.Equal(t, true, out["idempotent"])

		require.Equal(t, int64(1), countDebits(t, userID))
		require.InDelta(t, 75.0, currentBalance(t, userID, walletType), 0.001)
	})

	// O achado que mais dói: a chave de idempotência é o pedido e NÃO carrega
	// o valor. Sem comparar o valor registrado, debitar 0,01 e depois 100,00
	// no mesmo order_id devolvia 200 "debitado com sucesso" com
	// amount_deducted=100,00 — e nada saía da carteira. Quem consome a
	// resposta dá o pedido por pago.
	t.Run("mesmo order_id com valor diferente é recusado", func(t *testing.T) {
		saldoAntes := currentBalance(t, userID, walletType)
		debitosAntes := countDebits(t, userID)

		body := fmt.Sprintf(`{"user_id":%d,"amount":50.0,"order_id":"order-idem-1"}`, userID)
		resp := post(body)
		require.Equal(t, 409, resp.StatusCode,
			"replay com valor diferente não pode ser confirmado como sucesso")

		require.Equal(t, debitosAntes, countDebits(t, userID), "nada novo no ledger")
		require.InDelta(t, saldoAntes, currentBalance(t, userID, walletType), 0.001,
			"saldo intacto")
	})

	t.Run("order_id vazio é recusado", func(t *testing.T) {
		// Vazio cairia fora do índice parcial e debitaria sem proteção.
		body := fmt.Sprintf(`{"user_id":%d,"amount":10.0,"order_id":""}`, userID)
		require.Equal(t, 400, post(body).StatusCode)
		require.Equal(t, int64(1), countDebits(t, userID),
			"requisição recusada não pode gerar lançamento")
	})
}
