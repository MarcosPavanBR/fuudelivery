package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil
	}
	db.AutoMigrate(&models.Product{}, &models.Category{}, &models.Coupon{}, &models.CouponUsage{}, &models.LoyaltyPoints{})
	return db
}

func TestCreateProduct_OwnershipCheck(t *testing.T) {
	db := setupTestDB()
	if db == nil {
		t.Skip("SQLite unavailable (needs CGO)")
	}
	models.DB = db

	app := fiber.New()
	app.Post("/products", CreateProduct)

	body, _ := json.Marshal(map[string]interface{}{
		"name":             "Pizza",
		"price":            25.0,
		"establishment_id": 1,
	})
	req := httptest.NewRequest("POST", "/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	// Without valid JWT middleware, should get 403 or 401
	assert.True(t, resp.StatusCode >= 400, "Expected error status code")
}

func TestGetByEstablishmentId_ErrorCheck(t *testing.T) {
	db := setupTestDB()
	if db == nil {
		t.Skip("SQLite unavailable (needs CGO)")
	}
	models.DB = db

	app := fiber.New()
	app.Get("/establishments/:establishmentId/products", GetByEstablishmentId)

	req := httptest.NewRequest("GET", "/establishments/1/products", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}
