package main

import (
	"testing"

	deliveryModels "github.com/carloshomar/fuudelivery/delivery_api/app/models"
	"github.com/golang-jwt/jwt/v5"
)

// Pedido da loja 5, do cliente 7 (tel +55119), com o entregador 9.
var solicitacaoTeste = deliveryModels.DeliverySolicitation{
	EstablishmentID: 5,
	UserID:          7,
	UserPhone:       "+55119",
	DeliveryManID:   9,
}

func TestClaimsParticipateInSolicitation(t *testing.T) {
	casos := []struct {
		nome   string
		claims jwt.MapClaims
		quer   bool
	}{
		{"cliente dono (id)", jwt.MapClaims{"id": float64(7), "role": "client"}, true},
		{"cliente dono (telefone)", jwt.MapClaims{"id": float64(70), "role": "client", "phone": "+55119"}, true},
		{"loja do pedido", jwt.MapClaims{"id": float64(33), "role": "user", "establishment_id": float64(5)}, true},
		{"entregador atribuído", jwt.MapClaims{"id": float64(9), "phone": "+55000"}, true},

		// Colisões de id entre tabelas — todas eram aceitas antes.
		{"cliente com id = id da loja", jwt.MapClaims{"id": float64(5), "role": "client"}, false},
		{"cliente com id = id do entregador", jwt.MapClaims{"id": float64(9), "role": "client"}, false},
		{"entregador com id = id do cliente", jwt.MapClaims{"id": float64(7)}, false},
		{"usuário de outra loja com id = cliente", jwt.MapClaims{"id": float64(7), "role": "user", "establishment_id": float64(6)}, false},
		{"entregador com telefone do cliente", jwt.MapClaims{"id": float64(99), "phone": "+55119"}, false},
		{"outra loja", jwt.MapClaims{"id": float64(1), "role": "user", "establishment_id": float64(6)}, false},
	}
	for _, c := range casos {
		if got := claimsParticipateInSolicitation(c.claims, solicitacaoTeste); got != c.quer {
			t.Errorf("%s: got %v, want %v", c.nome, got, c.quer)
		}
	}
}

// O sender_type do chat vem do token: um cliente não se apresenta como loja
// ou suporte trocando o :userType da URL.
func TestChatUserTypeFromClaims(t *testing.T) {
	casos := map[string]jwt.MapClaims{
		"client":      {"id": float64(7), "role": "client"},
		"restaurant":  {"id": float64(3), "role": "user", "establishment_id": float64(42)},
		"deliveryman": {"id": float64(9)},
		"admin":       {"id": float64(1), "role": "admin"},
	}
	for want, claims := range casos {
		if got := chatUserTypeFromClaims(claims); got != want {
			t.Errorf("%v: got %q, want %q", claims, got, want)
		}
	}
}
