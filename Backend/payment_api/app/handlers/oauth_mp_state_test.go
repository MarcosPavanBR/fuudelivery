package handlers

import (
	"strings"
	"testing"
	"time"
)

const stateSecret = "segredo-de-teste-do-state-hmac"

// TestState_RoundTrip: um state assinado agora verifica e devolve o mesmo
// establishment_id.
func TestState_RoundTrip(t *testing.T) {
	now := time.Now()
	s := signState(42, stateSecret, now)
	got, err := verifyState(s, stateSecret, now)
	if err != nil {
		t.Fatalf("verifyState do que acabei de assinar: %v", err)
	}
	if got != 42 {
		t.Errorf("establishment_id: obtive %d, queria 42", got)
	}
}

// TestState_AssinaturaAdulterada é a garantia central: mexer em qualquer parte
// do state (inclusive trocar o establishment_id) invalida a assinatura. Sem
// isso, alguém conectaria a conta MP dele ao estabelecimento de outro.
func TestState_AssinaturaAdulterada(t *testing.T) {
	now := time.Now()
	s := signState(42, stateSecret, now)

	// Vira o último caractere da assinatura.
	adulterado := s[:len(s)-1] + flipChar(s[len(s)-1])
	if _, err := verifyState(adulterado, stateSecret, now); err == nil {
		t.Error("state com assinatura adulterada devia ser rejeitado")
	}
}

// TestState_PayloadTrocado: tentar trocar o establishment_id no payload sem
// re-assinar tem que falhar.
func TestState_PayloadTrocado(t *testing.T) {
	now := time.Now()
	s := signState(42, stateSecret, now)
	partes := strings.SplitN(s, ".", 2)

	// Assina o estab 42, mas cola a assinatura num payload do estab 99.
	forjado := signPayloadFor(t, 99, now) + "." + partes[1]
	if _, err := verifyState(forjado, stateSecret, now); err == nil {
		t.Error("payload de outro establishment com assinatura alheia devia ser rejeitado")
	}
}

// TestState_SegredoErrado: um state assinado com um segredo não verifica com
// outro. É o que impede forjar state sem conhecer o MERCADOPAGO_OAUTH_STATE_SECRET.
func TestState_SegredoErrado(t *testing.T) {
	now := time.Now()
	s := signState(42, stateSecret, now)
	if _, err := verifyState(s, "outro-segredo", now); err == nil {
		t.Error("state assinado com outro segredo devia ser rejeitado")
	}
}

// TestState_Expirado: passado o TTL, o state não vale mais.
func TestState_Expirado(t *testing.T) {
	assinadoEm := time.Now().Add(-stateTTL - time.Minute)
	s := signState(42, stateSecret, assinadoEm)
	if _, err := verifyState(s, stateSecret, time.Now()); err == nil {
		t.Error("state além do TTL devia ser rejeitado")
	}
}

// TestState_Malformado: entradas quebradas não podem dar panic nem passar.
func TestState_Malformado(t *testing.T) {
	now := time.Now()
	for _, s := range []string{"", "semponto", "a.b", "!!!.###", "."} {
		if _, err := verifyState(s, stateSecret, now); err == nil {
			t.Errorf("verifyState(%q) devia falhar", s)
		}
	}
}

// TestState_NonceVaria: dois states do mesmo estab não são iguais (nonce), então
// um não é previsível a partir do outro.
func TestState_NonceVaria(t *testing.T) {
	now := time.Now()
	a := signState(42, stateSecret, now)
	b := signState(42, stateSecret, now)
	if a == b {
		t.Error("dois states do mesmo estab saíram idênticos — nonce não variou")
	}
}

func flipChar(b byte) string {
	if b == 'a' {
		return "b"
	}
	return "a"
}

// signPayloadFor reconstrói só o payload base64 de um estab (sem assinatura
// válida), para o teste de payload trocado.
func signPayloadFor(t *testing.T, estID int64, now time.Time) string {
	t.Helper()
	full := signState(estID, stateSecret, now)
	return strings.SplitN(full, ".", 2)[0]
}
