package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"

)

func TestCreateAdditional_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Post("/additional", CreateAdditional)
	req := httptest.NewRequest("POST", "/additional", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateAdditional_NoAuth(t *testing.T) {
	app := newTestApp()
	app.Post("/additional", CreateAdditional)
	body := `{"name":"Cheese","price":5.0,"establishmentId":1}`
	req := httptest.NewRequest("POST", "/additional", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 403 {
		t.Errorf("got %d, want 403", resp.StatusCode)
	}
}

func TestCreateProductToAdditional_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Post("/additional/product", CreateProductToAdditional)
	req := httptest.NewRequest("POST", "/additional/product", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateProductCategorie_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Post("/categories/product", CreateProductCategorie)
	req := httptest.NewRequest("POST", "/categories/product", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}
