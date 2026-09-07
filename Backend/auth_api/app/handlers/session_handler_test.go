// Package handlers - session_handler_test.go
// Testes unitarios da montagem do JSON de usuario exposto por
// /auth/session (login, refresh e GET) apos a migracao do frontend web
// de token em localStorage para cookie HttpOnly.
package handlers

import (
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func TestSessionUserJSON_SemEstabelecimento(t *testing.T) {
	user := models.User{
		Name:      "Ana",
		Email:     "ana@fuudelivery.com",
		Phone:     "11999999999",
		Role:      "user",
		AvatarURL: "https://example.com/avatar.png",
	}

	got := sessionUserJSON(&user, nil)

	if got["name"] != "Ana" || got["email"] != "ana@fuudelivery.com" || got["role"] != "user" {
		t.Errorf("campos basicos incorretos: %+v", got)
	}
	if got["avatar_url"] != "https://example.com/avatar.png" {
		t.Errorf("avatar_url incorreto: %+v", got)
	}
	if _, hasEstablishment := got["establishment"]; hasEstablishment {
		t.Errorf("nao deveria ter campo 'establishment' sem estabelecimento: %+v", got)
	}
	if _, hasName := got["establishment_name"]; hasName {
		t.Errorf("nao deveria ter campo 'establishment_name' sem estabelecimento: %+v", got)
	}
}

func TestSessionUserJSON_ComEstabelecimento(t *testing.T) {
	user := models.User{Name: "Carlos", Email: "carlos@fuudelivery.com", Role: "admin", EstablishmentID: 42}
	establishment := models.Establishment{Name: "Restaurante do Carlos"}
	establishment.ID = 42

	got := sessionUserJSON(&user, &establishment)

	if got["establishment_name"] != "Restaurante do Carlos" {
		t.Errorf("establishment_name incorreto: %+v", got)
	}

	nested, ok := got["establishment"].(fiber.Map)
	if !ok {
		t.Fatalf("establishment deveria ser um objeto aninhado, veio: %T", got["establishment"])
	}
	if nested["id"] != uint(42) || nested["name"] != "Restaurante do Carlos" {
		t.Errorf("establishment aninhado incorreto: %+v", nested)
	}
}
