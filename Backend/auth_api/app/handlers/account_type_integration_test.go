//go:build integration

package handlers

import (
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// UpdateEstablishment carregava o id do token na tabela users: o cliente 5
// herdava a loja do usuário 5 e editava o estabelecimento dela.
func TestUpdateEstablishment_ClienteComIDDoDonoNaoEdita(t *testing.T) {
	uri := requirePostgres(t)
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Migrator().DropTable(&models.User{}, &models.Establishment{})
	if err := db.AutoMigrate(&models.Establishment{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	prev := models.DB
	models.DB = db
	defer func() { models.DB = prev }()
	t.Setenv("JWT_SECRET", hoursTestSecret)

	if err := db.Create(&models.Establishment{ID: 1, Name: "Loja do usuário 5"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.User{ID: 5, Name: "Dono", Email: "dono@x.com", Password: "x", Role: "user", EstablishmentID: 1}).Error; err != nil {
		t.Fatal(err)
	}

	body := `{"establishment":{"name":"Tomada pelo cliente 5"}}`
	cliente := hoursToken(t, jwt.MapClaims{"id": 5, "role": "client", "account_type": "client"})
	if got := acctReq(t, UpdateEstablishment, "PUT", "/establishments/:id", "/establishments/1", cliente, body); got != 403 {
		t.Fatalf("cliente 5: got %d, want 403", got)
	}
	var est models.Establishment
	db.First(&est, 1)
	if est.Name != "Loja do usuário 5" {
		t.Fatalf("o cliente alterou a loja: %q", est.Name)
	}

	dono := hoursToken(t, jwt.MapClaims{"id": 5, "role": "user", "account_type": "user", "establishment_id": 1})
	if got := acctReq(t, UpdateEstablishment, "PUT", "/establishments/:id", "/establishments/1", dono, `{"establishment":{"name":"Nome novo"}}`); got != 200 {
		t.Fatalf("dono: got %d, want 200", got)
	}
}
