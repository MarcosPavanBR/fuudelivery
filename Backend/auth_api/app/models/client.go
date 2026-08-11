package models

import (
	"time"
)

// Client representa um cliente do AppComida.
// Diferente do User (que é dono de restaurante), o Client
// se autentica via telefone + senha e não tem estabelecimento vinculado.
type Client struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Phone     string    `gorm:"uniqueIndex:idx_clients_phone;not null" json:"phone"`
	Password  string    `gorm:"not null" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
