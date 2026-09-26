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
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Usuários pelo painel admin nos DOIS schemas de users: o Create do GORM não
// preenchia "createdAt"/"updatedAt" nem conhecia o enum "Role" (produção);
// a senha digitada na edição era ignorada; e dava para excluir/rebaixar o
// único admin.
func TestAdminUsers_NosDoisSchemas(t *testing.T) {
	uri := requirePostgres(t)
	const secret = "admin-users-secret"
	t.Setenv("JWT_SECRET", secret)

	for _, legacy := range []bool{false, true} {
		name := "banco novo"
		if legacy {
			name = "schema de produção"
		}
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(postgres.Open(uri), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			for _, tbl := range []string{"refresh_tokens", "establishments", "users"} {
				db.Exec("DROP TABLE IF EXISTS " + tbl + " CASCADE")
			}
			db.Exec(`DROP TYPE IF EXISTS "Role"`)
			if err := db.AutoMigrate(&models.User{}, &models.Establishment{}, &models.RefreshToken{}); err != nil {
				t.Fatal(err)
			}
			if legacy {
				for _, q := range []string{
					`CREATE TYPE "Role" AS ENUM ('user', 'restaurant', 'admin')`,
					`ALTER TABLE users ALTER COLUMN role DROP DEFAULT`,
					`ALTER TABLE users ALTER COLUMN role TYPE "Role" USING role::"Role"`,
					`ALTER TABLE users ADD COLUMN "createdAt" timestamptz NOT NULL`,
					`ALTER TABLE users ADD COLUMN "updatedAt" timestamptz NOT NULL`,
				} {
					if err := db.Exec(q).Error; err != nil {
						t.Fatalf("%s: %v", q, err)
					}
				}
			}
			prev := models.DB
			models.DB = db
			defer func() { models.DB = prev }()
			db.Exec("INSERT INTO establishments (id, name) VALUES (1, 'Loja')")

			app := fiber.New()
			app.Post("/users", CreateUserAdmin)
			app.Put("/users/:id", UpdateUser)
			app.Delete("/users/:id", DeleteUser)
			app.Post("/users/login", Login)
			call := func(method, path string, body any, token string) (int, map[string]any) {
				b, _ := json.Marshal(body)
				req := httptest.NewRequest(method, path, bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")
				if token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				resp, err := app.Test(req, -1)
				if err != nil {
					t.Fatal(err)
				}
				var out map[string]any
				json.NewDecoder(resp.Body).Decode(&out)
				return resp.StatusCode, out
			}

			// Primeiro admin, criado pelo próprio endpoint.
			st, out := call("POST", "/users", map[string]any{"name": "Admin", "email": "Admin@X.com ", "password": "segredo1", "role": "admin"}, "")
			if st != 201 {
				t.Fatalf("criar admin: %d %v", st, out)
			}
			adminID := uint(out["id"].(float64))
			tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"id": float64(adminID), "role": "admin", "account_type": "user",
			}).SignedString([]byte(secret))

			st, out = call("POST", "/users", map[string]any{"name": "Caixa", "email": "caixa@x.com", "password": "segredo1", "role": "restaurant", "establishment_id": 1, "phone": "11999990000"}, "")
			if st != 201 {
				t.Fatalf("criar equipe da loja: %d %v", st, out)
			}
			staffID := uint(out["id"].(float64))
			var u struct {
				Email  string
				Role   string
				Status string
				EstID  uint `gorm:"column:establishment_id"`
			}
			db.Raw("SELECT email, role::text AS role, status, establishment_id FROM users WHERE id = ?", staffID).Scan(&u)
			wantRole := "user"
			if legacy {
				wantRole = "restaurant"
			}
			if u.Role != wantRole || u.Status != "active" || u.EstID != 1 {
				t.Fatalf("usuário gravado errado: %+v (role esperado %s)", u, wantRole)
			}
			var adminEmail string
			db.Raw("SELECT email FROM users WHERE id = ?", adminID).Scan(&adminEmail)
			if adminEmail != "admin@x.com" {
				t.Fatalf("e-mail não normalizado: %q", adminEmail)
			}

			if st, _ = call("POST", "/users", map[string]any{"name": "C", "email": "c@x.com", "password": "segredo1", "role": "client"}, ""); st != 400 {
				t.Fatalf("papel client devia dar 400, deu %d", st)
			}
			if st, _ = call("POST", "/users", map[string]any{"name": "D", "email": "caixa@x.com", "password": "segredo1"}, ""); st != 409 {
				t.Fatalf("e-mail repetido devia dar 409, deu %d", st)
			}
			if st, _ = call("POST", "/users", map[string]any{"name": "E", "email": "e@x.com", "password": "segredo1", "establishment_id": 99}, ""); st != 400 {
				t.Fatalf("loja inexistente devia dar 400, deu %d", st)
			}

			// Admin redefine a senha: grava o hash e derruba as sessões.
			db.Create(&models.RefreshToken{UserID: staffID, Token: "tok-staff"})
			if st, out = call("PUT", "/users/"+itoa(staffID), map[string]any{"password": "novaSenha9"}, tok); st != 200 {
				t.Fatalf("redefinir senha: %d %v", st, out)
			}
			var hash string
			db.Raw("SELECT password FROM users WHERE id = ?", staffID).Scan(&hash)
			if bcrypt.CompareHashAndPassword([]byte(hash), []byte("novaSenha9")) != nil {
				t.Fatal("senha nova não foi gravada")
			}
			var revoked bool
			db.Raw("SELECT revoked FROM refresh_tokens WHERE token = 'tok-staff'").Scan(&revoked)
			if !revoked {
				t.Fatal("sessão antiga continuou valendo depois da troca de senha")
			}

			// Login não diferencia maiúsculas no e-mail (teclado do celular).
			if st, out = call("POST", "/users/login", map[string]any{"email": " CAIXA@x.com", "password": "novaSenha9"}, ""); st != 200 {
				t.Fatalf("login com e-mail em maiúsculas: %d %v", st, out)
			}
			if st, _ = call("POST", "/users/login", map[string]any{"email": "", "password": "segredo1"}, ""); st != 403 {
				t.Fatalf("login sem e-mail devia dar 403, deu %d", st)
			}

			// Único admin: não rebaixa nem exclui.
			if st, _ = call("PUT", "/users/"+itoa(adminID), map[string]any{"role": "restaurant"}, tok); st != 400 {
				t.Fatalf("rebaixar o único admin devia dar 400, deu %d", st)
			}
			if st, _ = call("DELETE", "/users/"+itoa(adminID), nil, tok); st != 400 {
				t.Fatalf("excluir o único admin devia dar 400, deu %d", st)
			}
			if st, _ = call("DELETE", "/users/"+itoa(staffID), nil, tok); st != 200 {
				t.Fatalf("excluir equipe: %d", st)
			}
		})
	}
}

func itoa(n uint) string {
	b, _ := json.Marshal(n)
	return string(b)
}
