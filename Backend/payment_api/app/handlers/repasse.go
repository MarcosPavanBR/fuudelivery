package handlers

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
)

// Modelo de repasse: o restaurante recebe o pedido direto e DEVE à plataforma o
// frete + a comissão. É o plano B do split na origem — troca risco regulatório
// por risco de crédito, então nasce atrás de um flag desligado E com trava de
// crédito por loja. Sem a trava, uma loja acumularia dívida sem limite e a
// plataforma viraria uma financeira sem garantia.

// repasseCreditLimitDefaultCents é o teto de dívida em aberto por loja quando
// REPASSE_CREDIT_LIMIT_CENTS não é configurado: R$ 500,00. Conservador de
// propósito — o Marcos sobe por loja conforme a confiança.
const repasseCreditLimitDefaultCents = 50000

// repasseEnabled é o flag master. Nasce desligado: sem REPASSE_ENABLED=true,
// nenhuma dívida é criada e a trava não bloqueia nada.
func repasseEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("REPASSE_ENABLED")), "true")
}

// repasseCreditLimitCents devolve o teto de dívida em aberto por loja. Valor
// inválido ou ausente cai no default conservador (nunca "sem limite").
func repasseCreditLimitCents() int64 {
	v := strings.TrimSpace(os.Getenv("REPASSE_CREDIT_LIMIT_CENTS"))
	if v == "" {
		return repasseCreditLimitDefaultCents
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		log.Printf("[REPASSE] REPASSE_CREDIT_LIMIT_CENTS inválido (%q) — usando o default %d", v, repasseCreditLimitDefaultCents)
		return repasseCreditLimitDefaultCents
	}
	return n
}

// debtBlocksNewOrder é a regra pura da trava: a loja é bloqueada quando a
// dívida em aberto atinge o limite. >= (não >) para o limite ser um teto de
// fato — atingiu, parou.
func debtBlocksNewOrder(openDebtCents, limitCents int64) bool {
	return openDebtCents >= limitCents
}

// EstablishmentBlockedByDebt diz se a loja deve ser impedida de receber novos
// pedidos por dívida de repasse em aberto acima do limite.
//
// Falha ABERTA de propósito: se o repasse está desligado, ou não dá para
// consultar a dívida, a venda NÃO é bloqueada — a trava é uma proteção de
// crédito, não um ponto único de falha que derruba o faturamento se o banco
// oscilar. Devolve também os números, para quem chama logar/mostrar o motivo.
func EstablishmentBlockedByDebt(establishmentID int64) (blocked bool, openCents, limitCents int64) {
	if !repasseEnabled() || models.DB == nil {
		return false, 0, repasseCreditLimitCents()
	}
	limitCents = repasseCreditLimitCents()
	openCents, err := models.OpenDebtTotalCents(models.DB, establishmentID)
	if err != nil {
		log.Printf("[REPASSE] não consegui somar dívida da loja %d (%v) — não bloqueando", establishmentID, err)
		return false, 0, limitCents
	}
	return debtBlocksNewOrder(openCents, limitCents), openCents, limitCents
}

// recordRepasseDebt cria a dívida da loja para um pagamento em repasse: ela
// recebeu 100%, então deve o frete (dinheiro do entregador) + a comissão da
// plataforma. Idempotente por pedido (CreateDebt usa ON CONFLICT no order_id),
// então o settle reprocessado não duplica a dívida.
//
// O cashback do cliente NÃO entra na dívida: no modelo 4→2 ele é despesa da
// plataforma, paga da comissão que ela cobra — mesma lógica do split.
func recordRepasseDebt(payment *models.Payment, now time.Time) error {
	platformPct, establishmentPct := splitConfigFor(payment.EstablishmentID)
	split, err := services.CalculateSplitRules(payment, platformPct, establishmentPct)
	if err != nil {
		return fmt.Errorf("calcular split para a dívida: %w", err)
	}
	created, err := models.CreateDebt(models.DB, &models.EstablishmentDebt{
		EstablishmentID:  payment.EstablishmentID,
		OrderID:          payment.OrderID,
		PaymentID:        payment.ID,
		DeliveryAmount:   split.DeliveryAmt, // frete (dinheiro do entregador)
		CommissionAmount: split.PlatformFee, // comissão da plataforma
		CreatedAt:        now,
	})
	if err != nil {
		return err
	}
	if created {
		log.Printf("[REPASSE] dívida criada: loja %d deve %.2f (frete %.2f + comissão %.2f) do pedido %s",
			payment.EstablishmentID, split.DeliveryAmt+split.PlatformFee, split.DeliveryAmt, split.PlatformFee, payment.OrderID)
	}
	return nil
}
