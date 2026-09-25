package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// No GORM, First(&x, idString) com string não numérica vira SQL literal.
// Estas rotas passavam o :id da URL assim — "1=1" era "WHERE 1=1".
//
// GET /coupons/1=1 devolvia o primeiro cupom: o admin sabe ler cupons, mas a
// mesma porta servia para montar qualquer condição SQL.
func TestGetCoupon_IDComSQLNaoViraCondicao(t *testing.T) {
	app := setupCouponAuthz(t)
	if err := models.DB.Create(&models.Coupon{Code: "SEGREDO10", EstablishmentID: 0, IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	admin := tokenFor(t, "admin", 0, "")

	for _, id := range []string{"1=1", "0=0", "1)or(1=1", "abc", "0", "-1"} {
		resp := doCoupon(t, app, "GET", "/coupons/"+id, admin, "")
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 400 {
			t.Errorf("GET /coupons/%s: got %d (%s), want 400", id, resp.StatusCode, body)
		}
		var c map[string]interface{}
		if json.Unmarshal(body, &c) == nil && c["code"] == "SEGREDO10" {
			t.Errorf("GET /coupons/%s devolveu o cupom", id)
		}
	}
	if resp := doCoupon(t, app, "GET", "/coupons/1", admin, ""); resp.StatusCode != 200 {
		t.Fatalf("id válido: got %d, want 200", resp.StatusCode)
	}
}

// Todas as rotas com :id corrigidas recusam id que não é inteiro positivo
// antes de tocar no banco (aqui nem há banco: sem a validação, o handler
// chegaria ao models.DB nulo).
func TestRotasComID_RecusamIDNaoNumerico(t *testing.T) {
	prev := models.DB
	models.DB = nil
	t.Cleanup(func() { models.DB = prev })
	t.Setenv("JWT_SECRET", couponAuthzSecret)
	admin := tokenFor(t, "admin", 0, "")

	rotas := []struct {
		method, route string
		h             fiber.Handler
	}{
		{"GET", "/coupons/:id", GetCoupon},
		{"DELETE", "/coupons/:id", DeleteCoupon},
		{"DELETE", "/products/delete/:id", DeleteProduct},
		{"PUT", "/additional/:id", UpdateAdditional},
		{"DELETE", "/additional/:id", DeleteAdditional},
		{"PUT", "/reviews/respond/:id", RespondToReview},
		{"DELETE", "/categories/:id", DeleteCategory},
		{"PUT", "/categories/:id", UpdateCategory},
		{"PUT", "/delivery/regions/:id", UpdateDeliveryRegion},
		{"DELETE", "/delivery/regions/:id", DeleteDeliveryRegion},
	}
	for _, r := range rotas {
		app := fiber.New()
		app.Add(r.method, r.route, r.h)
		path := r.route[:len(r.route)-len(":id")] + "1=1"
		req := httptest.NewRequest(r.method, path, bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+admin)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", r.method, path, err)
		}
		if resp.StatusCode != 400 {
			t.Errorf("%s %s: got %d, want 400", r.method, path, resp.StatusCode)
		}
	}
}
