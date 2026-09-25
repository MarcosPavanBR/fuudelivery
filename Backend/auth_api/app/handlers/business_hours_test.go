package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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

// is_open=false precisa chegar ao banco — na criação (IsOpen tem default:true)
// e na atualização (Updates com struct pula valores zero). Roda contra
// Postgres quando POSTGRES_TEST_URI está definido.
func TestBusinessHours_GravaDiaFechado(t *testing.T) {
	uri := os.Getenv("POSTGRES_TEST_URI")
	if uri == "" {
		t.Skip("POSTGRES_TEST_URI não definido")
	}
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Migrator().DropTable(&models.BusinessHours{})
	if err := db.AutoMigrate(&models.BusinessHours{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	defer func() { models.DB = prev }()

	t.Setenv("JWT_SECRET", hoursTestSecret)
	dono := hoursToken(t, jwt.MapClaims{"id": 7, "role": "restaurant", "establishment_id": 42})
	load := func() models.BusinessHours {
		var h models.BusinessHours
		if err := db.Where("establishment_id = 42 AND day_of_week = 1").First(&h).Error; err != nil {
			t.Fatal(err)
		}
		return h
	}

	// Criação já fechada.
	if got := postHours(t, "/establishments/hours", dono, `{"establishment_id":42,"day_of_week":1,"is_open":false}`, UpsertBusinessHours); got != 200 {
		t.Fatalf("criar: got %d", got)
	}
	if load().IsOpen {
		t.Fatal("dia criado como fechado ficou aberto")
	}

	// Abre com intervalo, depois fecha e limpa o intervalo.
	postHours(t, "/establishments/hours", dono, `{"establishment_id":42,"day_of_week":1,"is_open":true,"open_time":"10:00","close_time":"22:00","break_start_time":"15:00","break_end_time":"17:00"}`, UpsertBusinessHours)
	if h := load(); !h.IsOpen || h.BreakStartTime != "15:00" {
		t.Fatalf("abrir: %+v", h)
	}
	if got := postHours(t, "/establishments/hours/bulk", dono, `[{"establishment_id":42,"day_of_week":1,"is_open":false,"open_time":"10:00","close_time":"22:00"}]`, BulkUpdateBusinessHours); got != 200 {
		t.Fatalf("bulk: got %d", got)
	}
	if h := load(); h.IsOpen || h.BreakStartTime != "" {
		t.Fatalf("fechar pelo bulk não gravou: %+v", h)
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
