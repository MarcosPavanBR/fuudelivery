// Package handlers - create_order_coupon_test.go
//
// O cupom no CHECKOUT inteiro, não só na função isolada.
//
// Este é o teste que fecha o buraco de verdade. `applyCouponToOrder` pode
// calcular o desconto certinho e o pedido ainda assim ser gravado com o total
// cheio — foi exatamente essa a situação por muito tempo: a validação de cupom
// existia, funcionava, e ninguém a chamava no caminho do dinheiro. Aqui o
// pedido entra pelo handler e sai do banco, e a asserção é sobre o que ficou
// gravado em order_documents — que é o que payment_api cobra.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const createOrderSecret = "create-order-coupon-secret"

// setupCreateOrder monta o checkout completo: produtos, configuração de frete,
// estabelecimento aberto, cupons e a tabela de pedidos.
func setupCreateOrder(t *testing.T) *fiber.App {
	t.Helper()
	setupCouponOrderDB(t)

	if err := models.DB.AutoMigrate(&models.OrderDocument{}); err != nil {
		t.Fatalf("migrar order_documents: %v", err)
	}
	if err := authModels.DB.AutoMigrate(&authModels.Establishment{}); err != nil {
		t.Fatalf("migrar establishments: %v", err)
	}

	// OpenData não-nulo é o que checkEstablishmentOpen lê como "aberto".
	aberto := time.Now().Format(time.RFC3339)
	if err := authModels.DB.Create(&authModels.Establishment{
		ID: 1, Name: "Restaurante", OpenData: &aberto,
	}).Error; err != nil {
		t.Fatalf("semear estabelecimento: %v", err)
	}

	seedDelivery(t, 1, 5.00, 2.00) // frete = 5 + 2/km
	seedProduct(t, 100, 1, 30.00)

	t.Setenv("JWT_SECRET", createOrderSecret)

	app := fiber.New()
	app.Post("/orders", func(c *fiber.Ctx) error {
		return CreateOrder(c, func(clientID int64, message []byte) error { return nil })
	})
	return app
}

func tokenComTelefone(t *testing.T, phone string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"id": 1, "role": "client", "phone": phone,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(createOrderSecret))
	if err != nil {
		t.Fatalf("assinar token: %v", err)
	}
	return s
}

// corpoDoPedido: 2 produtos de 30 (subtotal 60), 3 km (frete 5 + 3×2 = 11).
// `extra` entra cru para poder mandar campo que o cliente não deveria mandar.
func corpoDoPedido(extra string) string {
	base := `"cart":[{"item":{"id":100},"quantity":2}],"distance":3,"establishmentId":1,
		"user":{"phone":"+5511999900001"},"deliveryValue":11`
	if extra != "" {
		base += "," + extra
	}
	return "{" + base + "}"
}

func postPedido(t *testing.T, app *fiber.App, token, body string) (*http.Response, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", "/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

// pedidoGravado devolve o payload como o banco guardou — a fonte que
// payment_api lê para cobrar.
func pedidoGravado(t *testing.T, orderID string) dto.RequestPayload {
	t.Helper()
	var doc models.OrderDocument
	if err := models.DB.Where("legacy_id = ?", orderID).First(&doc).Error; err != nil {
		t.Fatalf("pedido %s não foi gravado: %v", orderID, err)
	}
	var p dto.RequestPayload
	if err := json.Unmarshal(doc.Payload, &p); err != nil {
		t.Fatalf("desserializar payload: %v", err)
	}
	return p
}

// ── O caso central ──

// Mesmo pedido, com e sem cupom: a diferença no total gravado é exatamente o
// desconto. Sem a aplicação no checkout os dois totais seriam iguais a 71,00 e
// o cupom não teria feito nada.
func TestCreateOrder_CupomDescontaDoTotalGravado(t *testing.T) {
	app := setupCreateOrder(t)
	seedCoupon(t, models.Coupon{
		Code: "DEZ", DiscountType: "PERCENTAGE", DiscountValue: 10,
		EstablishmentID: 1, FundedBy: models.CouponFundedByPlatform,
	})
	token := tokenComTelefone(t, "+5511999900001")

	// Sem cupom: 2 × 30 + frete 11 = 71,00.
	_, semCupom := postPedido(t, app, token, corpoDoPedido(""))
	pSem := pedidoGravado(t, semCupom["orderId"].(string))
	if pSem.OrderTotal != 71.00 {
		t.Fatalf("pedido sem cupom deveria fechar em 71.00, veio %.2f", pSem.OrderTotal)
	}
	if pSem.DiscountAmount != 0 || pSem.CouponCode != "" {
		t.Fatalf("pedido sem cupom não carrega desconto: %+v", pSem)
	}

	// Com cupom de 10% sobre os produtos: 71,00 − 6,00 = 65,00.
	resp, comCupom := postPedido(t, app, token, corpoDoPedido(`"coupon_code":"DEZ"`))
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("esperava 200, veio %d (%v)", resp.StatusCode, comCupom)
	}
	pCom := pedidoGravado(t, comCupom["orderId"].(string))
	if pCom.OrderTotal != 65.00 {
		t.Fatalf("com o cupom o pedido deveria fechar em 65.00, veio %.2f", pCom.OrderTotal)
	}
	if pCom.DiscountAmount != 6.00 {
		t.Fatalf("desconto gravado deveria ser 6.00, veio %.2f", pCom.DiscountAmount)
	}
	if pCom.CouponCode != "DEZ" {
		t.Fatalf("código gravado deveria ser DEZ, veio %q", pCom.CouponCode)
	}
	if pCom.DiscountFundedBy != models.CouponFundedByPlatform {
		t.Fatalf("quem banca deveria estar gravado como platform, veio %q", pCom.DiscountFundedBy)
	}
	// O frete NÃO muda: o entregador recebe igual, o desconto sai de quem banca.
	if pCom.DeliveryValue != 11.00 {
		t.Fatalf("o frete não muda com o cupom, veio %.2f", pCom.DeliveryValue)
	}
}

// Quem banca chega ao pedido com o valor do cupom, não com um padrão fixo — é
// o que o split lê para subtrair do lado certo.
func TestCreateOrder_FundedByDoEstabelecimentoChegaAoPedido(t *testing.T) {
	app := setupCreateOrder(t)
	seedCoupon(t, models.Coupon{
		Code: "DALOJA", DiscountType: "FIXED", DiscountValue: 8,
		EstablishmentID: 1, FundedBy: models.CouponFundedByEstablishment,
	})

	_, out := postPedido(t, app, tokenComTelefone(t, "+5511999900001"),
		corpoDoPedido(`"coupon_code":"DALOJA"`))
	p := pedidoGravado(t, out["orderId"].(string))

	if p.DiscountFundedBy != models.CouponFundedByEstablishment {
		t.Fatalf("esperava establishment gravado, veio %q", p.DiscountFundedBy)
	}
	if p.OrderTotal != 63.00 {
		t.Fatalf("71.00 − 8.00 = 63.00, veio %.2f", p.OrderTotal)
	}
}

// ── O cliente só manda o código ──

// Desconto mandado no corpo é ignorado. Sem isto o campo seria um "quanto eu
// quero pagar" — o mesmo furo que o total e o frete já tinham.
func TestCreateOrder_DescontoDoCorpoEhIgnorado(t *testing.T) {
	app := setupCreateOrder(t)

	_, out := postPedido(t, app, tokenComTelefone(t, "+5511999900001"),
		corpoDoPedido(`"discount_amount":70,"discount_funded_by":"establishment"`))
	p := pedidoGravado(t, out["orderId"].(string))

	if p.DiscountAmount != 0 {
		t.Fatalf("desconto do corpo deveria ser ignorado, veio %.2f", p.DiscountAmount)
	}
	if p.DiscountFundedBy != "" {
		t.Fatalf("funded_by do corpo deveria ser ignorado, veio %q", p.DiscountFundedBy)
	}
	if p.OrderTotal != 71.00 {
		t.Fatalf("sem cupom o total é 71.00, veio %.2f", p.OrderTotal)
	}
}

// Cupom real mais desconto inventado no corpo: vale o do cupom.
func TestCreateOrder_DescontoDoCorpoNaoSomaAoDoCupom(t *testing.T) {
	app := setupCreateOrder(t)
	seedCoupon(t, models.Coupon{
		Code: "DEZ", DiscountType: "FIXED", DiscountValue: 10, EstablishmentID: 1,
	})

	_, out := postPedido(t, app, tokenComTelefone(t, "+5511999900001"),
		corpoDoPedido(`"coupon_code":"DEZ","discount_amount":70`))
	p := pedidoGravado(t, out["orderId"].(string))

	if p.DiscountAmount != 10.00 || p.OrderTotal != 61.00 {
		t.Fatalf("vale o desconto do cupom (10.00 → 61.00), veio desconto=%.2f total=%.2f",
			p.DiscountAmount, p.OrderTotal)
	}
}

// O telefone do CORPO não resgata cupom pessoal — quem decide é o token. Este
// é o caso do cupom de indicação: sem a regra, bastava mandar o telefone de
// outra pessoa no corpo do pedido.
func TestCreateOrder_TelefoneDoCorpoNaoResgataCupomPessoal(t *testing.T) {
	app := setupCreateOrder(t)
	seedCoupon(t, models.Coupon{
		Code: "INDICA", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, OwnerPhone: "+5511999900001",
	})

	// Token de OUTRA pessoa, corpo alegando ser o dono do cupom.
	corpo := `{"cart":[{"item":{"id":100},"quantity":2}],"distance":3,"establishmentId":1,
		"user":{"phone":"+5511999900001"},"coupon_code":"INDICA"}`
	resp, out := postPedido(t, app, tokenComTelefone(t, "+5511999900002"), corpo)

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("cupom pessoal de outro deveria dar 400, veio %d (%v)", resp.StatusCode, out)
	}
}

// ── Cupom inválido recusa o pedido, em vez de cobrar o preço cheio ──

func TestCreateOrder_CupomInvalidoRecusaOPedido(t *testing.T) {
	app := setupCreateOrder(t)

	resp, out := postPedido(t, app, tokenComTelefone(t, "+5511999900001"),
		corpoDoPedido(`"coupon_code":"NAOEXISTE"`))

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("esperava 400 para cupom inexistente, veio %d", resp.StatusCode)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "cupom") {
		t.Fatalf("o erro deveria falar do cupom, veio %q", msg)
	}
	var n int64
	models.DB.Model(&models.OrderDocument{}).Count(&n)
	if n != 0 {
		t.Fatalf("nenhum pedido deveria ter sido criado, há %d", n)
	}
}

// ── O uso é gasto uma vez por pedido ──

func TestCreateOrder_CupomDeUsoUnicoNaoDescontaEmDoisPedidos(t *testing.T) {
	app := setupCreateOrder(t)
	c := seedCoupon(t, models.Coupon{
		Code: "UNICO", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, MaxUses: 1,
	})
	token := tokenComTelefone(t, "+5511999900001")

	resp1, out1 := postPedido(t, app, token, corpoDoPedido(`"coupon_code":"UNICO"`))
	if resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("primeiro pedido deveria passar, veio %d (%v)", resp1.StatusCode, out1)
	}
	resp2, _ := postPedido(t, app, token, corpoDoPedido(`"coupon_code":"UNICO"`))
	if resp2.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("segundo pedido com cupom esgotado deveria dar 400, veio %d", resp2.StatusCode)
	}

	var depois models.Coupon
	models.DB.First(&depois, c.ID)
	if depois.UsedCount != 1 {
		t.Fatalf("o cupom deveria ter sido gasto exatamente uma vez, used_count=%d", depois.UsedCount)
	}
	// E o uso aponta para o pedido que existe.
	var uso models.CouponUsage
	if err := models.DB.Where("coupon_id = ?", c.ID).First(&uso).Error; err != nil {
		t.Fatalf("uso não registrado: %v", err)
	}
	if uso.OrderID != fmt.Sprint(out1["orderId"]) {
		t.Fatalf("o uso deveria apontar para o pedido criado (%v), aponta para %q",
			out1["orderId"], uso.OrderID)
	}
}

// ── O uso do cupom sobrevive quando o pedido não nasce ──

// Falha no caminho DEPOIS do consumo não pode queimar o uso: o cupom é
// consumido antes da checagem de horário, e um restaurante fechado é evento
// comum — o cliente tenta de novo quando abrir. Sem o release, um cupom de
// uso único virava cinza na primeira tentativa fracassada.
func TestCreateOrder_CupomNaoQueimaQuandoEstabelecimentoFechado(t *testing.T) {
	app := setupCreateOrder(t)
	seedCoupon(t, models.Coupon{
		Code: "UNICO", DiscountType: "FIXED", DiscountValue: 10,
		EstablishmentID: 1, MaxUses: 1, FundedBy: models.CouponFundedByPlatform,
	})
	token := tokenComTelefone(t, "+5511999900001")

	// Fecha o estabelecimento DEPOIS do setup (que semeia aberto).
	// checkEstablishmentOpen trata OpenData != NULL como "aberto": fechado
	// é a coluna NULL, não um texto qualquer.
	if err := authModels.DB.Model(&authModels.Establishment{ID: 1}).
		Update("open_data", gorm.Expr("NULL")).Error; err != nil {
		t.Fatalf("fechar estabelecimento: %v", err)
	}

	resp, out := postPedido(t, app, token, corpoDoPedido(`"coupon_code":"UNICO"`))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("esperava 400 para estabelecimento fechado, veio %d (%v)", resp.StatusCode, out)
	}

	// O uso tem de estar intacto: nenhuma linha em coupon_usages, e o
	// used_count do cupom de volta em zero.
	var usages int64
	models.DB.Model(&models.CouponUsage{}).Count(&usages)
	if usages != 0 {
		t.Fatalf("uso do cupom deveria ter sido devolvido, veio %d registro(s) em coupon_usages", usages)
	}
	var coupon models.Coupon
	if err := models.DB.Where("code = ?", "UNICO").First(&coupon).Error; err != nil {
		t.Fatalf("reler cupom: %v", err)
	}
	if coupon.UsedCount != 0 {
		t.Fatalf("used_count deveria ser 0, veio %d", coupon.UsedCount)
	}

	// E o mesmo pedido com o mesmo cupom tem de funcionar quando o
	// estabelecimento reabre — é exatamente o que o release preserva.
	reaberto := time.Now().Format(time.RFC3339)
	if err := authModels.DB.Model(&authModels.Establishment{ID: 1}).
		Update("open_data", &reaberto).Error; err != nil {
		t.Fatalf("reabrir estabelecimento: %v", err)
	}
	resp2, out2 := postPedido(t, app, token, corpoDoPedido(`"coupon_code":"UNICO"`))
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("esperava 200 com o estabelecimento aberto (o uso foi devolvido), veio %d (%v)", resp2.StatusCode, out2)
	}
}
