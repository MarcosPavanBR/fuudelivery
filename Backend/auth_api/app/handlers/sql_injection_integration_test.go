//go:build integration

package handlers

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GET /establishments/:id é pública. O :id ia cru para First(&est, id), e o
// GORM trata string não numérica como SQL: "0=0" devolvia a primeira loja e
// qualquer condição sem espaço (subconsultas com parênteses) era executada no
// Postgres — injeção de SQL sem login.
func TestGetEstablishments_IDComSQLNaoExecuta(t *testing.T) {
	uri := requirePostgres(t)
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Migrator().DropTable(&models.Establishment{})
	if err := db.AutoMigrate(&models.Establishment{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	defer func() { models.DB = prev }()
	if err := db.Create(&models.Establishment{ID: 1, Name: "Loja Um", PaymentWalletID: "carteira-privada"}).Error; err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Get("/establishments/:id", GetEstablishments)
	get := func(path string) (int, string) {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	for _, id := range []string{"0=0", "(select(count(*))from(establishments))>0", "1)or(1=1"} {
		code, body := get("/establishments/" + id)
		if code != 400 || strings.Contains(body, "Loja Um") {
			t.Errorf("/establishments/%s: got %d %s, want 400 sem dados", id, code, body)
		}
	}
	code, body := get("/establishments/1")
	if code != 200 || !strings.Contains(body, "Loja Um") {
		t.Fatalf("id válido: got %d %s", code, body)
	}
	// A rota é pública: a carteira de recebimento da loja não sai nela.
	if strings.Contains(body, "carteira-privada") {
		t.Fatalf("GET público expôs a carteira da loja: %s", body)
	}
}
