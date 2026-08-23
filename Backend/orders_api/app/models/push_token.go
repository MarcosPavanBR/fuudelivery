package models

import "time"

// PushToken armazena tokens de notificação push (Expo) em Postgres.
// Substitui a collection MongoDB "push_tokens" (corte 1 da migração
// banco-único — ver docs/ARQUITETURA-BANCO-UNICO.md e sql/01_dominio_pedidos.sql).
//
// Durante a transição o handler faz dual-write (Postgres + Mongo best-effort);
// a leitura já é 100% Postgres. Quando o Mongo for desligado, remover a
// escrita legada em notifications.go.
type PushToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"not null;uniqueIndex:idx_push_user_type" json:"user_id"`
	UserType  string    `gorm:"size:20;not null;default:'client';uniqueIndex:idx_push_user_type" json:"user_type"`
	PushToken string    `gorm:"type:text;not null" json:"push_token"`
	Platform  string    `gorm:"size:20" json:"platform,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName casa com a tabela criada por sql/01_dominio_pedidos.sql.
func (PushToken) TableName() string { return "push_tokens" }
