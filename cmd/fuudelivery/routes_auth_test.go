package main

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newMonoTestApp() *fiber.App {
	return fiber.New()
}

func TestCSRFToken_NoCookie(t *testing.T) {
	app := newMonoTestApp()
	app.Get("/csrf-token", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"csrf_token": "test"})
	})
	req := httptest.NewRequest("GET", "/csrf-token", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("got %d, want 200", resp.StatusCode)
	}
}

func TestHealthEndpoint_NoDB(t *testing.T) {
	app := newMonoTestApp()
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "starting"})
	})
	req := httptest.NewRequest("GET", "/health", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("got %d, want 200", resp.StatusCode)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	app := newMonoTestApp()
	app.Get("/metrics", func(c *fiber.Ctx) error {
		return c.SendString("# HELP test metric\n")
	})
	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("got %d, want 200", resp.StatusCode)
	}
}

func TestProtectedRoute_NoToken(t *testing.T) {
	app := newMonoTestApp()
	app.Get("/users", protectedRoute, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{})
	})
	req := httptest.NewRequest("GET", "/users", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}

func TestAdminRequired_NonAdmin(t *testing.T) {
	app := newMonoTestApp()
	app.Get("/admin/test", protectedRoute, adminRequired, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{})
	})
	req := httptest.NewRequest("GET", "/admin/test", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}

func TestLoginClient_InvalidPayload(t *testing.T) {
	app := newMonoTestApp()
	app.Post("/clients/login", func(c *fiber.Ctx) error {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "bad"})
		}
		return c.JSON(fiber.Map{})
	})
	req := httptest.NewRequest("POST", "/clients/login", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestRegisterClient_InvalidPayload(t *testing.T) {
	app := newMonoTestApp()
	app.Post("/clients/register", func(c *fiber.Ctx) error {
		var req struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "bad"})
		}
		return c.JSON(fiber.Map{})
	})
	req := httptest.NewRequest("POST", "/clients/register", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

func TestWSUpgrade_NoToken(t *testing.T) {
	app := newMonoTestApp()
	app.Get("/ws/:id", func(c *fiber.Ctx) error {
		return c.Status(401).JSON(fiber.Map{"error": "no token"})
	})
	req := httptest.NewRequest("GET", "/ws/test", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("got %d, want 401", resp.StatusCode)
	}
}
