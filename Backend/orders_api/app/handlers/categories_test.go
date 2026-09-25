package handlers

import (
	"io"

	"bytes"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

// TestGetCategoriesWithProducts_Cardapio: o cardápio (AppComida/WebRestaurant)
// devolve as categorias com os produtos. O check de erro fazia
// `Association(...).Find(...).Error` — method value de um error, nunca nil —
// e o endpoint respondia 500 (ou panic) para toda loja com categoria.
func TestGetCategoriesWithProducts_Cardapio(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.Additional{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	defer func() { models.DB = prev }()

	cat := models.Category{Name: "Lanches", EstablishmentID: 7}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatal(err)
	}
	prod := models.Product{Name: "X-Burger", EstablishmentID: 7}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&cat).Association("Products").Append(&prod); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	app.Get("/categories/product/:establishmentId", GetCategoriesWithProducts)
	resp, err := app.Test(httptest.NewRequest("GET", "/categories/product/7", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("got %d (%s), want 200", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("X-Burger")) {
		t.Fatalf("cardápio sem o produto: %s", body)
	}
}
