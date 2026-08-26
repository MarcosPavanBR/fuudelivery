package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateResetCode_LengthAndAlphabet(t *testing.T) {
	for i := 0; i < 500; i++ {
		code := generateResetCode()
		if len(code) != passwordResetCodeLength {
			t.Fatalf("código com tamanho errado: %q", code)
		}
		for _, ch := range code {
			if !strings.ContainsRune(passwordResetAlphabet, ch) {
				t.Fatalf("caractere fora do alfabeto sem ambiguidade: %q em %q", ch, code)
			}
		}
	}
}

func TestGenerateResetCode_CodesVary(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		seen[generateResetCode()] = true
	}
	// 100 códigos aleatórios de 32^8 colidirem é praticamente impossível.
	if len(seen) < 99 {
		t.Fatalf("gerador produzindo códigos repetidos: %d únicos em 100", len(seen))
	}
}

func TestHashResetCode_KnownVectorAndDeterminism(t *testing.T) {
	sum := sha256.Sum256([]byte("abc"))
	expected := hex.EncodeToString(sum[:])

	if got := hashResetCode("abc"); got != expected {
		t.Fatalf("hash divergiu do SHA-256 direto: got %s want %s", got, expected)
	}
	if hashResetCode("abc") != hashResetCode("abc") {
		t.Fatal("hash não é determinístico")
	}
	if hashResetCode("abc") == hashResetCode("abd") {
		t.Fatal("hashes de entradas distintas colidiram")
	}
	if len(hashResetCode("abc")) != 64 {
		t.Fatalf("hash deveria ter 64 chars hex, tem %d", len(hashResetCode("abc")))
	}
}

func TestIsValidResetUserType(t *testing.T) {
	valid := []string{"client", "user", "delivery_man"}
	invalid := []string{"", "admin", "Client", "clients", "deliveryman"}

	for _, v := range valid {
		if !isValidResetUserType(v) {
			t.Errorf("%q deveria ser válido", v)
		}
	}
	for _, iv := range invalid {
		if isValidResetUserType(iv) {
			t.Errorf("%q deveria ser inválido", iv)
		}
	}
}

func TestMaskIdentifier(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "***"},
		{"abc", "***"},
		{"abcd", "***"},
		{"abcde", "ab****de"},
		{"11999998888", "11****88"},
	}
	for _, tc := range cases {
		if got := maskIdentifier(tc.in); got != tc.want {
			t.Errorf("maskIdentifier(%q) = %q; queria %q", tc.in, got, tc.want)
		}
	}
}
