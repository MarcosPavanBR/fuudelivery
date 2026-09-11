package handlers

// order_coupon.go — o cupom entrando no dinheiro do pedido.
//
// Até aqui o cupom era decoração. Existia a tabela, existiam os handlers de
// criar/validar/aplicar, e o app até mostrava "Cupom válido!" — mas nenhum
// caminho de checkout chamava nada disso: `computeOrderTotal` somava produtos
// e frete e pronto. Quem digitasse um cupom pagava o preço cheio.
//
// Este arquivo fecha isso, com três regras que vêm de onde o dinheiro pode
// vazar:
//
//  1. O código do cupom é a ÚNICA coisa que vem do cliente. Valor do desconto,
//     tipo, vigência e limites saem do banco — como já acontece com o preço do
//     produto e com o frete.
//  2. O TELEFONE sai do token, não do corpo. É ele que decide se um cupom
//     pessoal (indicação) é seu e se você já bateu o limite por usuário;
//     lendo do corpo, qualquer um resgatava o cupom de indicação alheio.
//  3. O consumo é um UPDATE CONDICIONAL, não um "leu e depois escreveu". Dois
//     pedidos simultâneos com o último uso de um cupom de uso único: só um
//     incrementa, o outro é recusado.

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"gorm.io/gorm"
)

// couponApplication é o que o cupom fez com o pedido.
type couponApplication struct {
	// Code é o código efetivamente consumido (vazio quando não houve cupom).
	Code string
	// Discount é quanto saiu do total, em reais, já limitado.
	Discount float64
	// FundedBy diz de quem sai — é o que o split usa depois para subtrair do
	// lado certo em vez de diluir o desconto entre plataforma e restaurante.
	FundedBy string
}

// errCouponUnavailable é o cupom que existia na validação e não existia mais
// no consumo: acabou o estoque de usos entre uma coisa e outra.
var errCouponUnavailable = errors.New("cupom não está mais disponível")

// roundReais fixa o valor em centavos. Sem isto o desconto percentual entra no
// total com a cauda binária do float e o pedido fecha em R$47,299999999999997,
// que depois vira um centavo de divergência contra a cobrança em payment_api.
func roundReais(v float64) float64 { return math.Round(v*100) / 100 }

// applyCouponToOrder valida e CONSOME o cupom para este pedido.
//
// subtotal é o valor dos produtos e delivery é o frete já calculado pelo
// servidor; os dois entram porque as regras dependem deles: o mínimo de compra
// e o desconto percentual olham para os produtos, e FREE_DELIVERY desconta
// exatamente o frete.
//
// Devolve erro quando o cupom foi digitado e não vale. Recusar é deliberado:
// seguir em frente cobrando o preço cheio faria o cliente pagar mais do que a
// tela dele mostrava.
func applyCouponToOrder(code, orderID, tokenPhone string, establishmentID int64, subtotal, delivery float64) (couponApplication, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return couponApplication{}, nil
	}
	if models.DB == nil {
		return couponApplication{}, fmt.Errorf("banco indisponível para validar o cupom")
	}

	var coupon models.Coupon
	if err := models.DB.Where("code = ?", code).First(&coupon).Error; err != nil {
		return couponApplication{}, fmt.Errorf("cupom não encontrado")
	}

	// Cupom pessoal ou com limite por usuário exige saber QUEM é o usuário.
	//
	// ValidateCouponInternal pula as duas checagens quando o telefone está
	// vazio (`coupon.OwnerPhone != "" && req.UserPhone != ""`), então sem esta
	// guarda um token sem telefone resgataria o cupom de indicação de
	// qualquer pessoa — bastava não ter telefone no claim.
	if tokenPhone == "" && (coupon.OwnerPhone != "" || coupon.MaxUsesPerUser > 0) {
		return couponApplication{}, fmt.Errorf("cupom exige usuário identificado")
	}

	// Reusa a validação existente como fonte única das regras (vigência,
	// limite global, limite por usuário, mínimo de compra, escopo de
	// estabelecimento, dono do cupom).
	res := ValidateCouponInternal(dto.ValidateCouponRequest{
		Code:            code,
		UserPhone:       tokenPhone,
		OrderValue:      subtotal,
		EstablishmentID: uint(establishmentID),
	})
	if !res.Valid {
		return couponApplication{}, errors.New(strings.ToLower(res.Message))
	}

	discount := res.DiscountAmount
	if coupon.DiscountType == "FREE_DELIVERY" {
		// ValidateCouponInternal devolve 0 para FREE_DELIVERY porque não
		// conhece o frete. Aqui conhecemos: o desconto É o frete.
		discount = delivery
	} else if discount > subtotal {
		// Um FIXED de R$20 num pedido de R$15 não pode comer o frete: o
		// entregador recebe o frete de qualquer jeito, e a diferença sairia
		// do bolso de quem banca o cupom sem que ninguém tivesse escolhido
		// isso. O desconto para no valor dos produtos.
		discount = subtotal
	}
	discount = roundReais(discount)
	if discount <= 0 {
		// Cupom válido que não desconta nada (FREE_DELIVERY em retirada no
		// balcão, por exemplo): não gasta um uso do cliente à toa.
		return couponApplication{}, nil
	}

	fundedBy := coupon.FundedBy
	if fundedBy == "" {
		// Cupom anterior à migração 22, lido de uma base ainda sem backfill.
		// O mesmo padrão da migração: quem oferece sem dizer é a plataforma.
		fundedBy = models.CouponFundedByPlatform
	}

	if err := consumeCoupon(&coupon, orderID, tokenPhone, discount); err != nil {
		return couponApplication{}, err
	}

	return couponApplication{Code: code, Discount: discount, FundedBy: fundedBy}, nil
}

// consumeCoupon gasta um uso do cupom e registra o uso, numa transação.
//
// O incremento é um UPDATE CONDICIONAL (`used_count < max_uses` no WHERE) em
// vez de ler-somar-gravar: é o banco que decide quem pegou o último uso. Dois
// checkouts simultâneos com o mesmo cupom de uso único fazem exatamente um
// UPDATE afetar linha; o outro recebe errCouponUnavailable e o pedido é
// recusado em vez de gerar dois descontos com um uso só.
//
// Resíduo conhecido e assumido: o limite POR USUÁRIO ainda é verificado por
// leitura (em ValidateCouponInternal), então dois pedidos disparados no mesmo
// instante pelo mesmo telefone podem passar dos dois. O teto global — que é o
// que limita o prejuízo total de uma promoção — está fechado aqui.
func consumeCoupon(coupon *models.Coupon, orderID, userPhone string, discount float64) error {
	return models.DB.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&models.Coupon{}).
			Where("id = ? AND is_active = ?", coupon.ID, true)
		if coupon.MaxUses > 0 {
			q = q.Where("used_count < ?", coupon.MaxUses)
		}
		upd := q.UpdateColumn("used_count", gorm.Expr("used_count + ?", 1))
		if upd.Error != nil {
			return upd.Error
		}
		if upd.RowsAffected == 0 {
			return errCouponUnavailable
		}

		usage := models.CouponUsage{
			CouponID:       coupon.ID,
			UserPhone:      userPhone,
			OrderID:        orderID,
			DiscountAmount: discount,
			UsedAt:         time.Now(),
		}
		if err := tx.Create(&usage).Error; err != nil {
			return err
		}
		return nil
	})
}

// releaseCoupon devolve o uso gasto quando o pedido não chegou a existir.
//
// O cupom é consumido ANTES de gravar o pedido de propósito: se a gravação
// falhasse depois de um consumo não registrado, o cliente teria o desconto e o
// cupom continuaria com uso sobrando — dinheiro. Na ordem inversa o pior caso
// é um uso queimado sem pedido, que é o que esta função desfaz.
func releaseCoupon(couponCode, orderID string) {
	if couponCode == "" || models.DB == nil {
		return
	}
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		var coupon models.Coupon
		if err := tx.Where("code = ?", couponCode).First(&coupon).Error; err != nil {
			return err
		}
		// O decremento é condicionado ao DELETE ter apagado ALGO: devolver uso
		// que não existe infla o used_count para baixo e vira uso grátis — o
		// espelho exato do bug do consumo duplo. Chamar releaseCoupon duas
		// vezes (ou para um order_id que nunca consumiu) vira no-op.
		del := tx.Where("coupon_id = ? AND order_id = ?", coupon.ID, orderID).
			Delete(&models.CouponUsage{})
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&models.Coupon{}).
			Where("id = ? AND used_count > 0", coupon.ID).
			UpdateColumn("used_count", gorm.Expr("used_count - ?", 1)).Error
	})
	if err != nil {
		// Não derruba a resposta: o pedido já falhou por outro motivo e o
		// cliente precisa saber disso, não do estorno do cupom. O log é o que
		// permite devolver o uso na mão.
		log.Printf("[ORDER] cupom %s consumido no pedido %s que não foi gravado — devolver o uso manualmente: %v",
			couponCode, orderID, err)
	}
}
