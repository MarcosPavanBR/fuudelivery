//go:build integration

package handlers

import (
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/stretchr/testify/require"
)

// pagamentoNaoLiquidado devolve um pagamento no estado EXATO que a falha
// deixa: CONFIRMED, com confirmed_at no passado e establishment_credited_at
// nulo. É o que sobra quando o processo morre entre o UPDATE do status e o
// crédito da carteira, ou quando CalculateSplitRules falha.
func pagamentoNaoLiquidado(orderID, chargeID string, confirmadoHa time.Duration) models.Payment {
	confirmadoEm := time.Now().Add(-confirmadoHa)
	return models.Payment{
		OrderID:         orderID,
		CustomerID:      100,
		CustomerPhone:   "+5511999999999",
		EstablishmentID: 42,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    chargeID,
		CreatedAt:       confirmadoEm,
		ConfirmedAt:     &confirmadoEm,
		// EstablishmentCreditedAt deliberadamente nil: o dinheiro não chegou
		// ao restaurante.
	}
}

// TestReconciliacao_LiquidaPagamentoAbandonado é o teste do cenário real.
//
// Falsificação: sem a chamada a settlePaymentApproved dentro do job, a
// carteira continua zerada e o teste quebra.
func TestReconciliacao_LiquidaPagamentoAbandonado(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	pagamento := pagamentoNaoLiquidado("order-recon-001", "charge-recon-001", 5*time.Minute)
	seedPayment(t, &pagamento)

	// A carteira do restaurante existe e está zerada — o webhook confirmou o
	// pagamento mas nunca creditou.
	seedWallet(t, 42, "establishment", 0)

	antes := getWalletByUser(t, 42)
	require.InDelta(t, 0.0, antes.Balance, 0.001, "pré-condição: carteira zerada")

	stats := ReconcilePaymentsOnce()

	require.Equal(t, 1, stats.Pending, "o job deve encontrar o pagamento abandonado")
	require.Equal(t, 1, stats.Healed, "e deve liquidá-lo")
	require.Equal(t, 0, stats.Failed)

	depois := getWalletByUser(t, 42)
	require.InDelta(t, 100.00*0.85, depois.Balance, 0.01,
		"a carteira do restaurante deve receber o share do split (85%%)")

	liquidado := findPaymentByAbacate(t, "charge-recon-001")
	require.NotNil(t, liquidado.EstablishmentCreditedAt,
		"establishment_credited_at precisa ficar gravado, senão o job tenta de novo para sempre")
	require.Len(t, liquidado.SplitRules, 3, "split deve ter sido calculado e persistido")

	require.Equal(t, int64(1), countLedger(t, 42, "credit", "", "charge-recon-001"),
		"exatamente 1 lançamento de crédito no ledger")
}

// TestReconciliacao_DuasPassadasNaoDuplicamDinheiro é o teste que sustenta a
// decisão de desenho inteira.
//
// Todo o plano se apoia em "reprocessar é seguro por causa da idempotência que
// já existe no banco". Se esta afirmação for falsa, a reconciliação não é uma
// rede de segurança — é uma máquina de creditar duas vezes, e seria melhor não
// existir. Por isso o job roda DUAS vezes aqui.
//
// Regra do projeto (CLAUDE.md): toda operação financeira tem teste de
// idempotência.
func TestReconciliacao_DuasPassadasNaoDuplicamDinheiro(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	pagamento := pagamentoNaoLiquidado("order-recon-002", "charge-recon-002", 5*time.Minute)
	seedPayment(t, &pagamento)
	seedWallet(t, 42, "establishment", 0)

	primeira := ReconcilePaymentsOnce()
	require.Equal(t, 1, primeira.Healed)

	saldoAposPrimeira := getWalletByUser(t, 42).Balance
	require.InDelta(t, 100.00*0.85, saldoAposPrimeira, 0.01)

	// Segunda passada, sem nada ter mudado no mundo.
	//
	// A primeira versão deste teste passava pelo motivo ERRADO: settle
	// sobrescrevia confirmed_at com `now`, o que jogava o pagamento de volta
	// para dentro da janela de graça, e era a JANELA — não a idempotência —
	// que o excluía da segunda passada. Descobri isso ao falsificar: remover
	// o filtro `establishment_credited_at IS NULL` não quebrava o teste.
	//
	// A asserção abaixo trava o motivo certo: com confirmed_at preservado, o
	// pagamento continua elegível pela janela, e só não volta porque foi
	// creditado.
	liquidado := findPaymentByAbacate(t, "charge-recon-002")
	require.NotNil(t, liquidado.ConfirmedAt)
	require.True(t, liquidado.ConfirmedAt.Before(time.Now().Add(-reconcileGracePeriod)),
		"confirmed_at tem que continuar sendo a hora REAL do pagamento; se o settle o "+
			"reescreve com now, a janela de graça mascara a duplicação em vez da idempotência")

	segunda := ReconcilePaymentsOnce()

	require.Equal(t, 0, segunda.Pending,
		"depois de liquidado, o pagamento não pode mais aparecer como pendente — "+
			"se aparecer, o job fica tentando creditá-lo a cada 5 minutos para sempre")
	require.Equal(t, 0, segunda.Healed)

	saldoFinal := getWalletByUser(t, 42).Balance
	require.InDelta(t, saldoAposPrimeira, saldoFinal, 0.001,
		"a segunda passada NÃO pode mexer no saldo")

	require.Equal(t, int64(1), countLedger(t, 42, "credit", "", "charge-recon-002"),
		"o ledger deve ter exatamente 1 crédito, não 2")
}

// TestReconciliacao_RespeitaJanelaDeGraca garante que o job não dispute com o
// webhook que ainda está executando.
//
// Sem a janela, um pagamento recém-confirmado seria pego pelo job enquanto o
// caminho síncrono ainda está no meio do split, e os dois tentariam creditar ao
// mesmo tempo. A idempotência aguentaria, mas produziria erro e ruído — e ruído
// num log de dinheiro é o que faz ninguém olhar o log de dinheiro.
func TestReconciliacao_RespeitaJanelaDeGraca(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	// Confirmado há 30 segundos: dentro da janela de graça de 2 minutos.
	pagamento := pagamentoNaoLiquidado("order-recon-003", "charge-recon-003", 30*time.Second)
	seedPayment(t, &pagamento)
	seedWallet(t, 42, "establishment", 0)

	stats := ReconcilePaymentsOnce()

	require.Equal(t, 0, stats.Pending,
		"pagamento dentro da janela de graça não pode ser tocado: o webhook ainda pode estar liquidando")
	require.Equal(t, 0, stats.Healed)

	require.InDelta(t, 0.0, getWalletByUser(t, 42).Balance, 0.001,
		"a carteira não pode ser creditada pelo job durante a janela de graça")
}

// TestReconciliacao_IgnoraPagamentoJaLiquidado confirma que o filtro da
// consulta é establishment_credited_at, e não só o status.
func TestReconciliacao_IgnoraPagamentoJaLiquidado(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	creditadoEm := time.Now().Add(-10 * time.Minute)
	pagamento := pagamentoNaoLiquidado("order-recon-004", "charge-recon-004", 30*time.Minute)
	pagamento.EstablishmentCreditedAt = &creditadoEm
	seedPayment(t, &pagamento)
	seedWallet(t, 42, "establishment", 85.00)

	stats := ReconcilePaymentsOnce()

	require.Equal(t, 0, stats.Pending, "pagamento já creditado não é pendente")
	require.InDelta(t, 85.00, getWalletByUser(t, 42).Balance, 0.001,
		"saldo de um pagamento já liquidado não pode ser mexido")
}

// TestReconciliacao_IgnoraPagamentoAntigoDemais garante o limite da varredura.
// Pagamento não liquidado há mais de uma semana é caso para investigar à mão,
// não para o job tentar para sempre a cada 5 minutos.
func TestReconciliacao_IgnoraPagamentoAntigoDemais(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	pagamento := pagamentoNaoLiquidado("order-recon-005", "charge-recon-005", 8*24*time.Hour)
	seedPayment(t, &pagamento)
	seedWallet(t, 42, "establishment", 0)

	stats := ReconcilePaymentsOnce()

	require.Equal(t, 0, stats.Pending, "fora da janela de 7 dias o job não deve pegar o pagamento")
	require.InDelta(t, 0.0, getWalletByUser(t, 42).Balance, 0.001)
}

// TestReconciliacao_NaoReescreveConfirmedAt trava a hora real do pagamento.
//
// settlePaymentApproved é reentrante (webhook reenviado, aprovação manual no
// admin, reconciliação). Se cada passagem gravasse `now` em confirmed_at, o
// extrato passaria a mentir sobre quando o cliente pagou — e o próprio job
// perderia de vista os pagamentos que acabou de tocar, já que a consulta
// filtra por confirmed_at.
func TestReconciliacao_NaoReescreveConfirmedAt(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	pagamento := pagamentoNaoLiquidado("order-recon-006", "charge-recon-006", 45*time.Minute)
	seedPayment(t, &pagamento)
	seedWallet(t, 42, "establishment", 0)

	original := findPaymentByAbacate(t, "charge-recon-006")
	require.NotNil(t, original.ConfirmedAt)
	horaReal := *original.ConfirmedAt

	require.Equal(t, 1, ReconcilePaymentsOnce().Healed)

	depois := findPaymentByAbacate(t, "charge-recon-006")
	require.NotNil(t, depois.ConfirmedAt)
	require.WithinDuration(t, horaReal, *depois.ConfirmedAt, time.Second,
		"a reconciliação liquidou o pagamento, mas NÃO pode reescrever a hora em que ele foi confirmado")
}
