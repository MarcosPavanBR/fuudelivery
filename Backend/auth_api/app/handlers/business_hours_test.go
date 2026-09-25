package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

const hoursTestSecret = "segredo-de-teste-horarios"

func hoursToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(hoursTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func postHours(t *testing.T, path, token, body string, h fiber.Handler) int {
	t.Helper()
	app := fiber.New()
	app.Post(path, h)
	req := httptest.NewRequest("POST", path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

// IDOR: um cliente (sem establishment_id) ou o dono de OUTRA loja não pode
// mexer nos horários — antes, qualquer usuário logado fechava qualquer loja.
func TestBusinessHours_BarraQuemNaoEDono(t *testing.T) {
	t.Setenv("JWT_SECRET", hoursTestSecret)
	body := `{"establishment_id":42,"day_of_week":1,"is_open":false}`
	bulk := `[{"establishment_id":41,"day_of_week":1,"is_open":true},{"establishment_id":42,"day_of_week":1,"is_open":false}]`

	cliente := hoursToken(t, jwt.MapClaims{"id": 9, "role": "client"})
	outraLoja := hoursToken(t, jwt.MapClaims{"id": 8, "role": "restaurant", "establishment_id": 41})

	for nome, tok := range map[string]string{"cliente": cliente, "outra loja": outraLoja} {
		if got := postHours(t, "/establishments/hours", tok, body, UpsertBusinessHours); got != 403 {
			t.Errorf("%s: upsert got %d, want 403", nome, got)
		}
	}
	// No lote, um item alheio barra o lote inteiro (antes de gravar qualquer coisa).
	if got := postHours(t, "/establishments/hours/bulk", outraLoja, bulk, BulkUpdateBusinessHours); got != 403 {
		t.Errorf("bulk com item alheio: got %d, want 403", got)
	}
}

func TestBusinessHours_ValidaDiaDaSemana(t *testing.T) {
	t.Setenv("JWT_SECRET", hoursTestSecret)
	dono := hoursToken(t, jwt.MapClaims{"id": 7, "role": "restaurant", "establishment_id": 42})
	if got := postHours(t, "/establishments/hours", dono, `{"establishment_id":42,"day_of_week":9}`, UpsertBusinessHours); got != 400 {
		t.Errorf("day_of_week=9: got %d, want 400", got)
	}
}

// O editor de horários do WebRestaurant lê day_of_week/is_open/open_time. Sem
// tags json o GET devolvia DayOfWeek/IsOpen e o painel nunca mostrava os
// horários salvos.
func TestBusinessHours_JSONEmSnakeCase(t *testing.T) {
	b, err := json.Marshal(models.BusinessHours{DayOfWeek: 2, IsOpen: false, OpenTime: "09:00"})
	if err != nil {
		t.Fatal(err)
	}
	for _, campo := range []string{`"day_of_week":2`, `"is_open":false`, `"open_time":"09:00"`, `"establishment_id"`} {
		if !bytes.Contains(b, []byte(campo)) {
			t.Errorf("JSON sem %s: %s", campo, b)
		}
	}
}
