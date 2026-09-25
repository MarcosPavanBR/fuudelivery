package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func doIDOR(t *testing.T, app *fiber.App, path, token, body string) int {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

// POST /delivery define a taxa de entrega da loja: dinheiro. Antes, qualquer
// usuário logado mudava o frete de qualquer loja.
func TestInsertDelivery_SoDonoDaLoja(t *testing.T) {
	t.Setenv("JWT_SECRET", couponAuthzSecret)
	app := fiber.New()
	app.Post("/delivery", InsertDelivery)
	body := `{"establishmentId":42,"fixedTaxa":0,"perKm":0}`

	if got := doIDOR(t, app, "/delivery", tokenFor(t, "client", 0, "+5511999990000"), body); got != 403 {
		t.Errorf("cliente: got %d, want 403", got)
	}
	if got := doIDOR(t, app, "/delivery", tokenFor(t, "restaurant", 41, ""), body); got != 403 {
		t.Errorf("outra loja: got %d, want 403", got)
	}
	if got := doIDOR(t, app, "/delivery", tokenFor(t, "restaurant", 42, ""), `{"establishmentId":42,"fixedTaxa":-5,"perKm":1}`); got != 400 {
		t.Errorf("taxa negativa: got %d, want 400", got)
	}
}

// POST /orders/schedule: só o cliente dono do pedido ou a loja dele.
func TestScheduleOrder_SoDonoOuLoja(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.OrderDocument{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = prev })
	t.Setenv("JWT_SECRET", couponAuthzSecret)

	if err := db.Create(&models.OrderDocument{
		LegacyID: "abc123", EstablishmentID: 42, UserPhone: "+5511911112222", Payload: []byte(`{}`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Post("/orders/schedule", ScheduleOrder)
	futuro := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	body := `{"order_id":"abc123","scheduled_at":"` + futuro + `"}`

	if got := doIDOR(t, app, "/orders/schedule", tokenFor(t, "client", 0, "+5511900000000"), body); got != 403 {
		t.Errorf("outro cliente: got %d, want 403", got)
	}
	if got := doIDOR(t, app, "/orders/schedule", tokenFor(t, "restaurant", 41, ""), body); got != 403 {
		t.Errorf("outra loja: got %d, want 403", got)
	}
	passado := `{"order_id":"abc123","scheduled_at":"2020-01-01T10:00:00Z"}`
	if got := doIDOR(t, app, "/orders/schedule", tokenFor(t, "client", 0, "+5511911112222"), passado); got != 400 {
		t.Errorf("horário no passado: got %d, want 400", got)
	}
	if got := doIDOR(t, app, "/orders/schedule", tokenFor(t, "client", 0, "+5511911112222"), body); got != 200 {
		t.Errorf("dono do pedido: got %d, want 200", got)
	}
	if got := doIDOR(t, app, "/orders/schedule", tokenFor(t, "restaurant", 42, ""), body); got != 200 {
		t.Errorf("loja do pedido: got %d, want 200", got)
	}
}
