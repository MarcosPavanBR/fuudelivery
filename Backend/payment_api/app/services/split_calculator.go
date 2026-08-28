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
//   - Se deliveryAmount > total: retorna ErrDeliveryExceedsTotal.
//   - platformFee + establishmentAmount + deliveryAmount + customerCredit == total.
//   - customerCredit é o "troco" para cashback do cliente.
func CalculateSplitRules(payment *models.Payment, platformPct, establishmentPct float64) (*SplitResult, error) {
	total := payment.Amount
	deliveryAmount := payment.DeliveryAmount

	if deliveryAmount > total {
		return nil, fmt.Errorf("%w: delivery=%.2f total=%.2f", ErrDeliveryExceedsTotal, deliveryAmount, total)
	}

	platformFee := total * (platformPct / 100.0)
	establishmentAmount := total * (establishmentPct / 100.0)
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
