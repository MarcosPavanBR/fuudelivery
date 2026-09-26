//go:build integration

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// O cadastro de restaurante usava SQL só para o schema de produção (enum
// "Role", colunas "createdAt"/"updatedAt"). Em banco novo (AutoMigrate) a
// consulta '"Role"'::regtype dava erro, abortava a transação e o cadastro
// falhava sempre. Roda nos DOIS schemas.
func TestRegisterEstablishment_NosDoisSchemas(t *testing.T) {
	uri := requirePostgres(t)
	t.Setenv("JWT_SECRET", "register-schema-secret")

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
			for _, tbl := range []string{"business_hours", "establishments", "users"} {
				db.Exec("DROP TABLE IF EXISTS " + tbl + " CASCADE")
			}
			db.Exec(`DROP TYPE IF EXISTS "Role"`)
			if err := db.AutoMigrate(&models.User{}, &models.Establishment{}, &models.BusinessHours{}); err != nil {
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

			app := fiber.New()
			app.Post("/establishments/register", RegisterEstablishment)
			app.Get("/users", ListAllUsers)
			body, _ := json.Marshal(map[string]string{
				"name": "Loja Teste", "owner_name": "Dono", "email": "dono@teste.local",
				"password": "segredo123", "phone": "+5511900000001", "address": "Rua A, 1",
				"opening_time": "08:00", "closing_time": "22:00",
			})
			req := httptest.NewRequest("POST", "/establishments/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req, -1)
			if resp.StatusCode != 201 && resp.StatusCode != 200 {
				var out map[string]any
				json.NewDecoder(resp.Body).Decode(&out)
				t.Fatalf("cadastro: status %d %v", resp.StatusCode, out)
			}
			var u struct {
				Role  string
				Phone string
				EstID uint `gorm:"column:establishment_id"`
			}
			db.Raw("SELECT role::text AS role, phone, establishment_id FROM users WHERE email = 'dono@teste.local'").Scan(&u)
			wantRole := "user"
			if legacy {
				wantRole = "restaurant"
			}
			if u.Role != wantRole || u.Phone != "+5511900000001" || u.EstID == 0 {
				t.Fatalf("usuário gravado errado: %+v (role esperado %s)", u, wantRole)
			}
			var hours int64
			db.Model(&models.BusinessHours{}).Where("establishment_id = ?", u.EstID).Count(&hours)
			if hours != 7 {
				t.Fatalf("grade de horários: %d dias, esperava 7", hours)
			}

			// A lista de usuários do admin também não pode quebrar.
			lr, _ := app.Test(httptest.NewRequest("GET", "/users", nil), -1)
			if lr.StatusCode != 200 {
				t.Fatalf("lista de usuários: status %d", lr.StatusCode)
			}
		})
	}
}
