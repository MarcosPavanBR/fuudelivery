//go:build integration

package handlers

import (
	"os"
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// requirePostgres devolve a URI do Postgres de teste. Em CI a ausência é
// erro de configuração do workflow e FALHA (senão o job ficaria verde sem ter
// rodado nada); fora do CI, pula.
func requirePostgres(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("POSTGRES_TEST_URI")
	if uri == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("POSTGRES_TEST_URI ausente em CI — o job precisa provê-la, senão este teste não roda")
		}
		t.Skip("POSTGRES_TEST_URI não definida (rode com um Postgres local)")
	}
	return uri
}

// is_open=false precisa chegar ao banco — na criação (IsOpen tem default:true)
// e na atualização (Updates com struct pula valores zero). Roda contra
// Postgres quando POSTGRES_TEST_URI está definido.
func TestBusinessHours_GravaDiaFechado(t *testing.T) {
	uri := requirePostgres(t)
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
