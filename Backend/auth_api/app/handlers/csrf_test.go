package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestGetCSRFToken(t *testing.T) {
	app := fiber.New()
	app.Get("/csrf-token", GetCSRFToken)

	req := httptest.NewRequest(http.MethodGet, "/csrf-token", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Check cookie is set
	cookie := resp.Header.Get("Set-Cookie")
	if !strings.Contains(cookie, "csrf_token=") {
		t.Errorf("expected Set-Cookie header with csrf_token, got: %s", cookie)
	}
	if !strings.Contains(cookie, "HttpOnly") {
		t.Errorf("expected HttpOnly to be absent (frontend needs JS access), got: %s", cookie)
	}
	if !strings.Contains(cookie, "secure") {
		t.Errorf("expected secure flag in cookie, got: %s", cookie)
	}
}

func TestGetCSRFTokenCookieLength(t *testing.T) {
	app := fiber.New()
	app.Get("/csrf-token", GetCSRFToken)

	req := httptest.NewRequest(http.MethodGet, "/csrf-token", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	cookie := resp.Header.Get("Set-Cookie")
	// Extract token value
	parts := strings.Split(cookie, ";")
	var tokenValue string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "csrf_token=") {
			tokenValue = strings.TrimPrefix(part, "csrf_token=")
			break
		}
	}

	if len(tokenValue) < 32 {
		t.Errorf("expected csrf_token length >= 32, got %d", len(tokenValue))
	}
}
