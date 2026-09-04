package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newAuthTestApp() *fiber.App {
	return fiber.New()
}

func TestListAllDeliveryMen_NoAuth(t *testing.T) {
	app := newAuthTestApp()
	app.Get("/delivery-man", ListAllDeliveryMen)
	req := httptest.NewRequest("GET", "/delivery-man", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 403 {
		t.Errorf("got %d, want 403", resp.StatusCode)
	}
}

func TestLoginDeliveryMan_InvalidPayload(t *testing.T) {
	app := newAuthTestApp()
	app.Post("/delivery-man/login", LoginDeliveryMan)
	req := httptest.NewRequest("POST", "/delivery-man/login", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateDeliveryMan_InvalidPayload(t *testing.T) {
	app := newAuthTestApp()
	app.Post("/delivery-man/register", CreateDeliveryMan)
	req := httptest.NewRequest("POST", "/delivery-man/register", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestUpdateDeliveryMan_InvalidPayload(t *testing.T) {
	app := newAuthTestApp()
	app.Put("/delivery-man/:id", UpdateDeliveryMan)
	req := httptest.NewRequest("PUT", "/delivery-man/1", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}
