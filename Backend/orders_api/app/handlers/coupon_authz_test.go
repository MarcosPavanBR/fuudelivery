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
	app.Get("/coupons", ListCoupons)
	app.Get("/coupons/:id", GetCoupon)
	app.Delete("/coupons/:id", DeleteCoupon)
	app.Post("/coupons/apply", ApplyCoupon)
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

// ── Quem banca o desconto ──
//
// FundedBy decide de qual lado o split subtrai. Se o estabelecimento pudesse
// escolher "platform", ele criaria a promoção dele e mandaria a conta para a
// plataforma.

func bodyComFunded(establishmentID int, fundedBy string) string {
	inicio := time.Now().Format(time.RFC3339)
	fim := time.Now().AddDate(0, 1, 0).Format(time.RFC3339)
	return fmt.Sprintf(`{"code":"FUND%s%d","discount_type":"FIXED","discount_value":5,
		"establishment_id":%d,"funded_by":%q,"start_date":%q,"expiry_date":%q}`,
		fundedBy, establishmentID, establishmentID, fundedBy, inicio, fim)
}

func TestCreateCoupon_EstabelecimentoNaoEmpurraCustoParaPlataforma(t *testing.T) {
	app := setupCouponAuthz(t)

	resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "establishment", 7, ""), bodyComFunded(7, "platform"))

	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("estabelecimento não pode fazer a plataforma bancar, veio %d", resp.StatusCode)
	}
}

func TestCreateCoupon_AdminEscolheQuemBanca(t *testing.T) {
	app := setupCouponAuthz(t)

	for _, quem := range []string{"platform", "establishment"} {
		resp := doCoupon(t, app, "POST", "/coupons",
			tokenFor(t, "admin", 0, ""), bodyComFunded(7, quem))
		if resp.StatusCode != fiber.StatusCreated {
			t.Fatalf("admin deveria poder criar cupom bancado por %s, veio %d", quem, resp.StatusCode)
		}
		var out models.Coupon
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decodificar resposta: %v", err)
		}
		if out.FundedBy != quem {
			t.Fatalf("esperava funded_by=%q gravado, veio %q", quem, out.FundedBy)
		}
	}
}

// Omitir funded_by não pode virar "o restaurante paga" por acidente.
func TestCreateCoupon_SemFundedByEhPlataforma(t *testing.T) {
	app := setupCouponAuthz(t)

	resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "admin", 0, ""), novoCupomBody(7, 15))
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("esperava 201, veio %d", resp.StatusCode)
	}
	var out models.Coupon
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if out.FundedBy != models.CouponFundedByPlatform {
		t.Fatalf("sem escolha explícita quem banca é a plataforma, veio %q", out.FundedBy)
	}
}

// O padrão do estabelecimento é ele mesmo — e omitir o campo não pode virar
// erro. Uma primeira versão desta regra fixava o padrão em "platform" e depois
// recusava não-admin que pedisse "platform": o dono que simplesmente não
// mandava o campo tomava 403 criando o próprio cupom.
func TestCreateCoupon_DonoSemFundedByBancaEleMesmo(t *testing.T) {
	app := setupCouponAuthz(t)

	resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "establishment", 7, ""), novoCupomBody(7, 15))
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("dono criando cupom sem funded_by deveria dar 201, veio %d", resp.StatusCode)
	}
	var out models.Coupon
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if out.FundedBy != models.CouponFundedByEstablishment {
		t.Fatalf("cupom do dono é bancado por ele, veio %q", out.FundedBy)
	}
}

func TestCreateCoupon_FundedByInvalido(t *testing.T) {
	app := setupCouponAuthz(t)

	resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "admin", 0, ""), bodyComFunded(7, "ninguem"))
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("funded_by inválido deveria dar 400, veio %d", resp.StatusCode)
	}
}

// ── Identidade: o ApplyCoupon lê o telefone do token, nunca do corpo ──

func TestApplyCoupon_TelefoneVemDoTokenNaoDoCorpo(t *testing.T) {
	app := setupCouponAuthz(t)
	models.DB.Create(&models.Coupon{
		Code: "CORPO", DiscountType: "FIXED", DiscountValue: 5,
		StartDate: time.Now().Add(-time.Hour), ExpiryDate: time.Now().Add(time.Hour),
		IsActive: true,
	})

	// O corpo diz "+5511111111111" (vítima), mas o token é de "+5511900000000".
	resp := doCoupon(t, app, "POST", "/coupons/apply",
		tokenFor(t, "client", 0, "+5511900000000"),
		`{"code":"CORPO","user_phone":"+5511111111111","order_id":"ped-1","order_value":100}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("apply deveria valer com telefone do token, veio %d", resp.StatusCode)
	}

	// O uso tem que ter sido registrado no nome do DONO DO TOKEN.
	var uso models.CouponUsage
	if err := models.DB.First(&uso).Error; err != nil {
		t.Fatalf("uso não registrado: %v", err)
	}
	if uso.UserPhone != "+5511900000000" {
		t.Fatalf("uso gravado no nome do corpo forjado: %q", uso.UserPhone)
	}
}

func TestApplyCoupon_SemTelefoneNoTokenEh401(t *testing.T) {
	app := setupCouponAuthz(t)

	resp := doCoupon(t, app, "POST", "/coupons/apply",
		tokenFor(t, "client", 0, ""), // token SEM claim de telefone
		`{"code":"CORPO","user_phone":"+5511111111111","order_id":"ped-1","order_value":100}`)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("token sem telefone deveria dar 401, veio %d", resp.StatusCode)
	}
}

// ── Listagem: escopo pelo token, não pelo query ──

func TestListCoupons_EstabelecimentoNaoListaDoOutro(t *testing.T) {
	app := setupCouponAuthz(t)
	app.Get("/coupons", ListCoupons)

	models.DB.Create(&models.Coupon{
		Code: "DOCONCORRENTE", DiscountType: "FIXED", DiscountValue: 5,
		EstablishmentID: 9, IsActive: true,
		StartDate: time.Now().Add(-time.Hour), ExpiryDate: time.Now().Add(time.Hour),
	})

	resp := doCoupon(t, app, "GET", "/coupons?establishment_id=9",
		tokenFor(t, "establishment", 7, ""), "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("listagem deveria responder 200, veio %d", resp.StatusCode)
	}
	var out []models.Coupon
	_ = json.NewDecoder(resp.Body).Decode(&out)
	for _, c := range out {
		if c.EstablishmentID == 9 {
			t.Fatal("o query param mandou no escopo: estabelecimento 7 viu cupons do 9")
		}
	}
}

// Contraprova: o dono continua vendo o dele (e os globais).
func TestListCoupons_DonoVeOProprioEGlobais(t *testing.T) {
	app := setupCouponAuthz(t)
	app.Get("/coupons", ListCoupons)

	models.DB.Create(&models.Coupon{
		Code: "MEU", DiscountType: "FIXED", DiscountValue: 5,
		EstablishmentID: 7, IsActive: true,
		StartDate: time.Now().Add(-time.Hour), ExpiryDate: time.Now().Add(time.Hour),
	})
	models.DB.Create(&models.Coupon{
		Code: "GLOBAL", DiscountType: "FIXED", DiscountValue: 5,
		EstablishmentID: 0, IsActive: true,
		StartDate: time.Now().Add(-time.Hour), ExpiryDate: time.Now().Add(time.Hour),
	})

	resp := doCoupon(t, app, "GET", "/coupons", tokenFor(t, "establishment", 7, ""), "")
	var out []models.Coupon
	_ = json.NewDecoder(resp.Body).Decode(&out)
	codigos := map[string]bool{}
	for _, c := range out {
		codigos[c.Code] = true
	}
	if !codigos["MEU"] || !codigos["GLOBAL"] {
		t.Fatalf("dono deveria ver o próprio + globais, veio %v", codigos)
	}
}

// ── Leitura por id: o id na URL não é autorização ──

func TestGetCoupon_ClienteNaoLeCupomPorID(t *testing.T) {
	app := setupCouponAuthz(t)
	app.Get("/coupons/:id", GetCoupon)

	c := models.Coupon{Code: "SECRETO", DiscountType: "FIXED", DiscountValue: 5, EstablishmentID: 7}
	models.DB.Create(&c)

	resp := doCoupon(t, app, "GET", fmt.Sprintf("/coupons/%d", c.ID),
		tokenFor(t, "client", 0, "+5511999900001"), "")
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("cliente não deveria ler cupom por id, veio %d", resp.StatusCode)
	}
}

func TestGetCoupon_DonoLeOProprioMasNaoOAlheio(t *testing.T) {
	app := setupCouponAuthz(t)
	app.Get("/coupons/:id", GetCoupon)

	me := models.Coupon{Code: "MEUID", DiscountType: "FIXED", DiscountValue: 5, EstablishmentID: 7}
	alheio := models.Coupon{Code: "ALHEIO", DiscountType: "FIXED", DiscountValue: 5, EstablishmentID: 9}
	models.DB.Create(&me)
	models.DB.Create(&alheio)

	if resp := doCoupon(t, app, "GET", fmt.Sprintf("/coupons/%d", me.ID),
		tokenFor(t, "establishment", 7, ""), ""); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("dono deveria ler o próprio, veio %d", resp.StatusCode)
	}
	if resp := doCoupon(t, app, "GET", fmt.Sprintf("/coupons/%d", alheio.ID),
		tokenFor(t, "establishment", 7, ""), ""); resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("dono não deveria ler cupom do 9, veio %d", resp.StatusCode)
	}
}

// ── Criação: cupom global e limites negativos ──

func TestCreateCoupon_EstabelecimentoNaoCriaGlobal(t *testing.T) {
	app := setupCouponAuthz(t)

	// establishment_id = 0 vale em TODOS os restaurantes — decisão da plataforma.
	resp := doCoupon(t, app, "POST", "/coupons",
		tokenFor(t, "establishment", 7, ""), novoCupomBody(0, 20))

	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("estabelecimento não pode criar cupom global, veio %d", resp.StatusCode)
	}
	var n int64
	models.DB.Model(&models.Coupon{}).Count(&n)
	if n != 0 {
		t.Fatalf("nenhum cupom deveria ter sido criado, há %d", n)
	}
}

func TestCreateCoupon_LimitesNegativosSaoRecusados(t *testing.T) {
	app := setupCouponAuthz(t)

	body := `{"code":"NEGATIVO","discount_type":"FIXED","discount_value":5,
		"establishment_id":0,"max_uses":-5,"start_date":"` + time.Now().Format(time.RFC3339) +
		`","expiry_date":"` + time.Now().AddDate(0, 1, 0).Format(time.RFC3339) + `"}`
	resp := doCoupon(t, app, "POST", "/coupons", tokenFor(t, "admin", 0, ""), body)

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("max_uses negativo deveria dar 400, veio %d", resp.StatusCode)
	}
}
