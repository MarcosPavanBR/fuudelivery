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
// TODO o resto (total − fatia da loja), e dela a plataforma paga o
// entregador. Nunca negativa.
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

// originChargePlan é o resultado da resolução: como cobrar esta venda no modo
// marketplace. OK=false significa "vai pelo fluxo antigo (custódia)".
type originChargePlan struct {
	SellerToken string // access_token do vendedor (Bearer da cobrança)
	AppFeeCents int64  // comissão retida no ato (0 no repasse)
	Repasse     bool   // true = loja recebe 100% e deve frete+comissão (dívida no settle)
	OK          bool
}

// resolveOriginCharge decide se ESTA cobrança vai direto pra conta da loja
// (marketplace) e como: split (comissão retida via application_fee) ou repasse
// (fee=0, loja deve frete+comissão depois). A escolha é por loja, em
// recipients.payment_mode.
//
// Falha SEMPRE em direção ao fluxo antigo (OK=false): flag desligado, loja sem
// conta ativa, token expirado, cofre indisponível ou qualquer erro. Nunca
// derruba a venda — quem não pode ir pela rota nova vai pela antiga.
func resolveOriginCharge(payment *models.Payment) originChargePlan {
	if !splitAtOriginEnabled() || models.DB == nil {
		return originChargePlan{}
	}

	recipient, err := models.ActiveRecipient(models.DB, "mercadopago", "establishment", payment.EstablishmentID)
	if err != nil {
		// Inclui ErrNoActiveRecipient (loja não conectou): fluxo antigo.
		return originChargePlan{}
	}
	if recipient.TokenExpired(time.Now()) {
		log.Printf("[SPLIT-ORIGEM] estabelecimento %d com token MP expirado — caindo no fluxo antigo até reconectar", payment.EstablishmentID)
		return originChargePlan{}
	}

	box, err := secretbox.FromEnv("SECRET_ENCRYPTION_KEY")
	if err != nil {
		log.Printf("[SPLIT-ORIGEM] cofre indisponível (%v) — caindo no fluxo antigo", err)
		return originChargePlan{}
	}
	token, err := recipient.AccessToken(box)
	if err != nil || token == "" {
		log.Printf("[SPLIT-ORIGEM] não consegui decifrar o token do estabelecimento %d (%v) — fluxo antigo", payment.EstablishmentID, err)
		return originChargePlan{}
	}

	// Repasse: a loja recebe 100% (fee=0) e passa a dever frete+comissão. O
	// settle cria a dívida. Não precisa nem calcular o split para a cobrança.
	if recipient.PaymentMode == models.PaymentModeRepasse {
		return originChargePlan{SellerToken: token, AppFeeCents: 0, Repasse: true, OK: true}
	}

	// Split: a comissão da plataforma é retida no ato via application_fee.
	platformPct, establishmentPct := splitConfigFor(payment.EstablishmentID)
	split, err := services.CalculateSplitRules(payment, platformPct, establishmentPct)
	if err != nil {
		log.Printf("[SPLIT-ORIGEM] cálculo de split falhou para o pedido %s (%v) — fluxo antigo", payment.OrderID, err)
		return originChargePlan{}
	}
	fee := applicationFeeCents(toCents(payment.Amount), toCents(split.EstablishmentAmt))
	return originChargePlan{SellerToken: token, AppFeeCents: fee, Repasse: false, OK: true}
}
