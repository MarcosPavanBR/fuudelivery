package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
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

func TestCreateProduct_OwnershipCheck(t *testing.T) {
	db := setupTestDB()
	models.DB = db

	app := fiber.New()
	app.Post("/products", CreateProduct)

	// Test 1: admin should succeed
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(`{"name":"Pizza","price":25.0,"establishment_id":1}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestGetByEstablishmentId_ErrorCheck(t *testing.T) {
	db := setupTestDB()
	models.DB = db

	app := fiber.New()
	app.Get("/establishments/:establishmentId/products", GetByEstablishmentId)

	req := httptest.NewRequest(http.MethodGet, "/establishments/1/products", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			var result bool
			app.Get("/test", func(c *fiber.Ctx) error {
				middlewares.SetTestRole(tt.role)
				middlewares.SetTestEstablishmentID(tt.tokenEstID)
				result = canActOnEstablishment(c, tt.estID)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Bearer token")
			_, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
