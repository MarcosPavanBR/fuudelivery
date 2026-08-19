package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// RefreshToken armazena tokens de renovação de sessão no PostgreSQL.
// Tokens de acesso (JWT) são stateless e de curta duração (15 min).
// Refresh tokens são stateful, de longa duração (30 dias) e revogáveis.
type RefreshToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Token     string    `gorm:"uniqueIndex;size:64" json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `gorm:"default:false" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// GenerateRefreshTokenValue gera um token criptograficamente seguro
// de 32 bytes (64 hex chars) para uso como refresh token.
func GenerateRefreshTokenValue() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
