// Package handlers - delivery_fee_test.go
//
// O frete é calculado pelo SERVIDOR, não aceito do corpo da requisição.
//
// Antes, computeOrderTotal recebia o `deliveryValue` que o cliente mandava e
// somava ao total. O total dos itens já era recalculado a partir dos preços do
// banco, mas o frete não — era o último valor do corpo que entrava no dinheiro
// sem conferência. Isso alimenta o split do pagamento: a cobrança confere o
// frete contra o pedido, mas se o valor gravado NO pedido veio do cliente, a
// conferência só empurra o problema um passo para trás.
package handlers

import (
	"errors"
	"testing"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupDeliveryFeeTestDB monta os dois DBs que o cálculo usa: o de pedidos
// (configuração de entrega + subscriptions) e o compartilhado com auth
// (produtos), que é de onde computeOrderTotal tira os preços.
func setupDeliveryFeeTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("abrir sqlite em memória: %v", err)
	}
	if err := db.AutoMigrate(&models.Delivery{}, &models.Product{}, &models.Additional{}); err != nil {
		t.Fatalf("migrar tabelas: %v", err)
	}
	// subscriptions vive no auth_api; aqui só precisa da forma que a query lê.
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS subscriptions (
		user_id INTEGER, status TEXT, plan TEXT,
		free_delivery_above REAL,
		current_period_start TEXT, current_period_end TEXT
	)`).Error; err != nil {
		t.Fatalf("criar subscriptions: %v", err)
	}

	prevOrders, prevAuth := models.DB, authModels.DB
	models.DB, authModels.DB = db, db
	t.Cleanup(func() { models.DB, authModels.DB = prevOrders, prevAuth })
}

// seedDelivery grava a configuração de frete do estabelecimento.
func seedDelivery(t *testing.T, establishmentID uint, fixedTaxa, perKm float32) {
	t.Helper()
	if err := models.DB.Create(&models.Delivery{
		EstablishmentID: establishmentID,
		FixedTaxa:       fixedTaxa,
		PerKm:           perKm,
	}).Error; err != nil {
		t.Fatalf("semear delivery: %v", err)
	}
}

func seedProduct(t *testing.T, id, establishmentID uint, price float64) {
	t.Helper()
	if err := models.DB.Create(&models.Product{
		ID: id, Name: "Produto", Price: price, EstablishmentID: establishmentID,
	}).Error; err != nil {
		t.Fatalf("semear produto: %v", err)
	}
}

// ── O cálculo em si ──

func TestComputeDeliveryFee_DistanciaVezesPerKmMaisFixa(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedDelivery(t, 1, 5.00, 2.00) // taxa fixa 5, R$2/km

	fee, err := computeDeliveryFee(3, 1, nil, 0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if fee.Value != 11.00 { // 3km * 2 + 5
		t.Fatalf("esperava frete 11.00 (3km × 2 + 5), veio %.2f", fee.Value)
	}
	if fee.SubscriptionDiscount {
		t.Error("sem usuário não deveria haver desconto de assinatura")
	}
}

// Os casos de ASSINATURA (premium/basic) não cabem aqui: a query de
// subscriptions usa cast `::text` do Postgres, que o sqlite rejeita com
// "unrecognized token". Testá-los neste harness daria falso verde — o Scan
// erraria, a função cairia no retorno sem desconto e a asserção "cobrou
// frete" passaria pelo motivo errado. Estão em delivery_fee_integration_test.go,
// contra Postgres de verdade.

// ── O ponto do teste: o corpo da requisição não manda no frete ──

func TestComputeOrderTotal_IgnoraFreteDoCliente(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedDelivery(t, 1, 5.00, 2.00)
	seedProduct(t, 100, 1, 30.00)

	cart := []dto.CartItem{{Item: dto.Item{ID: 100}, Quantity: 2}}

	total, frete, err := computeOrderTotal(cart, 3, 1, nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	// 2 × 30 = 60 de itens, frete 3×2+5 = 11 → 71.
	if frete != 11.00 {
		t.Fatalf("esperava frete calculado 11.00, veio %.2f", frete)
	}
	if total != 71.00 {
		t.Fatalf("esperava total 71.00 (60 + 11), veio %.2f", total)
	}
}

// Frete zero declarado pelo cliente não vira frete zero cobrado: o valor não
// entra mais na conta. (A assinatura é o ÚNICO caminho para frete grátis.)
func TestComputeOrderTotal_FreteNaoVemDoCorpo(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedDelivery(t, 1, 5.00, 2.00)
	seedProduct(t, 101, 1, 30.00)

	cart := []dto.CartItem{{Item: dto.Item{ID: 101}, Quantity: 1}}

	// A assinatura de computeOrderTotal não aceita mais frete: qualquer valor
	// que o cliente mandasse ficaria de fora por construção. O que este teste
	// trava é que a distância é que decide.
	_, fretePerto, err := computeOrderTotal(cart, 1, 1, nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	_, freteLonge, err := computeOrderTotal(cart, 10, 1, nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if fretePerto != 7.00 { // 1×2 + 5
		t.Fatalf("1km deveria custar 7.00, veio %.2f", fretePerto)
	}
	if freteLonge != 25.00 { // 10×2 + 5
		t.Fatalf("10km deveria custar 25.00, veio %.2f", freteLonge)
	}
	if freteLonge <= fretePerto {
		t.Fatal("o frete tem que crescer com a distância — sinal de que não está sendo calculado")
	}
}

func TestComputeOrderTotal_RejeitaDistanciaNegativa(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedDelivery(t, 1, 5.00, 2.00)
	seedProduct(t, 102, 1, 10.00)

	cart := []dto.CartItem{{Item: dto.Item{ID: 102}, Quantity: 1}}
	if _, _, err := computeOrderTotal(cart, -1, 1, nil); err == nil {
		t.Fatal("distância negativa deveria ser recusada")
	}
}

// ── Estabelecimento sem configuração de entrega ──
//
// Antes desta série, computeOrderTotal nem consultava `deliveries` — só somava
// o frete do corpo. Um estabelecimento que nunca chamou POST /delivery vendia
// normalmente. Se a ausência virasse erro, TODO pedido dele passaria a falhar:
// trocaria um problema de dinheiro por uma interrupção de venda.

func TestComputeDeliveryFee_SemConfigSinalizaErroProprio(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	// De propósito: nenhuma linha em deliveries.

	_, err := computeDeliveryFee(3, 42, nil, 0)
	if !errors.Is(err, errNoDeliveryConfig) {
		t.Fatalf("esperava errNoDeliveryConfig para poder distinguir de erro de banco, veio %v", err)
	}
}

func TestComputeOrderTotal_SemConfigNaoDerrubaOPedido(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedProduct(t, 200, 42, 25.00)
	// Sem seedDelivery para o estabelecimento 42.

	cart := []dto.CartItem{{Item: dto.Item{ID: 200}, Quantity: 2}}
	total, frete, err := computeOrderTotal(cart, 3, 42, nil)
	if err != nil {
		t.Fatalf("pedido não pode falhar por falta de configuração de entrega: %v", err)
	}
	if frete != 0 {
		t.Fatalf("sem configuração o frete é 0, veio %.2f", frete)
	}
	if total != 50.00 {
		t.Fatalf("esperava total 50.00 (2 × 25, sem frete), veio %.2f", total)
	}
}
