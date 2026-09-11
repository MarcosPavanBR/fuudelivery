//go:build integration

package handlers

import (
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
// Âncora server-side do débito de carteira.
//
// O amount do corpo já foi a maior fonte de desvio possível no /deduct:
// debitar R$ 0,01 da carteira para liberar um pedido de R$ 100. Agora o
// valor pagável é o order_total gravado pelo orders_api (líquido do cupom)
// e o dono do pedido é o telefone do token — o corpo não decide nenhum dos
// dois.
//
// Por que integração: a âncora é uma query com casts do Postgres
// (payload->>'order_total'::float8) e a idempotência é índice parcial —
// sqlite recusa a sintaxe e fingiria a proteção.
//
// Como rodar:
//
//	go test -tags=integration -run 'TestWalletDeductAnchor' ./app/handlers/
// ============================================================================

func TestWalletDeductAnchor(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	applyLedgerIdempotencyIndexes(t)
	createOrderDocumentsTable(t)

	os.Setenv("JWT_SECRET", "deduct-anchor-secret")
	defer os.Unsetenv("JWT_SECRET")

	const (
		donoID  int64 = 1001
		outroID int64 = 2002
	)
	const phoneDono = "+5511977776666"
	const phoneOutro = "+5511955554444"

	// Pedido real, como o orders_api grava: total líquido do cupom e dono.
	require.NoError(t, models.DB.Exec(
		`INSERT INTO order_documents (legacy_id, establishment_id, user_phone, payload)
		 VALUES ('ord-deduct-anchor', 7, ?, jsonb_build_object('order_total', 80.0::float8))`,
		phoneDono).Error)

	// Carteira do dono com saldo de sobra.
	seedWallet(t, donoID, "customer", 500.0)

	app := fiber.New()
	app.Post("/wallet/deduct", DeductFromWallet)

	makeToken := func(userID int64, phone string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"id":    float64(userID),
			"phone": phone,
			"role":  "client",
			"exp":   time.Now().Add(time.Hour).Unix(),
		})
		s, err := tok.SignedString([]byte("deduct-anchor-secret"))
		require.NoError(t, err)
		return s
	}

	do := func(token string, body string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/wallet/deduct", bytesReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		return resp
	}

	t.Run("dono debita o valor exato do pedido", func(t *testing.T) {
		resp := do(makeToken(donoID, phoneDono),
			`{"user_id":1001,"amount":80.0,"order_id":"ord-deduct-anchor"}`)
		require.Equal(t, 200, resp.StatusCode)
		require.InDelta(t, 420.0, currentBalance(t, donoID, "customer"), 0.001)
		require.EqualValues(t, 1, countDebits(t, donoID))
	})

	t.Run("replay idempotente não debita de novo", func(t *testing.T) {
		resp := do(makeToken(donoID, phoneDono),
			`{"user_id":1001,"amount":80.0,"order_id":"ord-deduct-anchor"}`)
		require.Equal(t, 200, resp.StatusCode)
		require.InDelta(t, 420.0, currentBalance(t, donoID, "customer"), 0.001)
		require.EqualValues(t, 1, countDebits(t, donoID))
	})

	t.Run("outro usuário não debita pedido alheio", func(t *testing.T) {
		seedWallet(t, outroID, "customer", 300.0)
		resp := do(makeToken(outroID, phoneOutro),
			`{"user_id":2002,"amount":80.0,"order_id":"ord-deduct-anchor"}`)
		require.Equal(t, 403, resp.StatusCode)
		require.InDelta(t, 300.0, currentBalance(t, outroID, "customer"), 0.001)
		require.Zero(t, countDebits(t, outroID))
	})

	t.Run("valor divergente do pedido é recusado", func(t *testing.T) {
		// Pedido novo para não colidir com o débito do primeiro subteste.
		require.NoError(t, models.DB.Exec(
			`INSERT INTO order_documents (legacy_id, establishment_id, user_phone, payload)
			 VALUES ('ord-deduct-anchor-2', 7, ?, jsonb_build_object('order_total', 55.0::float8))`,
			phoneDono).Error)
		resp := do(makeToken(donoID, phoneDono),
			`{"user_id":1001,"amount":0.01,"order_id":"ord-deduct-anchor-2"}`)
		require.Equal(t, 409, resp.StatusCode)
		require.Zero(t, countDebits(t, donoID)-1, "nenhum débito novo além do primeiro pedido")
	})

	t.Run("pedido sem total server-side é recusado", func(t *testing.T) {
		resp := do(makeToken(donoID, phoneDono),
			`{"user_id":1001,"amount":10.0,"order_id":"ord-inexistente"}`)
		require.Equal(t, 400, resp.StatusCode)
	})

	t.Run("token sem telefone não debita", func(t *testing.T) {
		require.NoError(t, models.DB.Exec(
			`INSERT INTO order_documents (legacy_id, establishment_id, user_phone, payload)
			 VALUES ('ord-deduct-anchor-3', 7, ?, jsonb_build_object('order_total', 20.0::float8))`,
			phoneDono).Error)
		resp := do(makeToken(donoID, ""), // sem claim phone
			`{"user_id":1001,"amount":20.0,"order_id":"ord-deduct-anchor-3"}`)
		require.Equal(t, 403, resp.StatusCode)
	})
}
