package models

import "time"

// ChatMessage armazena mensagens de chat por pedido em Postgres.
// Substitui a collection MongoDB "chat_messages" (corte 2 da migração
// banco-único — ver docs/ARQUITETURA-BANCO-UNICO.md e sql/04_dominio_chat.sql).
//
// Estratégia de corte: dual-write temporário (Postgres primário + Mongo
// best-effort) e leitura 100% Postgres. Quando o Mongo for desligado,
// remover a escrita legada em handlers/chat.go.
type ChatMessage struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID     string     `gorm:"column:order_id;size:100;not null;index" json:"order_id"`
	SenderID    int64      `gorm:"column:sender_id;not null;index:idx_chat_sender" json:"sender_id"`
	SenderType  string     `gorm:"column:sender_type;size:20;not null;index:idx_chat_sender" json:"sender_type"`
	SenderName  string     `gorm:"column:sender_name;size:255" json:"sender_name"`
	Message     string     `gorm:"column:message;type:text;not null" json:"message"`
	MessageType string     `gorm:"column:message_type;size:20;not null;default:'text'" json:"message_type"`
	ImageURL    string     `gorm:"column:image_url;type:text" json:"image_url,omitempty"`
	ReadAt      *time.Time `gorm:"column:read_at" json:"read_at,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
}

// TableName casa com a tabela criada por sql/04_dominio_chat.sql.
func (ChatMessage) TableName() string { return "chat_messages" }
