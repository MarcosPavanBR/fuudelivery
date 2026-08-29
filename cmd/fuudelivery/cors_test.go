package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/stretchr/testify/assert"
)

func TestCors_MultipleAllowedOrigins(t *testing.T) {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://fuudelivery-web.onrender.com,https://fuudelivery-admin-lv7f.onrender.com",
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token",
	}))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	origins := []string{
		"https://fuudelivery-web.onrender.com",
		"https://fuudelivery-admin-lv7f.onrender.com",
	}

	for _, origin := range origins {
		t.Run("origin="+origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", origin)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			// The response should include Access-Control-Allow-Origin
			acao := resp.Header.Get("Access-Control-Allow-Origin")
			assert.Equal(t, origin, acao)
		})
	}
}

func TestCors_CredentialsAllowed(t *testing.T) {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://fuudelivery-web.onrender.com",
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token",
	}))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://fuudelivery-web.onrender.com")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Credentials must be true for HttpOnly cookies to work cross-origin
	acac := resp.Header.Get("Access-Control-Allow-Credentials")
	assert.Equal(t, "true", acac)
}

func TestCors_AllowedMethods(t *testing.T) {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://fuudelivery-web.onrender.com",
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token",
	}))
	app.All("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for _, method := range methods {
		t.Run("method="+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Origin", "https://fuudelivery-web.onrender.com")
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestCors_CSRFHeaderAllowed(t *testing.T) {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://fuudelivery-web.onrender.com",
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token",
	}))
	app.Post("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Preflight OPTIONS request must allow X-CSRF-Token header
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://fuudelivery-web.onrender.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-CSRF-Token,Content-Type")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Fiber CORS returns 204 for preflight
	assert.Contains(t, []int{http.StatusNoContent, http.StatusOK}, resp.StatusCode)
	acao := resp.Header.Get("Access-Control-Allow-Headers")
	assert.Contains(t, acao, "X-CSRF-Token")
}

func TestCors_UntrustedOrigin(t *testing.T) {
	// Set env so the app starts without crashing
	os.Setenv("ALLOWED_ORIGINS", "https://fuudelivery-web.onrender.com")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://fuudelivery-web.onrender.com",
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token",
	}))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Request from an untrusted origin should NOT get CORS headers
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://evil-site.com")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// The response may be 200 but should NOT have the evil origin in ACAO
	acao := resp.Header.Get("Access-Control-Allow-Origin")
	assert.NotEqual(t, "https://evil-site.com", acao)
}

func TestCors_HealthEndpointAccessible(t *testing.T) {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://fuudelivery-web.onrender.com",
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token",
	}))
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://fuudelivery-web.onrender.com")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
