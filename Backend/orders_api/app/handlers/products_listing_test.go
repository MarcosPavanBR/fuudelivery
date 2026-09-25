package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// O cardápio de uma loja trazia os produtos de todas: um segundo Find sem
// filtro (só para o Preload das categorias) recarregava a tabela inteira.
func TestGetByEstablishmentId_SoProdutosDaLoja(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.Additional{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = prev })

	cat := models.Category{Name: "Pizzas", EstablishmentID: 1}
	db.Create(&cat)
	db.Create(&models.Product{Name: "Da loja 1", Price: 10, EstablishmentID: 1, Categories: []models.Category{cat}})
	db.Create(&models.Product{Name: "Da loja 2", Price: 10, EstablishmentID: 2})

	app := fiber.New()
	app.Get("/products/:establishmentId", GetByEstablishmentId)
	resp, err := app.Test(httptest.NewRequest("GET", "/products/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	var out []models.Product
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Name != "Da loja 1" {
		t.Fatalf("cardápio da loja 1 veio com %d produtos: %+v", len(out), out)
	}
	if len(out[0].Categories) != 1 {
		t.Fatalf("categorias não vieram: %+v", out[0].Categories)
	}
}
