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
	assert.Equal(t, 4, len(result.Rules))
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
	assert.Equal(t, 5, len(result.Rules))
}

func TestCalculateSplitRules_DeliveryExceedsTotal(t *testing.T) {
	payment := &models.Payment{
		Amount:         50.0,
		DeliveryAmount: 60.0,
		CustomerID:     42,
	}

	result, err := CalculateSplitRules(payment, 10, 80)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrDeliveryExceedsTotal)
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
