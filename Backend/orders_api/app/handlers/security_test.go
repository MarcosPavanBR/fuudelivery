package handlers

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

// Tests that work WITHOUT database (validation before DB access)

func TestValidateCoupon_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Post("/coupons/validate", ValidateCoupon)
	req := httptest.NewRequest("POST", "/coupons/validate", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateReview_InvalidPayload(t *testing.T) {
	app := newTestApp()
	app.Post("/reviews", CreateReview)
	req := httptest.NewRequest("POST", "/reviews", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateReview_RatingTooLow(t *testing.T) {
	app := newTestApp()
	app.Post("/reviews", CreateReview)
	body := `{"order_id":"order-123","user_phone":"+5511999999999","rating":0}`
	req := httptest.NewRequest("POST", "/reviews", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestCreateReview_RatingTooHigh(t *testing.T) {
	app := newTestApp()
	app.Post("/reviews", CreateReview)
	body := `{"order_id":"order-123","user_phone":"+5511999999999","rating":6}`
	req := httptest.NewRequest("POST", "/reviews", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestGetDeliveryByEstablishmentID_InvalidID(t *testing.T) {
	app := newTestApp()
	app.Get("/delivery/value/:establishmentId", GetDeliveryByEstablishmentID)
	req := httptest.NewRequest("GET", "/delivery/value/abc", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}
