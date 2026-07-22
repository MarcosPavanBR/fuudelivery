// Package models - wallet.go
// Define as estruturas de dados para carteiras digitais e transacoes.
// Cada usuario (restaurante ou entregador) possui uma carteira que
// acumula creditos de pagamentos aprovados.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TransactionType classifica o tipo de transacao na carteira.
type TransactionType string

const (
	TransactionCredit TransactionType = "credit" // Entrada de credito
	TransactionDebit  TransactionType = "debit"  // Saida de credito
)

// WalletTransaction representa uma movimentacao na carteira.
// Cada transacao registra o saldo antes e depois para auditoria.
type WalletTransaction struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`             // ID unico MongoDB
	WalletID      primitive.ObjectID `bson:"wallet_id" json:"wallet_id"`          // ID da carteira pai
	Type          TransactionType    `bson:"type" json:"type"`                    // Tipo: credit ou debit
	Amount        float64            `bson:"amount" json:"amount"`               // Valor da transacao
	BalanceBefore float64            `bson:"balance_before" json:"balance_before"` // Saldo antes da transacao
	BalanceAfter  float64            `bson:"balance_after" json:"balance_after"`  // Saldo depois da transacao
	Description   string             `bson:"description" json:"description"`      // Descricao da movimentacao
	ReferenceID   string             `bson:"reference_id,omitempty" json:"reference_id,omitempty"` // ID de referencia (payment_id, etc)
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`       // Data/hora da transacao
}

// Wallet representa a carteira digital de um usuario.
// Armazena o saldo disponivel e e usada para credito de pagamentos
// aprovados e debito de estornos.
type Wallet struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`      // ID unico MongoDB
	UserID    string             `bson:"user_id" json:"user_id"`       // ID do usuario dono da carteira
	UserType  string             `bson:"user_type" json:"user_type"`   // Tipo: "restaurant" ou "delivery"
	Balance   float64            `bson:"balance" json:"balance"`       // Saldo disponivel (R$)
	Currency  string             `bson:"currency" json:"currency"`     // Moeda (default: "BRL")
	Status    string             `bson:"status" json:"status"`         // Status: "active", "frozen", "closed"
	CreatedAt time.Time          `bson:"created_at" json:"created_at"` // Data de criacao
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"` // Data da ultima atualizacao
}
