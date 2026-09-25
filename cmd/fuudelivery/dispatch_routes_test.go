package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	deliveryHandlers "github.com/carloshomar/fuudelivery/delivery_api/app/handlers"
	dispatchServices "github.com/carloshomar/fuudelivery/delivery_api/app/services"
	"github.com/gofiber/fiber/v2"
)

// /dispatch/nearby (GPS ao vivo dos entregadores) e /dispatch/trigger são só
// de admin. Antes, qualquer cliente logado passava.
func TestDispatchRoutes_NearbyETriggerSoAdmin(t *testing.T) {
	t.Setenv("JWT_SECRET", "segredo-teste-dispatch")
	prev := dispatchHandler
	dispatchHandler = deliveryHandlers.NewDispatchHandler(dispatchServices.NewCourierStore(), nil)
	defer func() { dispatchHandler = prev }()

	app := fiber.New()
	setupDispatchRoutes(app)

	cliente := createTestJWT(t, map[string]interface{}{"id": float64(1), "role": "client"})
	admin := createTestJWT(t, map[string]interface{}{"id": float64(2), "role": "admin"})

	do := func(method, path, token string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(`{"order_id":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}

	if got := do("GET", "/dispatch/nearby?lat=-23.5&lng=-46.6&radius=500", cliente); got != 403 {
		t.Errorf("nearby como cliente: got %d, want 403", got)
	}
	if got := do("POST", "/dispatch/trigger", cliente); got != 403 {
		t.Errorf("trigger como cliente: got %d, want 403", got)
	}
	if got := do("GET", "/dispatch/nearby?lat=-23.5&lng=-46.6", admin); got != 200 {
		t.Errorf("nearby como admin: got %d, want 200", got)
	}
}
