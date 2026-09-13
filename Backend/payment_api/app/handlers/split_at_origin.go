package handlers

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/carloshomar/fuudelivery/pkg/secretbox"
)

// Split na origem no momento da cobrança.
//
// Quando o estabelecimento conectou a conta Mercado Pago dele E o recurso está
// ligado, a cobrança é criada com o token do vendedor e uma application_fee: a
// fatia da loja cai DIRETO na conta dela, e a plataforma recebe só a comissão.
// O dinheiro da venda não passa pela plataforma.
//
// Regra do rollout (decisão do Marcos): GRADUAL por loja. Quem conectou vai por
// aqui; quem não conectou segue no fluxo antigo (custódia) sem interrupção. E
// tudo isso atrás de um flag master que nasce DESLIGADO — o deploy não muda
// nada em produção até o flag ser ligado (depois de validar em sandbox).

// splitAtOriginEnabled é o flag master. Nasce desligado: sem
// SPLIT_AT_ORIGIN_ENABLED=true, nada muda no caminho do dinheiro.
func splitAtOriginEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("SPLIT_AT_ORIGIN_ENABLED")), "true")
}

// applicationFeeCents calcula a comissão da plataforma no modelo de 4→2 partes
// que o Marcos escolheu: a loja recebe a fatia dela direto; a application_fee é
// TODO o resto (total − fatia da loja), e dela a plataforma paga entregador e
// cashback. Nunca negativa.
func applicationFeeCents(totalCents, establishmentShareCents int64) int64 {
	fee := totalCents - establishmentShareCents
	if fee < 0 {
		return 0
	}
	return fee
}

// splitConfigFor devolve os percentuais de split do estabelecimento, com o
// mesmo default (5/85) e a mesma fonte que o settle usa.
func splitConfigFor(establishmentID int64) (platformPct, establishmentPct float64) {
	if GetSplitConfigForEstablishment != nil {
		return GetSplitConfigForEstablishment(establishmentID)
	}
	return 5.0, 85.0
}

// resolveSplitAtOrigin decide se ESTA cobrança vai por split na origem e, se
// sim, devolve o token do vendedor e a application_fee em centavos.
//
// Falha SEMPRE em direção ao fluxo antigo (ok=false): flag desligado, loja sem
// conta ativa, token expirado, cofre indisponível ou qualquer erro. Nunca
// derruba a venda por causa do split — quem não pode ir pela rota nova vai pela
// antiga. Bloquear a venda só entra depois, e é outra decisão.
func resolveSplitAtOrigin(payment *models.Payment) (sellerToken string, appFeeCents int64, ok bool) {
	if !splitAtOriginEnabled() {
		return "", 0, false
	}
	if models.DB == nil {
		return "", 0, false
	}

	recipient, err := models.ActiveRecipient(models.DB, "mercadopago", "establishment", payment.EstablishmentID)
	if err != nil {
		// Inclui ErrNoActiveRecipient (loja não conectou): fluxo antigo.
		return "", 0, false
	}
	if recipient.TokenExpired(time.Now()) {
		log.Printf("[SPLIT-ORIGEM] estabelecimento %d com token MP expirado — caindo no fluxo antigo até reconectar", payment.EstablishmentID)
		return "", 0, false
	}

	box, err := secretbox.FromEnv("SECRET_ENCRYPTION_KEY")
	if err != nil {
		log.Printf("[SPLIT-ORIGEM] cofre indisponível (%v) — caindo no fluxo antigo", err)
		return "", 0, false
	}
	token, err := recipient.AccessToken(box)
	if err != nil || token == "" {
		log.Printf("[SPLIT-ORIGEM] não consegui decifrar o token do estabelecimento %d (%v) — fluxo antigo", payment.EstablishmentID, err)
		return "", 0, false
	}

	platformPct, establishmentPct := splitConfigFor(payment.EstablishmentID)
	split, err := services.CalculateSplitRules(payment, platformPct, establishmentPct)
	if err != nil {
		log.Printf("[SPLIT-ORIGEM] cálculo de split falhou para o pedido %s (%v) — fluxo antigo", payment.OrderID, err)
		return "", 0, false
	}

	fee := applicationFeeCents(toCents(payment.Amount), toCents(split.EstablishmentAmt))
	return token, fee, true
}
