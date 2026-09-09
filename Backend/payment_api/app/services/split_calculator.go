package services

import (
	"fmt"
	"log"
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

	// O BRUTO é o pedido antes da promoção: o que o cliente pagou mais o
	// desconto do cupom. É sobre ele que as porcentagens incidem.
	//
	// Aplicar as porcentagens sobre o valor já descontado seria o erro
	// silencioso que este bloco existe para evitar: um cupom de R$10 num
	// pedido de R$100 (5% plataforma, 85% loja) tiraria R$0,50 da plataforma
	// e R$8,50 do restaurante — os dois pagando a promoção, em proporção,
	// independentemente de quem a ofereceu. Quem escolhe "descontar de mim"
	// na criação do cupom estaria escolhendo nada.
	discount := payment.DiscountAmount
	if discount < 0 {
		discount = 0
	}
	gross := roundCents(total + discount)

	// When delivery exceeds the payment total, zero out platform and
	// establishment shares — the delivery fee consumes the entire amount.
	// The caller (defaultSplitRules) expects a valid result, not an error.
	platformFee := roundCents(gross * (platformPct / 100.0))
	establishmentAmount := roundCents(gross * (establishmentPct / 100.0))

	// O desconto sai INTEIRO do lado que banca. As outras fatias ficam como
	// ficariam sem cupom nenhum — inclusive o cashback do cliente, que é o
	// resto e por isso não muda: a promoção não pode encolher o cashback de
	// quem usou o cupom.
	//
	// Se a fatia de quem banca não cobre o desconto, ela ZERA — quem banca dá
	// tudo o que tem antes de o outro lado perder um centavo. O que ainda
	// faltar depois disso é absorvido pelo cashback (o resto) e, se nem ele
	// bastar, pela cláusula `allocated > total` mais abaixo, que reduz o
	// estabelecimento. Isso não é escolha: o split só distribui o que o
	// cliente pagou, e um cupom acima da margem de quem o ofereceu não cria
	// dinheiro. É por isso que o caso é logado — significa cupom criado além
	// do que quem banca consegue bancar.
	if discount > 0 {
		switch payment.DiscountFundedBy {
		case "establishment":
			if discount > establishmentAmount {
				log.Printf("[SPLIT] pedido %s: cupom de %.2f acima da fatia do estabelecimento (%.2f) — a diferença sai das outras partes",
					payment.OrderID, discount, establishmentAmount)
			}
			establishmentAmount = roundCents(establishmentAmount - discount)
			if establishmentAmount < 0 {
				establishmentAmount = 0
			}
		default:
			if discount > platformFee {
				log.Printf("[SPLIT] pedido %s: cupom de %.2f acima da fatia da plataforma (%.2f) — a diferença sai das outras partes",
					payment.OrderID, discount, platformFee)
			}
			// "platform" e qualquer valor não reconhecido: a plataforma
			// absorve. É o mesmo default do cupom — quem oferece a promoção
			// sem dizer nada é quem a está oferecendo.
			platformFee = roundCents(platformFee - discount)
			if platformFee < 0 {
				platformFee = 0
			}
		}
	}

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
