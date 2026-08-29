package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect test database")
	}
	db.AutoMigrate(&models.Product{}, &models.Category{}, &authModels.Establishment{})
	return db
}

func createTestToken(role string, establishmentID int64) string {
	claims := jwt.MapClaims{
		"id":               float64(42),
		"name":             "Test User",
		"email":            "test@example.com",
		"role":             role,
		"establishment_id": establishmentID,
		"exp":              time.Now().UTC().Add(time.Hour * 24 * 7).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test-secret-key-for-ci"))
	return tokenString
}

func createExpiredToken(role string, establishmentID int64) string {
	claims := jwt.MapClaims{
		"id":               float64(42),
		"name":             "Test User",
		"email":            "test@example.com",
		"role":             role,
		"establishment_id": establishmentID,
		"exp":              time.Now().UTC().Add(-time.Hour).Unix(), // expired 1h ago
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test-secret-key-for-ci"))
	return tokenString
}

// --- HTTP-level ownership tests (using app.Test for real request/response) ---

func TestCreateProduct_NoAuth(t *testing.T) {
	db := setupTestDB()
	models.DB = db

	app := fiber.New()
	app.Post("/products", CreateProduct)

	// No Authorization header → should be rejected
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{"name":"Pizza","price":25.0,"establishment_id":1}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Contains(t, []int{fiber.StatusUnauthorized, fiber.StatusForbidden}, resp.StatusCode)
}

func TestCreateProduct_ExpiredToken(t *testing.T) {
	db := setupTestDB()
	models.DB = db

	os.Setenv("JWT_SECRET", "test-secret-key-for-ci")
	defer os.Unsetenv("JWT_SECRET")

	app := fiber.New()
	app.Post("/products", CreateProduct)

	token := createExpiredToken("admin", 1)
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{"name":"Pizza","price":25.0,"establishment_id":1}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Contains(t, []int{fiber.StatusUnauthorized, fiber.StatusForbidden}, resp.StatusCode)
}

func TestCreateProduct_ClientRole(t *testing.T) {
	db := setupTestDB()
	models.DB = db

	os.Setenv("JWT_SECRET", "test-secret-key-for-ci")
	defer os.Unsetenv("JWT_SECRET")

	app := fiber.New()
	app.Post("/products", CreateProduct)

	// Client role should NOT be able to create products
	token := createTestToken("client", 1)
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{"name":"Pizza","price":25.0,"establishment_id":1}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestGetProducts_EmptyEstablishment(t *testing.T) {
	db := setupTestDB()
	models.DB = db

	app := fiber.New()
	app.Get("/establishments/:establishmentId/products", GetByEstablishmentId)

	// No auth needed for GET (permissive), returns empty array
	req := httptest.NewRequest(http.MethodGet, "/establishments/999/products", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// --- canActOnEstablishment unit tests ---

func TestCanActOnEstablishment(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		estID      int64
		tokenEstID int64
		expected   bool
	}{
		{"admin can access any", "admin", 1, 2, true},
		{"restaurant owns establishment", "restaurant", 1, 1, true},
		{"restaurant does not own", "restaurant", 1, 2, false},
		{"client cannot access", "client", 1, 1, false},
		{"delivery cannot access", "delivery", 1, 1, false},
		{"admin with zero est ID", "admin", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("JWT_SECRET", "test-secret-key-for-ci")
			defer os.Unsetenv("JWT_SECRET")

			app := fiber.New()
			var result bool
			app.Get("/test", func(c *fiber.Ctx) error {
				result = canActOnEstablishment(c, tt.estID)
				return c.SendStatus(200)
			})

			token := createTestToken(tt.role, tt.tokenEstID)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			_, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCanActOnEstablishment_NoToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-ci")
	defer os.Unsetenv("JWT_SECRET")

	app := fiber.New()
	var result bool
	app.Get("/test", func(c *fiber.Ctx) error {
		result = canActOnEstablishment(c, 1)
		return c.SendStatus(200)
	})

	// No token → should return false (not authenticated)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_, err := app.Test(req)
	assert.NoError(t, err)
	assert.False(t, result)
}
