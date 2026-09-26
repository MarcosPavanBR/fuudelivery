//go:build integration

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Cadastro de entregador: o público (/delivery-man/register) escolhia o
// próprio status/zona (entrava "available" na fila), senha vazia passava, e o
// login com e-mail vazio caía no primeiro entregador da tabela.
func TestDeliveryManRegister_Endurecido(t *testing.T) {
	uri := requirePostgres(t)
	const secret = "courier-reg-secret"
	t.Setenv("JWT_SECRET", secret)
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("DROP TABLE IF EXISTS delivery_men CASCADE")
	if err := db.AutoMigrate(&models.DeliveryMan{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	defer func() { models.DB = prev }()

	app := fiber.New()
	app.Post("/delivery-man/register", CreateDeliveryMan)
	app.Post("/delivery-man", CreateDeliveryMan)
	app.Post("/delivery-man/login", LoginDeliveryMan)
	call := func(path string, body any, token string) int {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	admin, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": float64(1), "role": "admin", "account_type": "user"}).SignedString([]byte(secret))

	if st := call("/delivery-man/register", map[string]any{"name": "Zé", "email": "ze@x.com", "password": ""}, ""); st != 400 {
		t.Fatalf("senha vazia devia dar 400, deu %d", st)
	}
	if st := call("/delivery-man/register", map[string]any{"name": "Zé", "email": "Ze@X.com", "password": "segredo1", "status": "available", "max_orders": 50}, ""); st != 201 {
		t.Fatalf("cadastro público: %d", st)
	}
	var ze models.DeliveryMan
	db.First(&ze, "email = ?", "ze@x.com")
	if ze.Status != "offline" || ze.MaxOrders != 3 {
		t.Fatalf("cadastro público definiu status/limite: %+v", ze)
	}
	if st := call("/delivery-man/register", map[string]any{"name": "Zé 2", "email": "ZE@x.com", "password": "segredo1"}, ""); st != 409 {
		t.Fatalf("e-mail repetido devia dar 409, deu %d", st)
	}

	if st := call("/delivery-man", map[string]any{"name": "Ana", "email": "ana@x.com", "password": "segredo1", "status": "available", "max_orders": 2}, admin); st != 201 {
		t.Fatalf("cadastro pelo admin: %d", st)
	}
	var ana models.DeliveryMan
	db.First(&ana, "email = ?", "ana@x.com")
	if ana.Status != "available" || ana.MaxOrders != 2 {
		t.Fatalf("admin não conseguiu definir status/limite: %+v", ana)
	}
	if st := call("/delivery-man", map[string]any{"name": "B", "email": "b@x.com", "password": "segredo1", "status": "online"}, admin); st != 400 {
		t.Fatalf("status desconhecido devia dar 400, deu %d", st)
	}

	// Login: e-mail em maiúsculas entra; vazio não entra em conta nenhuma.
	if st := call("/delivery-man/login", map[string]any{"email": "ZE@x.com", "password": "segredo1"}, ""); st != 200 {
		t.Fatalf("login com maiúsculas: %d", st)
	}
	db.Exec("UPDATE delivery_men SET password = '' WHERE email = 'ze@x.com'")
	if st := call("/delivery-man/login", map[string]any{"email": "", "password": ""}, ""); st != 403 {
		t.Fatalf("login vazio devia dar 403, deu %d", st)
	}
}
