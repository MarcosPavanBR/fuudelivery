package models

import "time"

// PasswordResetToken guarda o hash SHA-256 de um código de uso único gerado
// pelo admin (fluxo de reset assistido: suporte fala com o usuário, gera o
// código no WebAdmin e informa por telefone/WhatsApp).
//
// Por que código em vez de link por email: clientes (tabela clients) NÃO têm
// email — só telefone — e o projeto não tem nenhum serviço de email
// integrado. O código de 8 caracteres sem ambiguidade pode ser ditado por
// telefone e usado na página pública /resetar-senha do WebRestaurant.
//
// Segurança:
//   - Só o HASH do código é persistido (o código em claro existe uma vez, na
//     resposta do endpoint admin).
//   - Uso único (UsedAt), TTL curto (15 min) e teto de tentativas
//     (maxPasswordResetAttempts no handler) contra força bruta.
//   - A coluna code_hash entra em audit_redacted_columns (sql/13).
type PasswordResetToken struct {
	ID        uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserType  string     `gorm:"size:20;not null" json:"user_type"` // client | user | delivery_man
	UserID    uint       `gorm:"not null" json:"-"`
	CodeHash  string     `gorm:"size:64;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	Attempts  int        `gorm:"not null;default:0" json:"attempts"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (PasswordResetToken) TableName() string { return "password_reset_tokens" }
