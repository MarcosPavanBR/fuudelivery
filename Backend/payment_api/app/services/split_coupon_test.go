package services

// split_coupon_test.go — o desconto do cupom sai de quem escolheu bancar.
//
// Marcos pediu poder escolher, ao criar o cupom, se a promoção sai da empresa
// ou do restaurante. O split é onde essa escolha vira dinheiro de verdade, e é
// onde ela se perdia sem estes testes: aplicar as porcentagens sobre o valor JÁ
// descontado rateia a promoção entre os dois lados em proporção às fatias,
// qualquer que tenha sido a escolha. O cupom "descontar de mim" tirava dinheiro
// do restaurante do mesmo jeito, só que menos — e ninguém veria, porque a conta
// fecha: as quatro partes continuam somando o total pago.
//
// O método de todos os testes abaixo é o mesmo: comparar o MESMO pedido com e
// sem cupom. É a única forma de ver de quem o desconto saiu.

import (
	"testing"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/stretchr/testify/assert"
)

// somaDasPartes é o invariante que vale em todo caso: o split aloca exatamente
// o que o cliente pagou, nem mais nem menos.
func somaDasPartes(r *SplitResult) float64 {
	return roundCents(r.PlatformFee + r.EstablishmentAmt + r.DeliveryAmt + r.CustomerCredit)
}

// Pedido de referência: R$100 de bruto, R$10 de frete, 10% plataforma / 80%
// loja. Sem cupom: plataforma 10, loja 80, entrega 10, cashback 0.
func semCupom() *models.Payment {
	return &models.Payment{Amount: 100.0, DeliveryAmount: 10.0, CustomerID: 42}
}

func TestSplitComCupom_PlataformaBancaSozinha(t *testing.T) {
	base, err := CalculateSplitRules(semCupom(), 10, 80)
	assert.NoError(t, err)

	// Mesmo pedido, cupom de R$5 bancado pela PLATAFORMA: o cliente paga 95.
	comCupom := &models.Payment{
		Amount: 95.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: 5.0, DiscountFundedBy: "platform",
	}
	res, err := CalculateSplitRules(comCupom, 10, 80)
	assert.NoError(t, err)

	// A plataforma perde exatamente os R$5 do cupom.
	assert.Equal(t, roundCents(base.PlatformFee-5.0), res.PlatformFee,
		"a plataforma bancou, então é a fatia dela que encolhe")
	// E o restaurante recebe EXATAMENTE o que receberia sem cupom nenhum.
	assert.Equal(t, base.EstablishmentAmt, res.EstablishmentAmt,
		"o restaurante não pode pagar uma promoção da plataforma")
	// O entregador também não entra na conta da promoção.
	assert.Equal(t, base.DeliveryAmt, res.DeliveryAmt)
	// E o cashback do cliente não encolhe por ele ter usado um cupom.
	assert.Equal(t, base.CustomerCredit, res.CustomerCredit)

	assert.Equal(t, 95.0, somaDasPartes(res), "as quatro partes somam o pago")
}

func TestSplitComCupom_EstabelecimentoBancaSozinho(t *testing.T) {
	base, err := CalculateSplitRules(semCupom(), 10, 80)
	assert.NoError(t, err)

	comCupom := &models.Payment{
		Amount: 95.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: 5.0, DiscountFundedBy: "establishment",
	}
	res, err := CalculateSplitRules(comCupom, 10, 80)
	assert.NoError(t, err)

	assert.Equal(t, roundCents(base.EstablishmentAmt-5.0), res.EstablishmentAmt,
		"o restaurante bancou, então é a fatia dele que encolhe")
	assert.Equal(t, base.PlatformFee, res.PlatformFee,
		"a plataforma não paga uma promoção do restaurante")
	assert.Equal(t, base.DeliveryAmt, res.DeliveryAmt)
	assert.Equal(t, base.CustomerCredit, res.CustomerCredit)

	assert.Equal(t, 95.0, somaDasPartes(res))
}

// O caso que prova que a escolha IMPORTA: o mesmo pedido, o mesmo desconto, e
// os dois lados recebem valores diferentes conforme quem banca. Sem a leitura
// de FundedBy os dois resultados seriam idênticos.
func TestSplitComCupom_AEscolhaMudaOResultado(t *testing.T) {
	daPlataforma := &models.Payment{
		Amount: 95.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: 5.0, DiscountFundedBy: "platform",
	}
	daLoja := &models.Payment{
		Amount: 95.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: 5.0, DiscountFundedBy: "establishment",
	}

	p, err := CalculateSplitRules(daPlataforma, 10, 80)
	assert.NoError(t, err)
	l, err := CalculateSplitRules(daLoja, 10, 80)
	assert.NoError(t, err)

	assert.NotEqual(t, p.PlatformFee, l.PlatformFee,
		"quem banca decide de qual fatia sai — os dois casos não podem dar igual")
	assert.NotEqual(t, p.EstablishmentAmt, l.EstablishmentAmt)
	// Os dois continuam alocando exatamente o que o cliente pagou.
	assert.Equal(t, 95.0, somaDasPartes(p))
	assert.Equal(t, 95.0, somaDasPartes(l))
}

// Sem a leitura do bruto, ESTE é o número que sairia: 10% e 80% de 95, ou seja
// 9,50 e 76,00 — a plataforma perdendo 0,50 e o restaurante 4,00 num cupom que
// a plataforma disse bancar. O teste fixa o valor errado para que ele não possa
// voltar sem alguém notar.
func TestSplitComCupom_NaoRateiaAPromocaoEntreOsDois(t *testing.T) {
	comCupom := &models.Payment{
		Amount: 95.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: 5.0, DiscountFundedBy: "platform",
	}
	res, err := CalculateSplitRules(comCupom, 10, 80)
	assert.NoError(t, err)

	assert.NotEqual(t, 9.50, res.PlatformFee,
		"9,50 é 10%% do valor JÁ descontado — a porcentagem incide sobre o bruto")
	assert.NotEqual(t, 76.00, res.EstablishmentAmt,
		"76,00 é 80%% do valor já descontado — o restaurante estaria pagando 4,00")
	assert.Equal(t, 5.00, res.PlatformFee)
	assert.Equal(t, 80.00, res.EstablishmentAmt)
}

// Cupom maior do que a fatia de quem banca.
//
// A fatia do financiador zera primeiro — mas o desconto pode ser maior do que
// tudo o que ele tem para dar, e aí a diferença sai do outro lado por
// aritmética, não por escolha: o split só distribui o que o cliente pagou.
// Bruto 100 (plataforma 10, loja 80, entrega 10, cashback 0), cupom de 30 pela
// plataforma, cliente paga 70. A plataforma dá seus 10; sobram 20 que não
// existem em lugar nenhum além da fatia da loja, porque 80 + 10 de entrega já
// passa dos 70 recebidos.
//
// O teste fixa esse limite explicitamente para que ele seja uma decisão visível
// e não uma surpresa na conciliação: quem banca dá TUDO antes de o outro lado
// perder um centavo, e a loja perde só o inevitável (20), nunca o desconto
// inteiro (30). CalculateSplitRules loga o caso — é sinal de cupom criado acima
// da margem de quem o ofereceu.
func TestSplitComCupom_MaiorQueAFatiaDeQuemBancaSoCobraOInevitavel(t *testing.T) {
	base, err := CalculateSplitRules(semCupom(), 10, 80)
	assert.NoError(t, err)

	comCupom := &models.Payment{
		Amount: 70.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: 30.0, DiscountFundedBy: "platform",
	}
	res, err := CalculateSplitRules(comCupom, 10, 80)
	assert.NoError(t, err)

	assert.Equal(t, 0.0, res.PlatformFee, "quem banca dá tudo o que tem, primeiro")

	// A loja perde 20 — o que faltou depois de a plataforma zerar — e não os
	// 30 do cupom.
	perdaDaLoja := roundCents(base.EstablishmentAmt - res.EstablishmentAmt)
	assert.Equal(t, 20.0, perdaDaLoja,
		"a loja só absorve o que sobrou depois de a plataforma zerar")
	assert.Less(t, perdaDaLoja, 30.0,
		"a loja nunca paga o desconto inteiro de uma promoção que não é dela")

	assert.Equal(t, 70.0, somaDasPartes(res))
}

// O mesmo pedido com um cupom que CABE na fatia de quem banca: aí o outro lado
// não perde nada mesmo. É o contraste que mostra que o caso acima é limite
// aritmético, e não a regra.
func TestSplitComCupom_QueCabeNaFatiaNaoTocaNoOutroLado(t *testing.T) {
	base, err := CalculateSplitRules(semCupom(), 10, 80)
	assert.NoError(t, err)

	// Cupom de 8, fatia da plataforma é 10.
	comCupom := &models.Payment{
		Amount: 92.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: 8.0, DiscountFundedBy: "platform",
	}
	res, err := CalculateSplitRules(comCupom, 10, 80)
	assert.NoError(t, err)

	assert.Equal(t, 2.0, res.PlatformFee, "10 − 8 = 2")
	assert.Equal(t, base.EstablishmentAmt, res.EstablishmentAmt,
		"cabendo na fatia de quem banca, o outro lado fica intacto")
	assert.Equal(t, 92.0, somaDasPartes(res))
}

func TestSplitComCupom_MaiorQueAFatiaDaLojaNaoCobraDaPlataforma(t *testing.T) {
	base, err := CalculateSplitRules(semCupom(), 10, 80)
	assert.NoError(t, err)

	// Bruto 100, cupom de 90 bancado pela loja, cuja fatia é 80.
	comCupom := &models.Payment{
		Amount: 10.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: 90.0, DiscountFundedBy: "establishment",
	}
	res, err := CalculateSplitRules(comCupom, 10, 80)
	assert.NoError(t, err)

	assert.Equal(t, 0.0, res.EstablishmentAmt)
	// Aqui o frete (10) consome todo o pagamento (10), então a regra de
	// `deliveryAmount >= total` zera os dois lados — o entregador tem
	// prioridade porque é custo real.
	assert.Equal(t, 0.0, res.PlatformFee)
	assert.Equal(t, 10.0, res.DeliveryAmt)
	assert.Equal(t, 10.0, somaDasPartes(res))
	assert.LessOrEqual(t, res.PlatformFee, base.PlatformFee)
}

// ── Entradas que não podem virar dinheiro ──

// funded_by desconhecido cai no default da plataforma, e nunca no
// restaurante: cobrar de terceiro por omissão é o pior default possível.
func TestSplitComCupom_FundedByDesconhecidoNaoCobraDaLoja(t *testing.T) {
	base, err := CalculateSplitRules(semCupom(), 10, 80)
	assert.NoError(t, err)

	for _, quem := range []string{"", "ninguem", "PLATFORM", "entregador"} {
		comCupom := &models.Payment{
			Amount: 95.0, DeliveryAmount: 10.0, CustomerID: 42,
			DiscountAmount: 5.0, DiscountFundedBy: quem,
		}
		res, err := CalculateSplitRules(comCupom, 10, 80)
		assert.NoError(t, err)
		assert.Equal(t, base.EstablishmentAmt, res.EstablishmentAmt,
			"funded_by=%q não pode acabar cobrando do restaurante", quem)
		assert.Equal(t, 95.0, somaDasPartes(res))
	}
}

// Desconto negativo inflaria o bruto e, com ele, a fatia percentual do
// estabelecimento acima do que o pedido pagou.
func TestSplitComCupom_DescontoNegativoNaoInflaOBruto(t *testing.T) {
	comCupom := &models.Payment{
		Amount: 100.0, DeliveryAmount: 10.0, CustomerID: 42,
		DiscountAmount: -50.0, DiscountFundedBy: "platform",
	}
	res, err := CalculateSplitRules(comCupom, 10, 80)
	assert.NoError(t, err)

	base, err := CalculateSplitRules(semCupom(), 10, 80)
	assert.NoError(t, err)
	assert.Equal(t, base.PlatformFee, res.PlatformFee)
	assert.Equal(t, base.EstablishmentAmt, res.EstablishmentAmt)
	assert.Equal(t, 100.0, somaDasPartes(res))
}

// Cobrança antiga (anterior à migração 23) tem desconto 0 e funded_by vazio: o
// split precisa dividir exatamente como sempre dividiu.
func TestSplitSemCupom_NaoMudaNada(t *testing.T) {
	res, err := CalculateSplitRules(semCupom(), 10, 80)
	assert.NoError(t, err)

	assert.Equal(t, 10.0, res.PlatformFee)
	assert.Equal(t, 80.0, res.EstablishmentAmt)
	assert.Equal(t, 10.0, res.DeliveryAmt)
	assert.Equal(t, 0.0, res.CustomerCredit)
	assert.Equal(t, 100.0, somaDasPartes(res))
}

// Percentual com centavo quebrado: o bruto arredonda antes de entrar nas
// fatias, e a soma continua fechando no centavo.
func TestSplitComCupom_CentavosFecham(t *testing.T) {
	comCupom := &models.Payment{
		Amount: 47.33, DeliveryAmount: 7.77, CustomerID: 42,
		DiscountAmount: 4.99, DiscountFundedBy: "establishment",
	}
	res, err := CalculateSplitRules(comCupom, 10, 80)
	assert.NoError(t, err)
	assert.Equal(t, 47.33, somaDasPartes(res),
		"soma das quatro partes tem de bater com o pago, no centavo")
}
