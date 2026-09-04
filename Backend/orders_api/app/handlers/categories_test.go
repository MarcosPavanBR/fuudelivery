package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestCreateCategories_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Post("/categories/create", CreateCategories)
	req := httptest.NewRequest("POST", "/categories/create", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateCategories_NoAuth(t *testing.T) {
	app := newTestApp()
	app.Post("/categories/create", CreateCategories)
	body := `{"name":"Drinks","establishmentId":1}`
	req := httptest.NewRequest("POST", "/categories/create", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 403 {
		t.Errorf("got %d, want 403", resp.StatusCode)
	}
}

func TestUpdateCategory_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Put("/categories/:id", UpdateCategory)
	req := httptest.NewRequest("PUT", "/categories/1", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}
