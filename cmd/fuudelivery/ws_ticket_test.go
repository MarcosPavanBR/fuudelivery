package main

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// createTestJWT gera um JWT assinado para testes unitários.
func createTestJWT(t *testing.T, extraClaims map[string]interface{}) string {
	t.Helper()
	claims := jwt.MapClaims{
		"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
	}
	for k, v := range extraClaims {
		claims[k] = v
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		t.Fatalf("failed to sign test JWT: %v", err)
	}
	return tokenString
}

func TestIssueWSTicket_ReturnsTicket(t *testing.T) {
	// IssueWSTicket precisa de JWT_SECRET no env
	t.Setenv("JWT_SECRET", "test-secret-key-for-ws-ticket")
	t.Setenv("JWT_EXPIRATION_HOURS", "1")

	// Gerar um JWT válido para a requisição
	jwtToken := createTestJWT(t, map[string]interface{}{
		"id":   1,
		"role": "admin",
	})

	ticket, err := IssueWSTicket(jwtToken)
	if err != nil {
		t.Fatalf("IssueWSTicket falhou: %v", err)
	}
	if ticket == "" {
		t.Fatal("ticket vazio")
	}
	if len(ticket) != 64 { // 32 bytes hex
		t.Fatalf("ticket com tamanho errado: %d (esperado 64)", len(ticket))
	}
}

func TestResolveWSTicket_ValidTicket(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-ws-ticket")
	t.Setenv("JWT_EXPIRATION_HOURS", "1")

	jwtToken := createTestJWT(t, map[string]interface{}{
		"id":    42,
		"role":  "client",
		"phone": "123456789",
	})

	ticket, err := IssueWSTicket(jwtToken)
	if err != nil {
		t.Fatalf("IssueWSTicket falhou: %v", err)
	}

	// Primeira resolução deve funcionar
	claims, err := resolveWSTicket("", ticket)
	if err != nil {
		t.Fatalf("resolveWSTicket falhou: %v", err)
	}

	userID, ok := claims["id"].(float64)
	if !ok || int64(userID) != 42 {
		t.Fatalf("claims id errado: %v", claims["id"])
	}
}

func TestResolveWSTicket_SingleUse(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-ws-ticket")
	t.Setenv("JWT_EXPIRATION_HOURS", "1")

	jwtToken := createTestJWT(t, map[string]interface{}{
		"id":   1,
		"role": "admin",
	})

	ticket, err := IssueWSTicket(jwtToken)
	if err != nil {
		t.Fatalf("IssueWSTicket falhou: %v", err)
	}

	// Primeira resolução: OK
	_, err = resolveWSTicket("", ticket)
	if err != nil {
		t.Fatalf("primeira resolução falhou: %v", err)
	}

	// Segunda resolução: ticket já consumido
	_, err = resolveWSTicket("", ticket)
	if err == nil {
		t.Fatal("segunda resolução deveria falhar (ticket de uso único)")
	}
}

func TestResolveWSTicket_InvalidTicket(t *testing.T) {
	_, err := resolveWSTicket("", "ticket-inexistente")
	if err == nil {
		t.Fatal("ticket inexistente deveria falhar")
	}
}

func TestResolveWSTicket_EmptyArgs(t *testing.T) {
	_, err := resolveWSTicket("", "")
	if err == nil {
		t.Fatal("args vazios deveriam falhar")
	}
}

func TestResolveWSTicket_DeprecatedTokenFallback(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-ws-ticket")
	t.Setenv("JWT_EXPIRATION_HOURS", "1")

	jwtToken := createTestJWT(t, map[string]interface{}{
		"id":   99,
		"role": "delivery_man",
	})

	// Passar JWT diretamente no arg "token" (caminho deprecated)
	claims, err := resolveWSTicket(jwtToken, "")
	if err != nil {
		t.Fatalf("deprecated token fallback falhou: %v", err)
	}

	userID, ok := claims["id"].(float64)
	if !ok || int64(userID) != 99 {
		t.Fatalf("claims id errado: %v", claims["id"])
	}
}

func TestCleanupWSTickets_RemovesExpired(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-ws-ticket")
	t.Setenv("JWT_EXPIRATION_HOURS", "1")

	jwtToken := createTestJWT(t, map[string]interface{}{
		"id":   1,
		"role": "admin",
	})

	// Inserir ticket com TTL zero (já expirado)
	claims, _ := parseWSToken(jwtToken)
	ticket := "expired-ticket-abc123"
	wsTicketsMu.Lock()
	wsTickets[ticket] = &wsTicket{
		Claims:    claims,
		ExpiresAt: time.Now().Add(-1 * time.Second), // já expirado
	}
	wsTicketsMu.Unlock()

	// Rodar cleanup manual
	cutoff := time.Now()
	wsTicketsMu.Lock()
	for k, t2 := range wsTickets {
		if t2.ExpiresAt.Before(cutoff) {
			delete(wsTickets, k)
		}
	}
	wsTicketsMu.Unlock()

	// Ticket não deve mais existir
	_, err := resolveWSTicket("", ticket)
	if err == nil {
		t.Fatal("ticket expirado deveria ter sido removido pelo cleanup")
	}
}
