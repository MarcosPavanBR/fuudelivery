// Package handlers - coupon_authz_test.go
//
// Autorização dos handlers de cupom.
//
// As rotas de cupom usam `protectedRoute`, que SÓ valida o JWT — qualquer
// usuário logado passa por ela. Os handlers não checavam papel nem dono, e o
// resultado era dinheiro na mão de quem pedisse:
//
//   - POST /coupons: um cliente comum criava um PERCENTAGE de 100 (a validação
//     só recusa ACIMA de 100) para o establishment_id que escolhesse;
//   - DELETE /coupons/:id: qualquer logado desativava qualquer cupom pelo id,
//     inclusive a promoção de um concorrente;
//   - POST /coupons/referral: o telefone do indicador vinha do corpo, então dava
//     para cunhar cupons de R$10 em loop com números arbitrários.
//
// É a regra sempre ativa do projeto: autorização por recurso em todo handler,
// sem exceção (esquadrao/rules/common/security.md).
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const couponAuthzSecret = "coupon-authz-test-secret"

func setupCouponAuthz(t *testing.T) *fiber.App {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("abrir sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Coupon{}, &models.CouponUsage{}); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	prev := models.DB
	models.DB = db
	t.Setenv("JWT_SECRET", couponAuthzSecret)
	t.Cleanup(func() { models.DB = prev })

	app := fiber.New()
	app.Post("/coupons", CreateCoupon)
	app.Delete("/coupons/:id", DeleteCoupon)
	app.Post("/coupons/referral", GenerateReferralCoupon)
	return app
}

// tokenFor monta um JWT com os claims que os helpers de middleware leem.
func tokenFor(t *testing.T, role string, establishmentID int64, phone string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"id":   1,
		"role": role,
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	if establishmentID > 0 {
		claims["establishment_id"] = float64(establishmentID)
	}
	if phone != "" {
		claims["phone"] = phone
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(couponAuthzSecret))
	if err != nil {
		t.Fatalf("assinar token: %v", err)
	}
	return s
}

func doCoupon(t *testing.T, app *fiber.App, method, path, token, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp
}

func novoCupomBody(establishmentID int, pct float64) string {
	inicio := time.Now().Format(time.RFC3339)
	fim := time.Now().AddDate(0, 1, 0).Format(time.RFC3339)
	return fmt.Sprintf(`{"code":"PROMO%d","discount_type":"PERCENTAGE","discount_value":%.0f,
		"establishment_id":%d,"start_date":%q,"expiry_date":%q}`,
		establishmentID, pct, establishmentID, inicio, fim)
}

// ── Criar ──

func TestCreateCoupon_ClienteNaoPodeCriar(t *testing.T) {
	app := setupCouponAuthz(t)

	// O caso que dava comida de graça: cliente comum, 100% de desconto.
	resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "client", 0, "+5511999900001"), novoCupomBody(7, 100))

	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("cliente comum não pode criar cupom, veio %d", resp.StatusCode)
	}
	var n int64
	models.DB.Model(&models.Coupon{}).Count(&n)
	if n != 0 {
		t.Fatalf("nenhum cupom deveria ter sido criado, há %d", n)
	}
}

func TestCreateCoupon_EstabelecimentoNaoCriaParaOutro(t *testing.T) {
	app := setupCouponAuthz(t)

	// Token do estabelecimento 7 tentando criar cupom para o 9.
	resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "establishment", 7, ""), novoCupomBody(9, 50))

	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("estabelecimento não pode criar cupom para outro, veio %d", resp.StatusCode)
	}
}

// Contraprova: sem ela, a checagem poderia estar recusando tudo.
func TestCreateCoupon_AdminEDonoPodem(t *testing.T) {
	app := setupCouponAuthz(t)

	if resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "admin", 0, ""), novoCupomBody(7, 20)); resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("admin deveria criar cupom, veio %d", resp.StatusCode)
	}
	if resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "establishment", 9, ""), novoCupomBody(9, 20)); resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("estabelecimento deveria criar cupom para si, veio %d", resp.StatusCode)
	}
}

// ── Desativar ──

func TestDeleteCoupon_ClienteNaoDesativaPromocaoAlheia(t *testing.T) {
	app := setupCouponAuthz(t)

	cupom := models.Coupon{
		Code: "DOLOJA", DiscountType: "PERCENTAGE", DiscountValue: 10,
		EstablishmentID: 7, IsActive: true,
		StartDate: time.Now(), ExpiryDate: time.Now().AddDate(0, 1, 0),
	}
	if err := models.DB.Create(&cupom).Error; err != nil {
		t.Fatalf("semear cupom: %v", err)
	}

	resp := doCoupon(t, app, "DELETE", fmt.Sprintf("/coupons/%d", cupom.ID),
		tokenFor(t, "client", 0, "+5511999900001"), "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("cliente não pode desativar cupom alheio, veio %d", resp.StatusCode)
	}

	var depois models.Coupon
	models.DB.First(&depois, cupom.ID)
	if !depois.IsActive {
		t.Fatal("o cupom continua ativo — a promoção não pode ser derrubada por terceiro")
	}
}

// ── Indicação ──

func TestReferralCoupon_SoParaOProprioTelefone(t *testing.T) {
	app := setupCouponAuthz(t)

	body := `{"referrer_phone":"+5511988887777","new_user_phone":"+5511977776666"}`
	resp := doCoupon(t, app, "POST", "/coupons/referral",
		tokenFor(t, "client", 0, "+5511900000000"), body)

	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("não pode gerar indicação para telefone alheio, veio %d", resp.StatusCode)
	}
	var n int64
	models.DB.Model(&models.Coupon{}).Count(&n)
	if n != 0 {
		t.Fatalf("nenhum cupom deveria ter sido cunhado, há %d", n)
	}
}

func TestReferralCoupon_ProprioTelefoneFunciona(t *testing.T) {
	app := setupCouponAuthz(t)

	body := `{"referrer_phone":"+5511900000000","new_user_phone":"+5511977776666"}`
	resp := doCoupon(t, app, "POST", "/coupons/referral",
		tokenFor(t, "client", 0, "+5511900000000"), body)

	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("indicação para o próprio telefone deveria funcionar, veio %d", resp.StatusCode)
	}
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["referrer_coupon_code"] == nil {
		t.Error("esperava os códigos gerados na resposta")
	}
}

func TestMain(m *testing.M) { os.Exit(m.Run()) }
