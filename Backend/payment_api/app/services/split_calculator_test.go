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

// TestCalculateSplitRules_ArredondaCentavos cobre o caso que os testes acima
// não pegavam: todos usam valores redondos (100.0 / 10% / 80%), que nunca
// produzem dízima. Com valores reais de pedido, `total * (pct/100)` gera
// coisas como 2.9997 ou 0.30000000000000004 — fração de centavo indo parar no
// ledger e no gateway, fazendo a conciliação não fechar.
func TestCalculateSplitRules_ArredondaCentavos(t *testing.T) {
	payment := &models.Payment{
		Amount:         99.99, // valor típico de pedido, não redondo
		DeliveryAmount: 7.77,
		CustomerID:     42,
	}

	result, err := CalculateSplitRules(payment, 3, 87)
	assert.NoError(t, err)

	// 99.99 * 0.03 = 2.9997  -> 3.00
	assert.Equal(t, 3.00, result.PlatformFee, "taxa da plataforma deve ficar em centavos exatos")
	// 99.99 * 0.87 = 86.9913 -> 86.99
	assert.Equal(t, 86.99, result.EstablishmentAmt, "parte do estabelecimento deve ficar em centavos exatos")

	// Nenhuma parcela pode ter mais de 2 casas decimais.
	for _, r := range result.Rules {
		cents := r.Amount * 100
		assert.InDelta(t, cents, float64(int64(cents+0.5)), 0.001,
			"regra %s tem fração de centavo: %v", r.ReceiverType, r.Amount)
	}

	// O invariante documentado continua valendo: as quatro partes somam o total.
	totalDistribuido := result.PlatformFee + result.EstablishmentAmt + result.DeliveryAmt + result.CustomerCredit
	assert.InDelta(t, payment.Amount, totalDistribuido, 0.001,
		"arredondar não pode quebrar a soma — o customerCredit absorve o resto")
}

// TestCalculateSplitRules_NaoSuperAloca cobre o ramo que os outros testes não
// tocavam e onde o invariante ESTAVA QUEBRADO: quando a entrega consome quase
// todo o pagamento, o establishment zerava mas a taxa da plataforma continuava
// inteira, e platform + delivery sozinhos passavam do total.
//
// Caso real encontrado na revisão: total=20, entrega=19, 10%/85% produzia
// plat=2.00 + del=19.00 = 21.00 para um pagamento de 20.00.
func TestCalculateSplitRules_NaoSuperAloca(t *testing.T) {
	casos := []struct {
		nome    string
		total   float64
		entrega float64
		platPct float64
		estPct  float64
	}{
		{"entrega consome quase tudo", 20.0, 19.0, 10, 85},
		{"entrega igual ao total", 20.0, 20.0, 10, 85},
		{"entrega maior que o total", 20.0, 25.0, 10, 85},
		{"entrega no limite da taxa", 100.0, 90.0, 10, 85},
		{"valores nao redondos", 99.99, 95.55, 3, 87},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := &models.Payment{Amount: c.total, DeliveryAmount: c.entrega, CustomerID: 1}
			r, err := CalculateSplitRules(p, c.platPct, c.estPct)
			assert.NoError(t, err)

			soma := r.PlatformFee + r.EstablishmentAmt + r.DeliveryAmt + r.CustomerCredit
			assert.LessOrEqual(t, soma, c.total+0.001,
				"split alocou %.2f para um pagamento de %.2f", soma, c.total)
			assert.InDelta(t, c.total, soma, 0.011,
				"as quatro partes devem somar o total")

			// Nenhuma parcela negativa.
			assert.GreaterOrEqual(t, r.PlatformFee, 0.0)
			assert.GreaterOrEqual(t, r.EstablishmentAmt, 0.0)
			assert.GreaterOrEqual(t, r.CustomerCredit, 0.0)

			// A soma das regras emitidas também não pode passar do total.
			somaRegras := 0.0
			for _, rule := range r.Rules {
				somaRegras += rule.Amount
			}
			assert.LessOrEqual(t, somaRegras, c.total+0.001,
				"regras emitidas somam %.2f para um pagamento de %.2f", somaRegras, c.total)
		})
	}
}
