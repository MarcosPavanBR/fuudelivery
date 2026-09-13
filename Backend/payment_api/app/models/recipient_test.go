package models

import (
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/secretbox"
)

const recipientTestKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	b, err := secretbox.New(recipientTestKey)
	if err != nil {
		t.Fatalf("secretbox.New: %v", err)
	}
	return b
}

// TestSetTokens_NuncaGravaEmClaro é a garantia central deste modelo: depois de
// SetTokens, as colunas *_enc não podem conter o token em texto puro. Se
// contivessem, um dump da tabela entregaria o controle financeiro dos lojistas.
func TestSetTokens_NuncaGravaEmClaro(t *testing.T) {
	box := testBox(t)
	r := &Recipient{}
	access := "APP_USR-token-do-lojista-123"
	refresh := "TG-refresh-abc.def"

	if err := r.SetTokens(box, access, refresh, time.Now().Add(180*24*time.Hour)); err != nil {
		t.Fatalf("SetTokens: %v", err)
	}

	if string(r.AccessTokenEnc) == access {
		t.Fatal("access_token gravado em texto puro na coluna _enc")
	}
	if string(r.RefreshTokenEnc) == refresh {
		t.Fatal("refresh_token gravado em texto puro na coluna _enc")
	}
	for _, blob := range [][]byte{r.AccessTokenEnc, r.RefreshTokenEnc} {
		if len(blob) == 0 {
			t.Fatal("coluna cifrada vazia depois de SetTokens")
		}
	}
}

// TestTokenRoundTrip: o que foi cifrado por SetTokens volta idêntico por
// AccessToken/RefreshToken.
func TestTokenRoundTrip(t *testing.T) {
	box := testBox(t)
	r := &Recipient{}
	access := "APP_USR-abc"
	refresh := "TG-xyz"
	exp := time.Now().Add(24 * time.Hour)

	if err := r.SetTokens(box, access, refresh, exp); err != nil {
		t.Fatalf("SetTokens: %v", err)
	}

	gotAccess, err := r.AccessToken(box)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if gotAccess != access {
		t.Errorf("access: obtive %q, queria %q", gotAccess, access)
	}

	gotRefresh, err := r.RefreshToken(box)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if gotRefresh != refresh {
		t.Errorf("refresh: obtive %q, queria %q", gotRefresh, refresh)
	}

	if r.TokenExpiresAt == nil || !r.TokenExpiresAt.Equal(exp) {
		t.Errorf("expiry: obtive %v, queria %v", r.TokenExpiresAt, exp)
	}
}

// TestChaveErradaNaoDecifra: um Recipient cifrado com uma chave não decifra com
// outra — o token vira ErrCiphertext, não lixo que tentaria cobrar.
func TestChaveErradaNaoDecifra(t *testing.T) {
	box1 := testBox(t)
	box2, err := secretbox.New("1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100")
	if err != nil {
		t.Fatalf("New box2: %v", err)
	}

	r := &Recipient{}
	if err := r.SetTokens(box1, "APP_USR-abc", "TG-xyz", time.Now()); err != nil {
		t.Fatalf("SetTokens: %v", err)
	}
	if _, err := r.AccessToken(box2); err == nil {
		t.Error("AccessToken com a chave errada devia falhar")
	}
}

// TestAccessToken_SemToken: recebedor sem token gravado devolve erro, não
// string vazia silenciosa que passaria como Bearer vazio.
func TestAccessToken_SemToken(t *testing.T) {
	box := testBox(t)
	r := &Recipient{}
	if _, err := r.AccessToken(box); err == nil {
		t.Error("AccessToken sem token gravado devia falhar")
	}
}

// TestSetTokens_BoxNil: sem cofre, SetTokens falha — nunca cai num caminho que
// gravaria token em claro.
func TestSetTokens_BoxNil(t *testing.T) {
	r := &Recipient{}
	if err := r.SetTokens(nil, "a", "b", time.Now()); err == nil {
		t.Error("SetTokens com box nil devia falhar")
	}
}
