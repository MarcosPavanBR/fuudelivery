package main

import (
	"testing"
)

// ============================================================================
// Contrato do /ws/:id e /ws/chat/:orderId/:userId/:userType quanto ao tipo do
// user ID na URL.
//
// Histórico: os clientes WebSocket do WebRestaurant passaram de user.sub
// (string legado) para user.id (numérico) quando a sessão passou a vir de
// GET /auth/session. O handler compara o id da URL com o claim "id" do JWT
// (float64 no MapClaims) via strconv.ParseInt + int64 cast — então o
// caminho NUMÉRICO é o que precisa estar provado. Um id string no claim
// (formato de sessão antiga) NÃO pode dar "User ID mismatch" para o dono
// legítimo, e um id numérico desalinhado com a URL TEM que ser barrado.
//
// Estes testes extraem a regra de decisão (claim vs. URL) para uma função
// pura testável — wsUserIDMatches — e o handler passa a usá-la, para que a
// regra tenha teste direto em vez de só via integração.
// ============================================================================

func TestWSUserIDMatches_NumericoDoToken(t *testing.T) {
	// Claim numérico (auth_api emite float64 no MapClaims) x URL numérica:
	// o caminho novo, user.id do /auth/session.
	claims := map[string]interface{}{"id": float64(42)}
	if !wsUserIDMatches(claims, "42") {
		t.Fatal("id numérico do token deve casar com a URL numérica")
	}
}

func TestWSUserIDMatches_StringDoClaim(t *testing.T) {
	// Token de sessão antiga com id em STRING: o dono legítimo não pode
	// tomar "User ID mismatch" por causa do tipo.
	claims := map[string]interface{}{"id": "42"}
	if !wsUserIDMatches(claims, "42") {
		t.Fatal("id string no claim deve ser aceito quando o valor casa com a URL")
	}
}

func TestWSUserIDMatches_RejeitaDesalinhado(t *testing.T) {
	claims := map[string]interface{}{"id": float64(42)}
	if wsUserIDMatches(claims, "43") {
		t.Fatal("id de outro usuário na URL deve ser barrado (IDOR)")
	}
}

func TestWSUserIDMatches_RejeitaURLNaoNumerica(t *testing.T) {
	claims := map[string]interface{}{"id": float64(42)}
	for _, urlID := range []string{"", "abc", "42x", "0x2a"} {
		if wsUserIDMatches(claims, urlID) {
			t.Fatalf("URL com id não numérico %q deve ser barrada", urlID)
		}
	}
}

func TestWSUserIDMatches_RejeitaClaimVazio(t *testing.T) {
	// Claim sem id (token forjado sem o campo) nunca casa com nada.
	claims := map[string]interface{}{}
	if wsUserIDMatches(claims, "42") {
		t.Fatal("claim sem id não deve casar com nenhum user da URL")
	}
	if wsUserIDMatches(claims, "0") {
		t.Fatal("claim sem id não deve casar nem com 0")
	}
}
