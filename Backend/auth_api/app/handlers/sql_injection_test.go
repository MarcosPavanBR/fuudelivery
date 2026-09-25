package handlers

import (
	"testing"

	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// No GORM, First(&x, idString) com string não numérica vira SQL literal.
// Estas rotas passavam o :id da URL assim ("/establishments/0=0" era
// "WHERE 0=0", numa rota pública). Todas recusam o id antes de tocar no
// banco — aqui models.DB é nulo: sem a validação, o handler entraria em pânico.
func TestRotasComID_RecusamIDNaoNumerico(t *testing.T) {
	prev := models.DB
	models.DB = nil
	t.Cleanup(func() { models.DB = prev })
	t.Setenv("JWT_SECRET", hoursTestSecret)
	admin := hoursToken(t, jwt.MapClaims{"id": 1, "role": "admin", "account_type": "user"})

	rotas := []struct {
		method, route string
		h             fiber.Handler
	}{
		{"GET", "/establishments/:id", GetEstablishments},
		{"PUT", "/establishments/status/handler/:id", HandlerEstablishmentStatus},
		{"PUT", "/establishments/:id", UpdateEstablishment},
		{"PUT", "/establishments/:id/wallet", UpdateEstablishmentWallet},
		{"DELETE", "/establishments/:id", DeleteEstablishment},
		{"PUT", "/delivery-man/:id", UpdateDeliveryMan},
		{"DELETE", "/delivery-man/:id", DeleteDeliveryMan},
		{"PUT", "/delivery-man/:id/wallet", UpdateDeliveryManWallet},
		{"GET", "/users/:id", GetUser},
		{"PUT", "/users/:id", UpdateUser},
		{"PUT", "/users/:id/password", ChangePassword},
		{"DELETE", "/users/:id", DeleteUser},
	}
	for _, r := range rotas {
		for _, id := range []string{"0=0", "1)or(1=1", "abc", "0"} {
			path := replaceIDParam(r.route, id)
			if got := acctReq(t, r.h, r.method, r.route, path, admin, `{}`); got != 400 {
				t.Errorf("%s %s: got %d, want 400", r.method, path, got)
			}
		}
	}
}

func replaceIDParam(route, id string) string {
	out := []byte{}
	for i := 0; i < len(route); i++ {
		if route[i] == ':' && i+3 <= len(route) && route[i:i+3] == ":id" {
			out = append(out, id...)
			i += 2
			continue
		}
		out = append(out, route[i])
	}
	return string(out)
}
