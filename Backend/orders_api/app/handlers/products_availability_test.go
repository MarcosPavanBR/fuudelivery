package handlers

import (
	"strings"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func availabilityApp(t *testing.T) *fiber.App {
	t.Helper()
	setupDeliveryFeeTestDB(t)
	t.Setenv("JWT_SECRET", couponAuthzSecret)
	app := fiber.New()
	app.Put("/products/:id/availability", SetProductAvailability)
	app.Post("/products/multi-create", CreateMultProducts)
	return app
}

func available(t *testing.T, id uint) bool {
	t.Helper()
	var p models.Product
	if err := models.DB.First(&p, id).Error; err != nil {
		t.Fatal(err)
	}
	return p.Available
}

// Produto novo nasce à venda (default da coluna), e o dono pausa e reativa.
// O false precisa chegar ao banco: Save/Updates com struct o pulariam.
func TestSetProductAvailability_DonoPausaEReativa(t *testing.T) {
	app := availabilityApp(t)
	seedProduct(t, 10, 1, 20)
	if !available(t, 10) {
		t.Fatal("produto novo deveria nascer disponível")
	}
	dono := tokenFor(t, "user", 1, "")

	if r := doCoupon(t, app, "PUT", "/products/10/availability", dono, `{"available":false}`); r.StatusCode != 200 {
		t.Fatalf("pausar: status %d", r.StatusCode)
	}
	if available(t, 10) {
		t.Fatal("available=false não foi gravado")
	}
	if r := doCoupon(t, app, "PUT", "/products/10/availability", dono, `{"available":true}`); r.StatusCode != 200 {
		t.Fatalf("reativar: status %d", r.StatusCode)
	}
	if !available(t, 10) {
		t.Fatal("available=true não foi gravado")
	}
}

func TestSetProductAvailability_OutraLojaNaoMexe(t *testing.T) {
	app := availabilityApp(t)
	seedProduct(t, 10, 1, 20)
	if r := doCoupon(t, app, "PUT", "/products/10/availability", tokenFor(t, "user", 2, ""), `{"available":false}`); r.StatusCode != 403 {
		t.Fatalf("loja 2 pausando produto da loja 1: status %d, esperava 403", r.StatusCode)
	}
	if r := doCoupon(t, app, "PUT", "/products/10/availability", tokenFor(t, "client", 0, "5511999990000"), `{"available":false}`); r.StatusCode != 403 {
		t.Fatalf("cliente pausando produto: status %d, esperava 403", r.StatusCode)
	}
	if !available(t, 10) {
		t.Fatal("produto foi pausado por quem não é dono")
	}
	if r := doCoupon(t, app, "PUT", "/products/10/availability", tokenFor(t, "user", 1, ""), `{}`); r.StatusCode != 400 {
		t.Fatalf("sem available: status %d, esperava 400", r.StatusCode)
	}
}

// Pedido com item esgotado é recusado no servidor, mesmo que o app do
// cliente esteja com o cardápio antigo em cache.
func TestComputeOrderTotal_RecusaItemEsgotado(t *testing.T) {
	setupDeliveryFeeTestDB(t)
	seedDelivery(t, 1, 5, 0)
	seedProduct(t, 10, 1, 20)
	models.DB.Model(&models.Product{ID: 10}).Update("available", false)

	_, _, err := computeOrderTotal(cartDe(10, 1), endereco("01310100"), 1, nil)
	if err == nil || !strings.Contains(err.Error(), "esgotado") {
		t.Fatalf("esperava erro de item esgotado, veio %v", err)
	}
}

// O lote só checava o primeiro item: o segundo criava produto em outra loja.
func TestCreateMultProducts_AutorizaCadaItem(t *testing.T) {
	app := availabilityApp(t)
	body := `[{"name":"Meu","price":10,"establishmentId":1},{"name":"Intruso","price":1,"establishmentId":2}]`
	if r := doCoupon(t, app, "POST", "/products/multi-create", tokenFor(t, "user", 1, ""), body); r.StatusCode != 403 {
		t.Fatalf("lote com item de outra loja: status %d, esperava 403", r.StatusCode)
	}
	var n int64
	models.DB.Model(&models.Product{}).Count(&n)
	if n != 0 {
		t.Fatalf("lote recusado mas %d produto(s) gravado(s)", n)
	}
}

func TestNormalizeCartNotes(t *testing.T) {
	longa := strings.Repeat("é", 200)
	cart := []dto.CartItem{
		{Note: "  sem cebola\n\ne bem passado  "},
		{Note: longa},
		{Note: "\t"},
	}
	normalizeCartNotes(cart)
	if cart[0].Note != "sem cebola e bem passado" {
		t.Fatalf("nota 0: %q", cart[0].Note)
	}
	if n := len([]rune(cart[1].Note)); n != maxItemNoteRunes {
		t.Fatalf("nota longa com %d caracteres, esperava %d", n, maxItemNoteRunes)
	}
	if cart[2].Note != "" {
		t.Fatalf("nota só com espaço deveria ficar vazia: %q", cart[2].Note)
	}
}

func TestCreateMultProducts_LoteDaPropriaLojaPassa(t *testing.T) {
	app := availabilityApp(t)
	body := `[{"name":"A","price":10,"establishmentId":1},{"name":"B","price":12,"establishmentId":1}]`
	if r := doCoupon(t, app, "POST", "/products/multi-create", tokenFor(t, "user", 1, ""), body); r.StatusCode != 200 {
		t.Fatalf("lote da própria loja: status %d", r.StatusCode)
	}
	var n int64
	models.DB.Model(&models.Product{}).Where("establishment_id = 1 AND available = ?", true).Count(&n)
	if n != 2 {
		t.Fatalf("esperava 2 produtos disponíveis, veio %d", n)
	}
}
