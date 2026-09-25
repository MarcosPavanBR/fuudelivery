package upload

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const uploadTestSecret = "segredo-teste-upload"

func canUploadWith(t *testing.T, claims jwt.MapClaims, entity, entityID string) bool {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(uploadTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	var got bool
	app := fiber.New()
	app.Post("/x", func(c *fiber.Ctx) error {
		got = canUploadFor(c, entity, entityID)
		return nil
	})
	req := httptest.NewRequest("POST", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}
	return got
}

// Um cliente nunca envia imagem de produto — nem quando o id dele coincide
// com o de um usuário de loja (a checagem antiga buscava o id em users).
func TestCanUploadFor_ClienteNuncaPassa(t *testing.T) {
	t.Setenv("JWT_SECRET", uploadTestSecret)
	if canUploadWith(t, jwt.MapClaims{"id": 5, "role": "client"}, "products", "10") {
		t.Fatal("cliente não pode enviar imagem de produto")
	}
	if !canUploadWith(t, jwt.MapClaims{"id": 1, "role": "admin"}, "products", "10") {
		t.Fatal("admin deveria passar")
	}
}

func TestEntityBelongsTo_Postgres(t *testing.T) {
	uri := os.Getenv("POSTGRES_TEST_URI")
	if uri == "" {
		t.Skip("POSTGRES_TEST_URI não definido")
	}
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range []string{"products", "categories", "additionals"} {
		db.Exec("DROP TABLE IF EXISTS " + tb + " CASCADE")
		if err := db.Exec("CREATE TABLE " + tb + " (id BIGINT PRIMARY KEY, establishment_id BIGINT)").Error; err != nil {
			t.Fatal(err)
		}
		db.Exec("INSERT INTO " + tb + " VALUES (10, 42)")
	}
	for _, tb := range []string{"products", "categories", "additionals"} {
		if !entityBelongsTo(db, tb, "10", 42) {
			t.Errorf("%s 10 é da loja 42", tb)
		}
		if entityBelongsTo(db, tb, "10", 41) {
			t.Errorf("%s 10 não é da loja 41", tb)
		}
	}
	if entityBelongsTo(db, "users", "10", 42) {
		t.Error("entidade fora da lista não pode passar")
	}
}
