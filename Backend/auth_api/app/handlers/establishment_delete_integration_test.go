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

// Excluir loja: com pedidos não exclui (409, desativar); sem pedidos leva o
// cardápio junto e deixa os usuários sem loja, com as sessões derrubadas.
// Desativada, a loja não consegue se abrir. E o admin tira alguém de uma loja
// mandando establishment_id 0.
func TestEstablishmentDeleteDisableUnlink(t *testing.T) {
	uri := requirePostgres(t)
	const secret = "est-delete-secret"
	t.Setenv("JWT_SECRET", secret)
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tbl := range []string{"order_documents", "category_products", "products", "categories", "business_hours", "refresh_tokens", "establishments", "users"} {
		db.Exec("DROP TABLE IF EXISTS " + tbl + " CASCADE")
	}
	if err := db.AutoMigrate(&models.User{}, &models.Establishment{}, &models.BusinessHours{}, &models.RefreshToken{}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE order_documents (id serial PRIMARY KEY, establishment_id bigint)`,
		`CREATE TABLE products (id serial PRIMARY KEY, establishment_id bigint, name text)`,
		`CREATE TABLE categories (id serial PRIMARY KEY, establishment_id bigint)`,
		`CREATE TABLE category_products (category_id int REFERENCES categories(id), product_id int REFERENCES products(id))`,
		`INSERT INTO establishments (id, name) VALUES (1, 'Com pedidos'), (2, 'Sem pedidos')`,
		`INSERT INTO order_documents (establishment_id) VALUES (1)`,
		`INSERT INTO products (id, establishment_id, name) VALUES (10, 2, 'Pizza'), (11, 1, 'Suco')`,
		`INSERT INTO categories (id, establishment_id) VALUES (20, 2)`,
		`INSERT INTO category_products VALUES (20, 10)`,
		`INSERT INTO business_hours (establishment_id, day_of_week, is_open, open_time, close_time) VALUES (2, 1, true, '08:00', '18:00')`,
		`INSERT INTO users (id, name, email, password, role, establishment_id) VALUES (5, 'Admin', 'a@x.com', 'x', 'admin', 0), (6, 'Dono', 'd@x.com', 'x', 'user', 2), (7, 'Caixa', 'c@x.com', 'x', 'user', 1)`,
		`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES (6, 'tok-dono', now() + interval '1 day'), (7, 'tok-caixa', now() + interval '1 day')`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	prev := models.DB
	models.DB = db
	defer func() { models.DB = prev }()

	app := fiber.New()
	app.Delete("/establishments/:id", DeleteEstablishment)
	app.Put("/establishments/:id/disabled", SetEstablishmentDisabled)
	app.Put("/establishments/status/handler/:id", HandlerEstablishmentStatus)
	app.Put("/users/:id", UpdateUser)
	admin, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": float64(5), "role": "admin", "account_type": "user"}).SignedString([]byte(secret))
	call := func(method, path string, body any) (int, map[string]any) {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+admin)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	count := func(q string) int64 {
		var n int64
		db.Raw(q).Scan(&n)
		return n
	}

	if st, out := call("DELETE", "/establishments/1", nil); st != 409 || out["has_history"] != true {
		t.Fatalf("loja com pedido devia dar 409 has_history, deu %d %v", st, out)
	}
	if count("SELECT COUNT(*) FROM products WHERE establishment_id = 1") != 1 {
		t.Fatal("a recusa não pode apagar nada")
	}

	if st, out := call("DELETE", "/establishments/2", nil); st != 200 {
		t.Fatalf("excluir loja sem pedidos: %d %v", st, out)
	}
	if count("SELECT COUNT(*) FROM products WHERE establishment_id = 2")+count("SELECT COUNT(*) FROM categories WHERE establishment_id = 2")+
		count("SELECT COUNT(*) FROM business_hours WHERE establishment_id = 2")+count("SELECT COUNT(*) FROM category_products") != 0 {
		t.Fatal("cardápio/horários da loja excluída ficaram órfãos")
	}
	if count("SELECT establishment_id FROM users WHERE id = 6") != 0 || count("SELECT COUNT(*) FROM refresh_tokens WHERE token = 'tok-dono' AND revoked") != 1 {
		t.Fatal("dono da loja excluída devia ficar sem loja e com a sessão derrubada")
	}

	// Desativar: fecha e impede reabrir.
	db.Exec("UPDATE establishments SET open_data = 'x' WHERE id = 1")
	if st, _ := call("PUT", "/establishments/1/disabled", map[string]any{"disabled": true}); st != 200 {
		t.Fatalf("desativar: %d", st)
	}
	if count("SELECT COUNT(*) FROM establishments WHERE id = 1 AND open_data IS NULL AND disabled_at IS NOT NULL") != 1 {
		t.Fatal("desativar devia fechar a loja e marcar disabled_at")
	}
	if st, _ := call("PUT", "/establishments/status/handler/1", nil); st != 403 {
		t.Fatalf("loja desativada não pode abrir, deu %d", st)
	}
	call("PUT", "/establishments/1/disabled", map[string]any{"disabled": false})
	if st, _ := call("PUT", "/establishments/status/handler/1", nil); st != 200 {
		t.Fatalf("reativada, a loja volta a poder abrir: %d", st)
	}

	// Tirar o caixa da loja 1.
	if st, out := call("PUT", "/users/7", map[string]any{"establishment_id": 0}); st != 200 {
		t.Fatalf("desvincular: %d %v", st, out)
	}
	if count("SELECT establishment_id FROM users WHERE id = 7") != 0 || count("SELECT COUNT(*) FROM refresh_tokens WHERE token = 'tok-caixa' AND revoked") != 1 {
		t.Fatal("desvincular devia zerar a loja e derrubar a sessão")
	}
	if st, _ := call("PUT", "/users/7", map[string]any{"establishment_id": 99}); st != 400 {
		t.Fatalf("loja inexistente devia dar 400, deu %d", st)
	}
}
