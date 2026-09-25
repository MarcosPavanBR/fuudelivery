package handlers

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// Renovar sem cobrança estendia o frete grátis a cada chamada: só admin.
func TestRenewSubscription_SoAdmin(t *testing.T) {
	t.Setenv("JWT_SECRET", hoursTestSecret)
	cliente := hoursToken(t, jwt.MapClaims{"id": 9, "role": "client"})
	if got := postHours(t, "/subscriptions/renew", cliente, `{"user_id":9}`, RenewSubscription); got != 403 {
		t.Errorf("cliente renovando: got %d, want 403", got)
	}
}
