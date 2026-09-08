package services

import (
	"fmt"
	"math"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
)

// roundCents arredonda um valor em reais para 2 casas.
//
// Sem isso, `total * (pct/100)` produz dízima de float (ex.: 0.30000000000000004
// ou 2.9997) e esse valor ia direto pro ledger e pro gateway — lançamentos com
// fração de centavo que fazem a conta não fechar na conciliação.
//
// Só as parcelas DERIVADAS são arredondadas; o customerCredit continua sendo
// calculado como resto (total - as outras três), o que preserva o invariante
// documentado de que as quatro partes somam exatamente o total.
func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}

// ErrDeliveryExceedsTotal é retornado quando o valor da entrega excede o total
// do pagamento, tornando o split impossível.
var ErrDeliveryExceedsTotal = fmt.Errorf("delivery amount exceeds payment total")

// SplitResult contém o resultado do cálculo de split.
type SplitResult struct {
	Rules            []models.SplitRule
	PlatformFee      float64
	EstablishmentAmt float64
	DeliveryAmt      float64
	CustomerCredit   float64
}

// CalculateSplitRules calcula as regras de split de forma determinística e
// segura, garantindo que a soma dos valores nunca exceda o total.
//
// Regras:
//   - Se deliveryAmount >= total: platform e establishment são zerados.
//   - platformFee + establishmentAmount + deliveryAmount + customerCredit == total.
//   - customerCredit é o "troco" para cashback do cliente.
func CalculateSplitRules(payment *models.Payment, platformPct, establishmentPct float64) (*SplitResult, error) {
	total := payment.Amount
	deliveryAmount := payment.DeliveryAmount

	// When delivery exceeds the payment total, zero out platform and
	// establishment shares — the delivery fee consumes the entire amount.
	// The caller (defaultSplitRules) expects a valid result, not an error.
	platformFee := roundCents(total * (platformPct / 100.0))
	establishmentAmount := roundCents(total * (establishmentPct / 100.0))

	if deliveryAmount >= total {
		platformFee = 0
		establishmentAmount = 0
		deliveryAmount = total
	}

	// Teto da plataforma: o que sobra depois da entrega.
	//
	// Sem isto o invariante quebrava de verdade. Exemplo real (total=20,
	// entrega=19, 10%/85%): o establishment absorvia o excesso e clampava em
	// 0, mas platformFee (2,00) + delivery (19,00) já somavam 21,00 sozinhos —
	// ninguém reduzia a taxa da plataforma, e o split alocava mais do que o
	// pagamento tinha. A entrega tem prioridade porque é custo real do
	// entregador; a taxa da plataforma cede o que faltar.
	maxPlatform := roundCents(total - deliveryAmount)
	if maxPlatform < 0 {
		maxPlatform = 0
	}
	if platformFee > maxPlatform {
		platformFee = maxPlatform
	}

	// Garante que platformFee + establishment + delivery nunca exceda o total.
	// Se o delivery consome parte do bolo, o establishment absorve a diferença
	// (o platform já foi limitado acima).
	allocated := platformFee + establishmentAmount + deliveryAmount
	if allocated > total {
		overage := allocated - total
		establishmentAmount = roundCents(establishmentAmount - overage)
		if establishmentAmount < 0 {
			establishmentAmount = 0
		}
	}

	// Resto: absorve qualquer resíduo do arredondamento acima, mantendo
	// a soma das quatro partes exatamente igual ao total.
	customerCredit := roundCents(total - platformFee - establishmentAmount - deliveryAmount)
	if customerCredit < 0 {
		customerCredit = 0
	}

	rules := []models.SplitRule{
		{
			ReceiverID:   0,
			ReceiverType: "platform",
			Amount:       platformFee,
			Percentage:   platformPct,
		},
		{
			ReceiverID:   payment.EstablishmentID,
			ReceiverType: "establishment",
			Amount:       establishmentAmount,
			Percentage:   establishmentPct,
		},
	}

	if deliveryAmount > 0 {
		rules = append(rules, models.SplitRule{
			ReceiverID:   0,
			ReceiverType: "deliveryman",
			Amount:       deliveryAmount,
			Percentage:   0,
		})
	}

	if customerCredit > 0 {
		rules = append(rules, models.SplitRule{
			ReceiverID:   payment.CustomerID,
			ReceiverType: "customer",
			Amount:       customerCredit,
			Percentage:   0,
		})
	}

	return &SplitResult{
		Rules:            rules,
		PlatformFee:      platformFee,
		EstablishmentAmt: establishmentAmount,
		DeliveryAmt:      deliveryAmount,
		CustomerCredit:   customerCredit,
	}, nil
}
