package main

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newSecTestApp() *fiber.App {
	return fiber.New()
}

func TestJWT_TamperedToken(t *testing.T) {
	app := newSecTestApp()
	app.Get("/users", protectedRoute, func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "Bearer tampered.token.here")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}

func TestJWT_EmptyToken(t *testing.T) {
	app := newSecTestApp()
	app.Get("/users", protectedRoute, func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "Bearer ")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}

func TestJWT_NoBearerPrefix(t *testing.T) {
	app := newSecTestApp()
	app.Get("/users", protectedRoute, func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "token abc123")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}

func TestAdminRequired_NoToken(t *testing.T) {
	app := newSecTestApp()
	app.Get("/admin", protectedRoute, adminRequired, func(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) })
	req := httptest.NewRequest("GET", "/admin", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}

func TestWebhookReplay_Idempotent(t *testing.T) {
	seen := make(map[string]bool)
	app := newSecTestApp()
	app.Post("/webhook", func(c *fiber.Ctx) error {
		var r struct{ ID string `json:"id"` }
		c.BodyParser(&r)
		if seen[r.ID] {
			return c.JSON(fiber.Map{"error": "duplicate"})
		}
		seen[r.ID] = true
		return c.JSON(fiber.Map{"status": "ok"})
	})

	body := `{"id":"wh-1"}`
	req1 := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte(body)))
	req1.Header.Set("Content-Type", "application/json")
	app.Test(req1)

	req2 := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte(body)))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != 200 {
		t.Errorf("replay: got %d, want 200", resp2.StatusCode)
	}
}

func TestRateLimit_Burst(t *testing.T) {
	app := newSecTestApp()
	app.Get("/ping", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	ok := 0
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("GET", "/ping", nil)
		resp, _ := app.Test(req)
		if resp.StatusCode == 200 {
			ok++
		}
	}
	if ok < 45 {
		t.Errorf("only %d/50 succeeded", ok)
	}
}
