package upload

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
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
