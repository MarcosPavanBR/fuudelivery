package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/delivery_api/app/services"
	"github.com/gofiber/fiber/v2"
)

func newDispatchTestApp() *fiber.App {
	return fiber.New()
}

func newTestDispatchHandler() *DispatchHandler {
	store := services.NewCourierStore()
	matching := services.NewMatchingEngine(store, &services.DefaultZoneResolver{})
	return NewDispatchHandler(store, matching)
}

// === UpdateLocation ===

func TestUpdateLocation_InvalidPayload(t *testing.T) {
	h := newTestDispatchHandler()
	app := newDispatchTestApp()
	app.Post("/dispatch/location", h.UpdateLocation)

	req := httptest.NewRequest("POST", "/dispatch/location", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestUpdateLocation_NoAuth(t *testing.T) {
	h := newTestDispatchHandler()
	app := newDispatchTestApp()
	app.Post("/dispatch/location", h.UpdateLocation)

	body := `{"lat":-23.5,"lng":-46.6,"name":"Test"}`
	req := httptest.NewRequest("POST", "/dispatch/location", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}

// === SetCourierStatus ===

func TestSetCourierStatus_InvalidPayload(t *testing.T) {
	h := newTestDispatchHandler()
	app := newDispatchTestApp()
	app.Post("/dispatch/status", h.SetCourierStatus)

	req := httptest.NewRequest("POST", "/dispatch/status", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestSetCourierStatus_NoAuth(t *testing.T) {
	h := newTestDispatchHandler()
	app := newDispatchTestApp()
	app.Post("/dispatch/status", h.SetCourierStatus)

	body := `{"status":"available"}`
	req := httptest.NewRequest("POST", "/dispatch/status", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}

// === TriggerDispatch ===

func TestTriggerDispatch_InvalidPayload(t *testing.T) {
	h := newTestDispatchHandler()
	app := newDispatchTestApp()
	app.Post("/dispatch/trigger", h.TriggerDispatch)

	req := httptest.NewRequest("POST", "/dispatch/trigger", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

// === GetDLQ ===

func TestGetDLQ_Success(t *testing.T) {
	h := newTestDispatchHandler()
	app := newDispatchTestApp()
	app.Get("/dispatch/dlq", h.GetDLQ)

	req := httptest.NewRequest("GET", "/dispatch/dlq", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("got %d, want 200", resp.StatusCode)
	}
}

// === GetDispatchStatus ===

func TestGetDispatchStatus_Success(t *testing.T) {
	h := newTestDispatchHandler()
	app := newDispatchTestApp()
	app.Get("/dispatch/status/:orderId", h.GetDispatchStatus)

	req := httptest.NewRequest("GET", "/dispatch/status/order-123", nil)
	resp, _ := app.Test(req)
	// Returns 404 or 200 depending on whether order exists
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		t.Errorf("got %d, want 200 or 404", resp.StatusCode)
	}
}

// === NearbyCouriers ===

func TestNearbyCouriers_Success(t *testing.T) {
	h := newTestDispatchHandler()
	app := newDispatchTestApp()
	app.Get("/dispatch/nearby", h.NearbyCouriers)

	req := httptest.NewRequest("GET", "/dispatch/nearby?lat=-23.5&lng=-46.6&radius=5", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("got %d, want 200", resp.StatusCode)
	}
}

// === Ping ===

func TestPing(t *testing.T) {
	app := newDispatchTestApp()
	app.Get("/ping", Ping)

	req := httptest.NewRequest("GET", "/ping", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("got %d, want 200", resp.StatusCode)
	}
}
