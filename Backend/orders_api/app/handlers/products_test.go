package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newTestApp() *fiber.App {
	return fiber.New()
}

func TestPing(t *testing.T) {
	app := newTestApp()
	app.Get("/ping", Ping)
	req := httptest.NewRequest("GET", "/ping", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("Ping: got %d, want 200", resp.StatusCode)
	}
}

func TestCreateProduct_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Post("/products/create", CreateProduct)
	req := httptest.NewRequest("POST", "/products/create", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateProduct_NoAuth(t *testing.T) {
	app := newTestApp()
	app.Post("/products/create", CreateProduct)
	body := `{"name":"Pizza","price":35.5,"establishmentId":1}`
	req := httptest.NewRequest("POST", "/products/create", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 403 {
		t.Errorf("got %d, want 403", resp.StatusCode)
	}
}

func TestUpdateProduct_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Put("/products/update/:id", UpdateProduct)
	req := httptest.NewRequest("PUT", "/products/update/1", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateMultProducts_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Post("/products/multi-create", CreateMultProducts)
	req := httptest.NewRequest("POST", "/products/multi-create", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateMultProducts_NoAuth(t *testing.T) {
	app := newTestApp()
	app.Post("/products/multi-create", CreateMultProducts)
	body := `[{"name":"Pizza","price":35.5,"establishmentId":1}]`
	req := httptest.NewRequest("POST", "/products/multi-create", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 403 {
		t.Errorf("got %d, want 403", resp.StatusCode)
	}
}
