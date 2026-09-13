//go:build integration

package handlers

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// TestDebt_CriarSomarQuitar exercita o razão de contas a receber ponta a ponta:
// criar dívida, somar o que a loja deve em aberto, quitar, e conferir que a
// soma zera.
func TestDebt_CriarSomarQuitar(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	// Duas dívidas em aberto para a loja 42: frete 10 + comissão 5, e 8 + 4.
	created, err := models.CreateDebt(models.DB, &models.EstablishmentDebt{
		EstablishmentID: 42, OrderID: "ord-d1", DeliveryAmount: 10, CommissionAmount: 5,
	})
	require.NoError(t, err)
	require.True(t, created)

	d2 := &models.EstablishmentDebt{EstablishmentID: 42, OrderID: "ord-d2", DeliveryAmount: 8, CommissionAmount: 4}
	created, err = models.CreateDebt(models.DB, d2)
	require.NoError(t, err)
	require.True(t, created)

	// total_amount é calculado (frete + comissão), não confiado do input.
	require.InDelta(t, 15.0, mustFindDebt(t, "ord-d1").TotalAmount, 0.001)
	require.InDelta(t, 12.0, mustFindDebt(t, "ord-d2").TotalAmount, 0.001)

	// Soma em aberto: (15 + 12) = 27,00 = 2700 centavos.
	open, err := models.OpenDebtTotalCents(models.DB, 42)
	require.NoError(t, err)
	require.Equal(t, int64(2700), open)

	// Quita a primeira; a soma cai para 1200.
	require.NoError(t, models.MarkDebtPaid(models.DB, mustFindDebt(t, "ord-d1").ID, "admin-teste"))
	open, err = models.OpenDebtTotalCents(models.DB, 42)
	require.NoError(t, err)
	require.Equal(t, int64(1200), open)
}

// TestDebt_Idempotente: criar a dívida do mesmo pedido duas vezes NÃO duplica —
// settle reprocessado não cria dívida em dobro.
func TestDebt_Idempotente(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	d := &models.EstablishmentDebt{EstablishmentID: 7, OrderID: "ord-idem", DeliveryAmount: 10, CommissionAmount: 5}
	created, err := models.CreateDebt(models.DB, d)
	require.NoError(t, err)
	require.True(t, created, "primeira criação insere")

	created, err = models.CreateDebt(models.DB, &models.EstablishmentDebt{
		EstablishmentID: 7, OrderID: "ord-idem", DeliveryAmount: 10, CommissionAmount: 5,
	})
	require.NoError(t, err)
	require.False(t, created, "segunda criação do mesmo pedido NÃO insere")

	// Uma dívida só → 15,00.
	open, err := models.OpenDebtTotalCents(models.DB, 7)
	require.NoError(t, err)
	require.Equal(t, int64(1500), open)
}

// TestDebt_TravaDeCredito: a soma em aberto alimenta a trava. Acima do limite,
// EstablishmentBlockedByDebt bloqueia; abaixo, libera.
func TestDebt_TravaDeCredito(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	t.Setenv("REPASSE_ENABLED", "true")
	t.Setenv("REPASSE_CREDIT_LIMIT_CENTS", "2000") // R$20,00

	_, err := models.CreateDebt(models.DB, &models.EstablishmentDebt{
		EstablishmentID: 99, OrderID: "ord-t1", DeliveryAmount: 10, CommissionAmount: 5, // 15,00
	})
	require.NoError(t, err)

	blocked, open, limit := EstablishmentBlockedByDebt(99)
	require.False(t, blocked, "15 < 20: não bloqueia")
	require.Equal(t, int64(1500), open)
	require.Equal(t, int64(2000), limit)

	// Mais uma dívida empurra para 27,00 > 20,00 → bloqueia.
	_, err = models.CreateDebt(models.DB, &models.EstablishmentDebt{
		EstablishmentID: 99, OrderID: "ord-t2", DeliveryAmount: 8, CommissionAmount: 4, // +12
	})
	require.NoError(t, err)

	blocked, open, _ = EstablishmentBlockedByDebt(99)
	require.True(t, blocked, "27 >= 20: bloqueia novos pedidos")
	require.Equal(t, int64(2700), open)
}

func mustFindDebt(t *testing.T, orderID string) models.EstablishmentDebt {
	t.Helper()
	var d models.EstablishmentDebt
	require.NoError(t, models.DB.Where("order_id = ?", orderID).First(&d).Error)
	return d
}

// TestDenyChargeIfIndebted trava a cobrança na ponta do pagamento: com o repasse
// ligado e a loja acima do limite, a cobrança é recusada com 409; abaixo do
// limite, passa. É uma das duas pontas da trava (a outra é a vitrine).
func TestDenyChargeIfIndebted(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()
	t.Setenv("REPASSE_ENABLED", "true")
	t.Setenv("REPASSE_CREDIT_LIMIT_CENTS", "2000") // R$20

	app := fiber.New()
	app.Get("/charge/:est", func(c *fiber.Ctx) error {
		est, _ := strconv.ParseInt(c.Params("est"), 10, 64)
		if denyChargeIfIndebted(c, est) {
			return nil
		}
		return c.SendString("ok")
	})

	// Loja 50 sem dívida: passa.
	resp, err := app.Test(httptest.NewRequest("GET", "/charge/50", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "sem dívida, a cobrança passa")

	// Cria dívida de 25,00 (> 20,00) para a loja 50.
	_, err = models.CreateDebt(models.DB, &models.EstablishmentDebt{
		EstablishmentID: 50, OrderID: "ord-deny", DeliveryAmount: 20, CommissionAmount: 5,
	})
	require.NoError(t, err)

	resp, err = app.Test(httptest.NewRequest("GET", "/charge/50", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusConflict, resp.StatusCode, "acima do limite, a cobrança é recusada (409)")

	// Com o repasse DESLIGADO, a mesma dívida não bloqueia.
	t.Setenv("REPASSE_ENABLED", "false")
	resp, err = app.Test(httptest.NewRequest("GET", "/charge/50", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "repasse desligado: não bloqueia")
}
