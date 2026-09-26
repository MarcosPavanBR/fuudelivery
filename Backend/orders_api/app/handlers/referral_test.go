package handlers

import (
	"fmt"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func setupReferral(t *testing.T) *fiber.App {
	t.Helper()
	app := setupCreateOrder(t)
	if err := models.DB.AutoMigrate(&models.ReferralCode{}, &models.Referral{},
		&models.LoyaltyPoints{}, &models.LoyaltyTransaction{}); err != nil {
		t.Fatal(err)
	}
	return app
}

func pedidoCom(t *testing.T, app *fiber.App, phone, code string) (int, map[string]any) {
	t.Helper()
	resp, out := postPedido(t, app, tokenComTelefone(t, phone), corpoDoPedido(fmt.Sprintf(`"coupon_code":%q`, code)))
	return resp.StatusCode, out
}

func setStatus(t *testing.T, orderID any, status string) {
	t.Helper()
	doc, err := findOrderByLegacyID(fmt.Sprint(orderID))
	if err != nil {
		t.Fatal(err)
	}
	doc.Status = status
	models.DB.Model(doc).Update("status", status)
	notifyOrderStatusChanged(doc)
}

func pontos(phone string) int {
	var acc models.LoyaltyPoints
	models.DB.Where("user_phone = ?", phone).First(&acc)
	return acc.Points
}

const (
	ana   = "+5511900000001" // indica
	bia   = "+5511900000002" // amiga nova
	carlo = "+5511900000003" // já é cliente
	duda  = "+5511900000004"
)

// Amiga nova ganha R$ 10 no 1º pedido; quem indicou ganha 100 pontos só
// quando o pedido é ENTREGUE, uma vez.
func TestReferral_AmigaGanhaEIndicadoraRecebeNaEntrega(t *testing.T) {
	app := setupReferral(t)
	rc, err := referralCodeFor(ana)
	if err != nil {
		t.Fatal(err)
	}

	code, out := pedidoCom(t, app, bia, rc.Code)
	if code != 200 {
		t.Fatalf("1º pedido com código: status %d (%v)", code, out)
	}
	// 2 × 30 + frete 11 = 71; menos R$ 10 = 61.
	if out["order_total"] != 61.0 {
		t.Fatalf("total com indicação: %v, esperava 61", out["order_total"])
	}
	if pontos(ana) != 0 {
		t.Fatal("quem indicou não pode ganhar antes da entrega")
	}

	setStatus(t, out["orderId"], "FINISHED")
	setStatus(t, out["orderId"], "FINISHED") // evento repetido
	if pontos(ana) != referralRewardPoints {
		t.Fatalf("pontos da indicadora: %d, esperava %d (uma vez)", pontos(ana), referralRewardPoints)
	}

	// Mesmo código de novo: só vale no 1º pedido.
	if code, out := pedidoCom(t, app, bia, rc.Code); code != 400 {
		t.Fatalf("2º pedido com código: status %d (%v), esperava 400", code, out)
	}
}

func TestReferral_Travas(t *testing.T) {
	app := setupReferral(t)
	rc, _ := referralCodeFor(ana)

	if code, _ := pedidoCom(t, app, ana, rc.Code); code != 400 {
		t.Fatalf("usar o próprio código: status %d, esperava 400", code)
	}
	// Quem já pediu antes não é "amigo novo".
	models.DB.Create(&models.OrderDocument{LegacyID: "antigo", EstablishmentID: 1, UserPhone: carlo, Status: "FINISHED", Payload: []byte(`{}`)})
	if code, _ := pedidoCom(t, app, carlo, rc.Code); code != 400 {
		t.Fatalf("cliente antigo com código: status %d, esperava 400", code)
	}
	var n int64
	models.DB.Model(&models.Referral{}).Count(&n)
	if n != 0 {
		t.Fatalf("%d indicação(ões) criada(s) indevidamente", n)
	}
}

// Pedido recusado devolve o cupom de boas-vindas: a amiga pede de novo.
func TestReferral_CanceladoDevolveOCupom(t *testing.T) {
	app := setupReferral(t)
	rc, _ := referralCodeFor(ana)

	code, out := pedidoCom(t, app, duda, rc.Code)
	if code != 200 {
		t.Fatalf("status %d (%v)", code, out)
	}
	setStatus(t, out["orderId"], "DENIED")
	if pontos(ana) != 0 {
		t.Fatal("pedido recusado não premia")
	}
	code, out = pedidoCom(t, app, duda, rc.Code)
	if code != 200 || out["order_total"] != 61.0 {
		t.Fatalf("novo pedido após recusa: status %d total %v", code, out["order_total"])
	}
}
