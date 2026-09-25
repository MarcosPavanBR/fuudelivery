//go:build integration

package handlers

import (
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/stretchr/testify/require"
)

// Estorno (processPaymentRefund) só pode reverter o que o settle de fato
// creditou. Estes testes travam os três casos em que ele revertia dinheiro que
// nunca entrou: split na origem, cashback do cliente e o top-up escondido pelo
// guard de idempotência.

// liquidarEEstornar semeia o pagamento, liquida pelo caminho real (settle) e
// estorna — a mesma sequência de produção.
func liquidarEEstornar(t *testing.T, p *models.Payment) {
	t.Helper()
	seedPayment(t, p)
	require.NoError(t, settlePaymentApproved(p))
	processPaymentRefund(p.AbacatePayID)
	require.Equal(t, "REFUNDED", findPaymentByAbacate(t, p.AbacatePayID).Status)
}

// TestEstorno_Custodia_ReverteCreditoDaLoja é a linha de base: no fluxo de
// custódia a loja foi creditada no settle, então o estorno debita a mesma fatia.
func TestEstorno_Custodia_ReverteCreditoDaLoja(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	seedWallet(t, 42, "establishment", 0)
	p := pagamentoNaoLiquidado("order-refund-custodia", "charge-refund-custodia", time.Minute)
	liquidarEEstornar(t, &p)

	require.InDelta(t, 0.0, getWalletByUser(t, 42).Balance, 0.001,
		"custódia: o estorno devolve exatamente o que foi creditado")
	require.Equal(t, int64(1), countLedger(t, 42, "debit", "", "charge-refund-custodia"))
}

// TestEstorno_SplitNaOrigem_NaoDebitaCarteiraDaLoja: a loja recebeu direto na
// conta MP dela e a carteira interna NUNCA foi creditada. O estorno não pode
// tirar a "fatia" do saldo que ela tem de outros pedidos (custódia).
func TestEstorno_SplitNaOrigem_NaoDebitaCarteiraDaLoja(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	seedWallet(t, 42, "establishment", 200) // saldo de outros pedidos
	p := pagamentoNaoLiquidado("order-refund-split", "charge-refund-split", time.Minute)
	p.SplitAtOrigin = true
	liquidarEEstornar(t, &p)

	require.InDelta(t, 200.0, getWalletByUser(t, 42).Balance, 0.001,
		"split na origem: o estorno não pode debitar a carteira interna da loja")
	require.Equal(t, int64(0), countLedger(t, 42, "debit", "", "charge-refund-split"))
}

// TestEstorno_Repasse_PerdoaDivida: no repasse a loja recebeu 100% e ficou
// devendo frete + comissão. Estornado, o dinheiro sai da conta dela de volta
// para o cliente — ela não deve mais nada por este pedido, e a dívida em aberto
// não pode continuar travando as vendas dela.
func TestEstorno_Repasse_PerdoaDivida(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	seedWallet(t, 42, "establishment", 200)
	p := pagamentoNaoLiquidado("order-refund-repasse", "charge-refund-repasse", time.Minute)
	p.SplitAtOrigin = true
	p.Repasse = true
	seedPayment(t, &p)
	require.NoError(t, settlePaymentApproved(&p))

	open, err := models.OpenDebtTotalCents(models.DB, 42)
	require.NoError(t, err)
	require.Equal(t, int64(1500), open, "pré-condição: settle criou a dívida")

	processPaymentRefund(p.AbacatePayID)

	open, err = models.OpenDebtTotalCents(models.DB, 42)
	require.NoError(t, err)
	require.Equal(t, int64(0), open, "estorno do repasse perdoa a dívida do pedido")
	d := mustFindDebt(t, "order-refund-repasse")
	require.Equal(t, models.DebtWaived, d.Status)
	require.NotNil(t, d.SettledAt)
	require.InDelta(t, 200.0, getWalletByUser(t, 42).Balance, 0.001)
}

// TestEstorno_NaoDebitaCashbackNuncaCreditado: o settle não credita cashback
// na carteira do cliente (só a fatia da loja e o top-up movem carteira). O
// estorno debitava essa "fatia customer" do saldo real do cliente.
func TestEstorno_NaoDebitaCashbackNuncaCreditado(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	seedWallet(t, 42, "establishment", 0)
	comCashback(t)
	seedWallet(t, 100, "customer", 50) // saldo próprio do cliente
	p := pagamentoNaoLiquidado("order-refund-cashback", "charge-refund-cashback", time.Minute)
	liquidarEEstornar(t, &p)

	require.Greater(t, customerCashbackAmount(findPaymentByAbacate(t, "charge-refund-cashback").SplitRules), 0.0,
		"pré-condição: o split tem fatia customer")
	require.InDelta(t, 50.0, getWalletByUser(t, 100).Balance, 0.001,
		"o estorno não pode tirar do cliente um cashback que nunca foi creditado")
	require.Equal(t, int64(0), countLedger(t, 100, "debit", "", "charge-refund-cashback"))
}

// TestEstorno_ReverteTopUpInteiro: um pagamento usado como top-up creditou o
// valor cheio na carteira do cliente. O estorno tem que tirar o valor cheio —
// antes, o débito do "cashback" gravava um lançamento com a mesma referência e
// o guard de idempotência pulava a reversão do top-up.
func TestEstorno_ReverteTopUpInteiro(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	seedWallet(t, 42, "establishment", 0)
	comCashback(t)
	seedWallet(t, 100, "customer", 100) // o top-up já creditado
	creditado := time.Now().Add(-time.Minute)
	p := pagamentoNaoLiquidado("order-refund-topup", "charge-refund-topup", time.Minute)
	p.WalletCreditedAt = &creditado
	liquidarEEstornar(t, &p)

	require.InDelta(t, 0.0, getWalletByUser(t, 100).Balance, 0.001,
		"o estorno reverte o top-up inteiro")
}

// comCashback troca o split da zona por um que deixa sobra para o cliente
// (5% plataforma + 80% loja + frete < total → fatia "customer" > 0).
func comCashback(t *testing.T) {
	t.Helper()
	anterior := GetSplitConfigForEstablishment
	GetSplitConfigForEstablishment = func(int64) (float64, float64) { return 5.0, 80.0 }
	t.Cleanup(func() { GetSplitConfigForEstablishment = anterior })
}

func customerCashbackAmount(rules models.SplitRules) float64 {
	var total float64
	for _, r := range rules {
		if r.ReceiverType == "customer" {
			total += r.Amount
		}
	}
	return total
}
