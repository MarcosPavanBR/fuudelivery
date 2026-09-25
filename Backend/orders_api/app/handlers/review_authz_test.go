package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupReviews(t *testing.T) *fiber.App {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.OrderDocument{}, &models.Review{}, &models.LoyaltyPoints{}, &models.LoyaltyTransaction{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = prev })
	t.Setenv("JWT_SECRET", couponAuthzSecret)

	for _, o := range []models.OrderDocument{
		{LegacyID: "ped-ana", EstablishmentID: 1, UserPhone: "+5511911110001", Status: "FINISHED", Payload: []byte(`{}`)},
		{LegacyID: "ped-andamento", EstablishmentID: 1, UserPhone: "+5511911110001", Status: "PREPARING", Payload: []byte(`{}`)},
	} {
		if err := db.Create(&o).Error; err != nil {
			t.Fatal(err)
		}
	}
	app := fiber.New()
	app.Post("/reviews", CreateReview)
	app.Put("/reviews/respond/:id", RespondToReview)
	app.Get("/reviews/establishment/:id", GetEstablishmentReviews)
	return app
}

func clientToken(t *testing.T, phone, role string) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": 1, "role": role, "phone": phone, "account_type": "client",
	}).SignedString([]byte(couponAuthzSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func reviewBody(order, phone string, rating int, comment string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"order_id": order, "user_phone": phone, "rating": rating, "comment": comment, "user_name": "Ana",
	})
	return string(b)
}

// Outro cliente não avalia o pedido da Ana — nem mandando o próprio telefone
// no corpo (antes bastava o corpo bater com o token).
func TestCreateReview_SoODonoDoPedido(t *testing.T) {
	app := setupReviews(t)
	intruso := "+5511922220002"
	r := doCoupon(t, app, "POST", "/reviews", clientToken(t, intruso, "client"), reviewBody("ped-ana", intruso, 1, "péssimo"))
	if r.StatusCode != 403 {
		t.Fatalf("intruso avaliando pedido alheio: status %d, esperava 403", r.StatusCode)
	}
	// Admin também não avalia (creditava pontos a telefone qualquer).
	r = doCoupon(t, app, "POST", "/reviews", clientToken(t, "", "admin"), reviewBody("ped-ana", intruso, 1, "x"))
	if r.StatusCode != 403 {
		t.Fatalf("admin avaliando: status %d, esperava 403", r.StatusCode)
	}
	var n int64
	models.DB.Model(&models.Review{}).Count(&n)
	if n != 0 {
		t.Fatalf("%d avaliação(ões) falsa(s) gravada(s)", n)
	}
}

func TestCreateReview_DonoAvaliaUmaVezELojaResponde(t *testing.T) {
	app := setupReviews(t)
	ana := clientToken(t, "5511911110001", "client") // formato diferente, mesmo número
	longo := strings.Repeat("a", 900)

	if r := doCoupon(t, app, "POST", "/reviews", ana, reviewBody("ped-andamento", "", 5, "")); r.StatusCode != 400 {
		t.Fatalf("pedido não entregue: status %d, esperava 400", r.StatusCode)
	}
	if r := doCoupon(t, app, "POST", "/reviews", ana, reviewBody("ped-ana", "+5599000000000", 5, longo)); r.StatusCode != 200 {
		t.Fatalf("dona avaliando: status %d", r.StatusCode)
	}
	if r := doCoupon(t, app, "POST", "/reviews", ana, reviewBody("ped-ana", "", 4, "de novo")); r.StatusCode != 409 {
		t.Fatalf("segunda avaliação: status %d, esperava 409", r.StatusCode)
	}

	var rv models.Review
	models.DB.First(&rv)
	if rv.UserPhone != "+5511911110001" {
		t.Fatalf("telefone gravado deveria ser o do pedido, veio %q", rv.UserPhone)
	}
	if len([]rune(rv.Comment)) != maxReviewText {
		t.Fatalf("comentário com %d caracteres, esperava corte em %d", len([]rune(rv.Comment)), maxReviewText)
	}
	var pts models.LoyaltyPoints
	models.DB.Where("user_phone = ?", "+5511911110001").First(&pts)
	if pts.Points != 5 {
		t.Fatalf("pontos da avaliação: %d, esperava 5 (uma vez)", pts.Points)
	}

	// Loja 2 não responde avaliação da loja 1; a loja 1 responde.
	path := "/reviews/respond/" + itoa(rv.ID)
	if r := doCoupon(t, app, "PUT", path, tokenFor(t, "user", 2, ""), `{"response_text":"x"}`); r.StatusCode != 403 {
		t.Fatalf("outra loja respondendo: status %d", r.StatusCode)
	}
	if r := doCoupon(t, app, "PUT", path, tokenFor(t, "user", 1, ""), `{"response_text":"Obrigado, Ana!"}`); r.StatusCode != 200 {
		t.Fatalf("loja respondendo: status %d", r.StatusCode)
	}

	r := doCoupon(t, app, "GET", "/reviews/establishment/1", ana, "")
	var out struct {
		Reviews []map[string]interface{} `json:"reviews"`
	}
	json.NewDecoder(r.Body).Decode(&out)
	if len(out.Reviews) != 1 || out.Reviews[0]["response_text"] != "Obrigado, Ana!" || out.Reviews[0]["id"] == nil {
		t.Fatalf("listagem sem id/resposta: %+v", out.Reviews)
	}
	if _, vazou := out.Reviews[0]["user_phone"]; vazou {
		t.Fatal("telefone do cliente não pode aparecer na listagem")
	}
}

func itoa(v uint) string { b, _ := json.Marshal(v); return string(b) }
