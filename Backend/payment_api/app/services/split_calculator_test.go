package services

import (
	"testing"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/stretchr/testify/assert"
)

func TestCalculateSplitRules_NormalCase(t *testing.T) {
	payment := &models.Payment{
		Amount:         100.0,
		DeliveryAmount: 10.0,
		CustomerID:     42,
	}

	result, err := CalculateSplitRules(payment, 10, 80)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 10% platform = 10.0
	// 80% establishment = 80.0
	// delivery = 10.0
	// customer credit = 0.0
	assert.Equal(t, 10.0, result.PlatformFee)
	assert.Equal(t, 80.0, result.EstablishmentAmt)
	assert.Equal(t, 10.0, result.DeliveryAmt)
	assert.Equal(t, 0.0, result.CustomerCredit)
	assert.Equal(t, 3, len(result.Rules)) // platform, establishment, delivery (no customerCredit)
}

func TestCalculateSplitRules_WithCashback(t *testing.T) {
	payment := &models.Payment{
		Amount:         100.0,
		DeliveryAmount: 5.0,
		CustomerID:     42,
	}

	result, err := CalculateSplitRules(payment, 10, 80)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 10% platform = 10.0
	// 80% establishment = 80.0
	// delivery = 5.0
	// customer credit = 100 - 10 - 80 - 5 = 5.0
	assert.Equal(t, 5.0, result.CustomerCredit)
	assert.Equal(t, 4, len(result.Rules)) // platform, establishment, delivery, customer
}

func TestCalculateSplitRules_DeliveryExceedsTotal(t *testing.T) {
	payment := &models.Payment{
		Amount:         50.0,
		DeliveryAmount: 60.0,
		CustomerID:     42,
	}

	// When delivery > amount, platform and establishment are zeroed out.
	// The function no longer returns an error — instead it adjusts the split
	// so that the total never exceeds the payment amount.
	result, err := CalculateSplitRules(payment, 10, 80)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// platform and establishment should be zero
	assert.Equal(t, 0.0, result.PlatformFee)
	assert.Equal(t, 0.0, result.EstablishmentAmt)

	// delivery is clamped to payment amount
	assert.Equal(t, 50.0, result.DeliveryAmt)
	assert.Equal(t, 0.0, result.CustomerCredit)

	// total of all rules should equal payment amount
	total := 0.0
	for _, r := range result.Rules {
		total += r.Amount
	}
	assert.InDelta(t, 50.0, total, 0.01)
}

func TestCalculateSplitRules_ZeroDelivery(t *testing.T) {
	payment := &models.Payment{
		Amount:         100.0,
		DeliveryAmount: 0.0,
		CustomerID:     42,
	}

	result, err := CalculateSplitRules(payment, 10, 80)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, 0.0, result.DeliveryAmt)
	assert.Equal(t, 3, len(result.Rules)) // platform, establishment, customer
}

func TestCalculateSplitRules_TotalSum(t *testing.T) {
	payment := &models.Payment{
		Amount:         100.0,
		DeliveryAmount: 15.0,
		CustomerID:     42,
	}

	result, err := CalculateSplitRules(payment, 10, 80)
	assert.NoError(t, err)

	totalDistributed := result.PlatformFee + result.EstablishmentAmt + result.DeliveryAmt + result.CustomerCredit
	assert.InDelta(t, payment.Amount, totalDistributed, 0.001)
}
