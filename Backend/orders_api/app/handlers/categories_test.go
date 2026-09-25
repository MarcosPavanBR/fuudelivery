package handlers

import (
	"fmt"
	"io"

	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	bacon := models.Additional{Name: "Bacon extra", EstablishmentID: 7}
	if err := db.Model(&prod).Association("Additional").Append(&bacon); err != nil {
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
	if !bytes.Contains(body, []byte("Bacon extra")) {
		t.Fatalf("cardápio sem os adicionais do produto: %s", body)
	}
}

// O cardápio fazia uma consulta de produtos POR categoria (N+1). Com
// Preload o número de consultas não cresce com o número de categorias.
func TestGetCategoriesWithProducts_ConsultasNaoCrescem(t *testing.T) {
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

	for i := 0; i < 12; i++ {
		cat := models.Category{Name: fmt.Sprintf("Cat %d", i), EstablishmentID: 7}
		db.Create(&cat)
		prod := models.Product{Name: fmt.Sprintf("Prod %d", i), EstablishmentID: 7}
		db.Create(&prod)
		db.Model(&cat).Association("Products").Append(&prod)
	}

	var consultas int
	db.Callback().Query().After("gorm:query").Register("conta_consultas", func(*gorm.DB) { consultas++ })

	app := newTestApp()
	app.Get("/categories/product/:establishmentId", GetCategoriesWithProducts)
	resp, err := app.Test(httptest.NewRequest("GET", "/categories/product/7", nil))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("status %v err %v", resp.StatusCode, err)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("Prod 11")) {
		t.Fatalf("cardápio incompleto: %s", body)
	}
	if consultas > 4 {
		t.Fatalf("%d consultas para 12 categorias: N+1", consultas)
	}
}
