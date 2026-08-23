package models

import (
	"regexp"
	"testing"
)

// Garante que o valor do refresh token tem 64 hex chars (32 bytes) e é único.
func TestGenerateRefreshTokenValue(t *testing.T) {
	hexPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := GenerateRefreshTokenValue()
		if err != nil {
			t.Fatalf("GenerateRefreshTokenValue retornou erro: %v", err)
		}
		if !hexPattern.MatchString(token) {
			t.Errorf("token fora do formato esperado (64 hex chars): %q", token)
		}
		if seen[token] {
			t.Error("GenerateRefreshTokenValue gerou token duplicado — entropia insuficiente")
		}
		seen[token] = true
	}
}
