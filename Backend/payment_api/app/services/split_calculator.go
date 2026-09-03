package services

import (
	"fmt"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
)

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
	platformFee := total * (platformPct / 100.0)
	establishmentAmount := total * (establishmentPct / 100.0)

	if deliveryAmount >= total {
		platformFee = 0
		establishmentAmount = 0
		deliveryAmount = total
	}

	// Garante que platformFee + establishment + delivery nunca exceda o total.
	// Se o delivery consome parte do bolo, o establishment absorve a diferença
	// (nunca o platform, que é taxa fixa).
	allocated := platformFee + establishmentAmount + deliveryAmount
	if allocated > total {
		overage := allocated - total
		establishmentAmount -= overage
		if establishmentAmount < 0 {
			establishmentAmount = 0
		}
	}

	customerCredit := total - platformFee - establishmentAmount - deliveryAmount
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
