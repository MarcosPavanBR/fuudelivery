// Package handlers - order_coupon_test.go
//
// O cupom descontando de verdade.
//
// O buraco que estes testes fecham: o sistema de cupons existia inteiro —
// tabela, criação, validação, tela no app mostrando "Cupom válido!" — e não
// tocava em um centavo. Nenhum caminho de checkout chamava a validação, e
// `computeOrderTotal` somava produtos e frete sem olhar para cupom nenhum.
// Quem digitasse o código pagava o preço cheio.
package handlers

import (
	"errors"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
)

// seedCoupon grava um cupom vigente. Os campos que cada teste não usa ficam
// no default para o caso permanecer legível.
func seedCoupon(t *testing.T, c models.Coupon) models.Coupon {
	t.Helper()
	if c.StartDate.IsZero() {
		c.StartDate = time.Now().Add(-time.Hour)
	}
	if c.ExpiryDate.IsZero() {
		c.ExpiryDate = time.Now().Add(24 * time.Hour)
	}
	if c.FundedBy == "" {
		c.FundedBy = models.CouponFundedByPlatform
	}
	c.IsActive = true
	if err := models.DB.Create(&c).Error; err != nil {
		t.Fatalf("semear cupom: %v", err)
	}
	return c
}

// setupCouponOrderDB reaproveita o harness do frete (dois DBs) e acrescenta as
// tabelas de cupom.
func setupCouponOrderDB(t *testing.T) {
	t.Helper()
	setupDeliveryFeeTestDB(t)
	if err := models.DB.AutoMigrate(&models.Coupon{}, &models.CouponUsage{}); err != nil {
		t.Fatalf("migrar cupons: %v", err)
	}
}

// ── O desconto sai do total ──

func TestApplyCoupon_PercentualSaiDoTotal(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "DEZ", DiscountType: "PERCENTAGE", DiscountValue: 10,
		EstablishmentID: 1, FundedBy: models.CouponFundedByPlatform,
	})

	// Produtos 60,00 + frete 11,00. 10% incide sobre os PRODUTOS: 6,00.
	app, err := applyCouponToOrder("DEZ", "ped-1", "+5511999900001", 1, 60.00, 11.00)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if app.Discount != 6.00 {
		t.Fatalf("esperava desconto 6.00 (10%% de 60), veio %.2f", app.Discount)
	}
	if app.FundedBy != models.CouponFundedByPlatform {
		t.Fatalf("esperava funded_by=platform, veio %q", app.FundedBy)
	}
}

// O percentual incide sobre os produtos, não sobre produtos+frete. Se
// incidisse sobre o frete, o desconto de 10%% num pedido com frete alto sairia
// maior do que a promoção anunciada — e o entregador continuaria recebendo o
// frete cheio, então a diferença viria do bolso de quem banca sem escolha.
func TestApplyCoupon_PercentualNaoIncideSobreOFrete(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "DEZ", DiscountType: "PERCENTAGE", DiscountValue: 10, EstablishmentID: 1,
	})

	app, err := applyCouponToOrder("DEZ", "ped-1", "+5511999900001", 1, 100.00, 50.00)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if app.Discount != 10.00 {
		t.Fatalf("10%% de 100 de produtos = 10.00 (não 15.00 com frete), veio %.2f", app.Discount)
	}
}

// FREE_DELIVERY desconta EXATAMENTE o frete. ValidateCouponInternal devolve 0
// para este tipo porque não conhece o frete — sem o tratamento aqui, o cupom
// de frete grátis não descontaria nada e o cliente pagaria a entrega assim
// mesmo.
func TestApplyCoupon_FreteGratisDescontaOFrete(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "FRETEGRATIS", DiscountType: "FREE_DELIVERY", DiscountValue: 0, EstablishmentID: 1,
	})

	app, err := applyCouponToOrder("FRETEGRATIS", "ped-1", "+5511999900001", 1, 60.00, 11.00)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if app.Discount != 11.00 {
		t.Fatalf("frete grátis desconta o frete (11.00), veio %.2f", app.Discount)
	}
}

// Retirada no balcão: frete zero, cupom de frete grátis não desconta nada — e
// por isso não pode gastar um uso do cliente.
func TestApplyCoupon_FreteGratisSemFreteNaoGastaUso(t *testing.T) {
	setupCouponOrderDB(t)
	c := seedCoupon(t, models.Coupon{
		Code: "FRETEGRATIS", DiscountType: "FREE_DELIVERY", EstablishmentID: 1, MaxUses: 1,
	})

	app, err := applyCouponToOrder("FRETEGRATIS", "ped-1", "+5511999900001", 1, 60.00, 0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if app.Discount != 0 || app.Code != "" {
		t.Fatalf("sem frete não há desconto, veio %+v", app)
	}
	var depois models.Coupon
	models.DB.First(&depois, c.ID)
	if depois.UsedCount != 0 {
		t.Fatalf("cupom que não descontou nada não pode gastar uso, used_count=%d", depois.UsedCount)
	}
}

// FIXED maior que o pedido para no valor dos PRODUTOS: o entregador recebe o
// frete de qualquer jeito, então deixar o desconto comer a entrega tiraria
// dinheiro de quem banca o cupom além do que foi prometido.
func TestApplyCoupon_FixoMaiorQueOPedidoParaNosProdutos(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "VINTE", DiscountType: "FIXED", DiscountValue: 20, EstablishmentID: 1,
	})

	app, err := applyCouponToOrder("VINTE", "ped-1", "+5511999900001", 1, 15.00, 11.00)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if app.Discount != 15.00 {
		t.Fatalf("desconto para no subtotal (15.00), veio %.2f", app.Discount)
	}
}

// ── Limites: um uso é um uso ──

func TestApplyCoupon_UsoUnicoNaoRendeDois(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "UNICO", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, MaxUses: 1,
	})

	if _, err := applyCouponToOrder("UNICO", "ped-1", "+5511999900001", 1, 60, 11); err != nil {
		t.Fatalf("primeiro uso deveria valer: %v", err)
	}
	// Segundo pedido, outro cliente: o cupom acabou.
	if _, err := applyCouponToOrder("UNICO", "ped-2", "+5511999900002", 1, 60, 11); err == nil {
		t.Fatal("cupom de uso único não pode ser aplicado duas vezes")
	}
}

// O caso acima é barrado já na validação (used_count >= max_uses), que é uma
// LEITURA. Este aqui ataca o consumo direto, que é onde a corrida acontece:
// dois checkouts que passaram pela validação no mesmo instante, cada um com
// sua cópia do cupom em memória dizendo used_count=0. Sem o UPDATE condicional
// os dois incrementariam e o cupom de uso único renderia dois descontos.
func TestConsumeCoupon_DoisConsumosSimultaneosSoUmPassa(t *testing.T) {
	setupCouponOrderDB(t)
	c := seedCoupon(t, models.Coupon{
		Code: "UNICO", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, MaxUses: 1,
	})

	// As duas cópias que os dois checkouts teriam lido antes de qualquer
	// escrita: ambas com used_count = 0.
	a, b := c, c

	if err := consumeCoupon(&a, "ped-1", "+5511999900001", 10); err != nil {
		t.Fatalf("o primeiro consumo deveria passar: %v", err)
	}
	err := consumeCoupon(&b, "ped-2", "+5511999900002", 10)
	if !errors.Is(err, errCouponUnavailable) {
		t.Fatalf("o segundo consumo deveria ser recusado com errCouponUnavailable, veio %v", err)
	}

	var depois models.Coupon
	models.DB.First(&depois, c.ID)
	if depois.UsedCount != 1 {
		t.Fatalf("cupom de uso único não pode passar de 1, used_count=%d", depois.UsedCount)
	}
	var usos int64
	models.DB.Model(&models.CouponUsage{}).Where("coupon_id = ?", c.ID).Count(&usos)
	if usos != 1 {
		t.Fatalf("deveria haver exatamente 1 uso registrado, há %d", usos)
	}
}

// Cupom desativado no meio do caminho também para no consumo.
func TestConsumeCoupon_DesativadoNaoConsome(t *testing.T) {
	setupCouponOrderDB(t)
	c := seedCoupon(t, models.Coupon{
		Code: "DESATIVADO", DiscountType: "FIXED", DiscountValue: 10, EstablishmentID: 1,
	})
	models.DB.Model(&models.Coupon{}).Where("id = ?", c.ID).UpdateColumn("is_active", false)

	if err := consumeCoupon(&c, "ped-1", "+5511999900001", 10); !errors.Is(err, errCouponUnavailable) {
		t.Fatalf("cupom desativado não consome, veio %v", err)
	}
}

// O uso fica amarrado ao pedido — é o que permite auditar (e estornar) qual
// pedido gastou qual cupom.
func TestApplyCoupon_RegistraUsoNoPedido(t *testing.T) {
	setupCouponOrderDB(t)
	c := seedCoupon(t, models.Coupon{
		Code: "DEZ", DiscountType: "FIXED", DiscountValue: 10, EstablishmentID: 1,
	})

	if _, err := applyCouponToOrder("DEZ", "ped-42", "+5511999900001", 1, 60, 11); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	var uso models.CouponUsage
	if err := models.DB.Where("order_id = ?", "ped-42").First(&uso).Error; err != nil {
		t.Fatalf("uso do cupom não foi registrado: %v", err)
	}
	if uso.CouponID != c.ID || uso.DiscountAmount != 10.00 {
		t.Fatalf("uso registrado errado: %+v", uso)
	}
	var depois models.Coupon
	models.DB.First(&depois, c.ID)
	if depois.UsedCount != 1 {
		t.Fatalf("used_count deveria ser 1, veio %d", depois.UsedCount)
	}
}

// releaseCoupon devolve o uso quando o pedido não chegou a ser gravado. Sem
// isto, uma falha de escrita queimaria o cupom do cliente sem lhe dar pedido
// nenhum.
func TestReleaseCoupon_DevolveOUsoQuandoOPedidoNaoExiste(t *testing.T) {
	setupCouponOrderDB(t)
	c := seedCoupon(t, models.Coupon{
		Code: "DEZ", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, MaxUses: 1,
	})

	if _, err := applyCouponToOrder("DEZ", "ped-1", "+5511999900001", 1, 60, 11); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	releaseCoupon("DEZ", "ped-1")

	var depois models.Coupon
	models.DB.First(&depois, c.ID)
	if depois.UsedCount != 0 {
		t.Fatalf("uso deveria ter voltado, used_count=%d", depois.UsedCount)
	}
	var n int64
	models.DB.Model(&models.CouponUsage{}).Where("order_id = ?", "ped-1").Count(&n)
	if n != 0 {
		t.Fatalf("registro de uso deveria ter sido apagado, há %d", n)
	}
	// E o cupom volta a valer para um pedido de verdade.
	if _, err := applyCouponToOrder("DEZ", "ped-2", "+5511999900001", 1, 60, 11); err != nil {
		t.Fatalf("cupom devolvido deveria voltar a valer: %v", err)
	}
}

// ── Identidade: o telefone é do token, não do corpo ──

// Cupom de indicação é pessoal. O telefone que decide isso vem do token; este
// teste prova que outro telefone não resgata.
func TestApplyCoupon_PessoalSoDoDono(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "INDICA", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, OwnerPhone: "+5511999900001",
	})

	if _, err := applyCouponToOrder("INDICA", "ped-1", "+5511999900002", 1, 60, 11); err == nil {
		t.Fatal("cupom pessoal não pode ser resgatado por outro telefone")
	}
	if _, err := applyCouponToOrder("INDICA", "ped-2", "+5511999900001", 1, 60, 11); err != nil {
		t.Fatalf("o dono deveria conseguir resgatar: %v", err)
	}
}

// Token sem telefone não resgata cupom pessoal.
//
// ValidateCouponInternal pula a checagem de dono quando o telefone está vazio
// (`coupon.OwnerPhone != "" && req.UserPhone != ""`), então sem a guarda
// explícita bastava um token sem o claim de telefone para pegar o cupom de
// indicação de qualquer pessoa.
func TestApplyCoupon_SemTelefoneNaoPegaCupomPessoal(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "INDICA", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, OwnerPhone: "+5511999900001",
	})

	if _, err := applyCouponToOrder("INDICA", "ped-1", "", 1, 60, 11); err == nil {
		t.Fatal("token sem telefone não pode resgatar cupom pessoal")
	}
}

// ── Escopo e vigência continuam valendo no checkout ──

func TestApplyCoupon_DeOutroEstabelecimentoNaoVale(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "DAOUTRA", DiscountType: "FIXED", DiscountValue: 10, EstablishmentID: 2,
	})

	if _, err := applyCouponToOrder("DAOUTRA", "ped-1", "+5511999900001", 1, 60, 11); err == nil {
		t.Fatal("cupom de outro estabelecimento não pode descontar aqui")
	}
}

func TestApplyCoupon_ExpiradoNaoVale(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "VELHO", DiscountType: "FIXED", DiscountValue: 10, EstablishmentID: 1,
		StartDate:  time.Now().Add(-48 * time.Hour),
		ExpiryDate: time.Now().Add(-time.Hour),
	})

	if _, err := applyCouponToOrder("VELHO", "ped-1", "+5511999900001", 1, 60, 11); err == nil {
		t.Fatal("cupom expirado não pode descontar")
	}
}

func TestApplyCoupon_AbaixoDoMinimoNaoVale(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "MIN50", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, MinOrderValue: 50,
	})

	if _, err := applyCouponToOrder("MIN50", "ped-1", "+5511999900001", 1, 30, 11); err == nil {
		t.Fatal("abaixo do mínimo o cupom não vale")
	}
	if _, err := applyCouponToOrder("MIN50", "ped-2", "+5511999900001", 1, 50, 11); err != nil {
		t.Fatalf("no mínimo exato deveria valer: %v", err)
	}
}

// Código inexistente é recusado, não ignorado: seguir em frente cobraria o
// preço cheio de quem viu o desconto na tela.
func TestApplyCoupon_InexistenteEhRecusado(t *testing.T) {
	setupCouponOrderDB(t)

	if _, err := applyCouponToOrder("NAOEXISTE", "ped-1", "+5511999900001", 1, 60, 11); err == nil {
		t.Fatal("cupom inexistente deveria ser recusado")
	}
}

// Pedido sem cupom continua funcionando exatamente como antes.
func TestApplyCoupon_SemCodigoNaoFazNada(t *testing.T) {
	setupCouponOrderDB(t)

	app, err := applyCouponToOrder("", "ped-1", "+5511999900001", 1, 60, 11)
	if err != nil {
		t.Fatalf("pedido sem cupom não pode falhar: %v", err)
	}
	if app.Discount != 0 || app.Code != "" || app.FundedBy != "" {
		t.Fatalf("sem cupom não há aplicação, veio %+v", app)
	}
}

// O código é normalizado: quem digita minúscula com espaço resgata o mesmo
// cupom.
func TestApplyCoupon_CodigoNormalizado(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "DEZ", DiscountType: "FIXED", DiscountValue: 10, EstablishmentID: 1,
	})

	app, err := applyCouponToOrder("  dez ", "ped-1", "+5511999900001", 1, 60, 11)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if app.Code != "DEZ" || app.Discount != 10.00 {
		t.Fatalf("esperava DEZ com 10.00, veio %+v", app)
	}
}

// ── Quem banca chega até o pedido ──

// É o dado que o split precisa: sem ele o desconto sai diluído entre
// plataforma e restaurante, e não de quem ofereceu a promoção.
func TestApplyCoupon_FundedByDoEstabelecimentoChegaNaAplicacao(t *testing.T) {
	setupCouponOrderDB(t)
	seedCoupon(t, models.Coupon{
		Code: "DALOJA", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, FundedBy: models.CouponFundedByEstablishment,
	})

	app, err := applyCouponToOrder("DALOJA", "ped-1", "+5511999900001", 1, 60, 11)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if app.FundedBy != models.CouponFundedByEstablishment {
		t.Fatalf("esperava funded_by=establishment, veio %q", app.FundedBy)
	}
}

// Cupom gravado antes da migração 22 (funded_by vazio numa base sem backfill)
// não pode chegar ao split com o campo em branco — o split não saberia de qual
// lado subtrair.
func TestApplyCoupon_FundedByVazioViraPlataforma(t *testing.T) {
	setupCouponOrderDB(t)
	c := seedCoupon(t, models.Coupon{
		Code: "ANTIGO", DiscountType: "FIXED", DiscountValue: 10, EstablishmentID: 1,
	})
	models.DB.Model(&models.Coupon{}).Where("id = ?", c.ID).UpdateColumn("funded_by", "")

	app, err := applyCouponToOrder("ANTIGO", "ped-1", "+5511999900001", 1, 60, 11)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if app.FundedBy != models.CouponFundedByPlatform {
		t.Fatalf("cupom sem funded_by é da plataforma, veio %q", app.FundedBy)
	}
}
