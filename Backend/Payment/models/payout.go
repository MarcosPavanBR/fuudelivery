// Package models - payout.go
// Define a estrutura de solicitacoes de saque (Pix payout).
// Cada solicitacao representa uma transferencia de dinheiro da carteira
// do usuario (restaurante/entregador) para a conta bancaria dele via Pix.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PayoutStatus representa o estado de uma solicitacao de saque.
type PayoutStatus string

const (
	PayoutPending    PayoutStatus = "pending"     // Aguardando processamento
	PayoutProcessing PayoutStatus = "processing"  // Enviado para o gateway
	PayoutCompleted  PayoutStatus = "completed"   // Pix enviado com sucesso
	PayoutFailed     PayoutStatus = "failed"      // Falha no envio do Pix
)

// PayoutRequest representa uma solicitacao de saque da carteira.
// O usuario informa a chave Pix e o valor, e o sistema transfere
// da carteira interna para a conta bancaria via API do AbacatePay.
type PayoutRequest struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID          string             `bson:"user_id" json:"user_id"`                       // ID do usuario solicitante
	UserType        string             `bson:"user_type" json:"user_type"`                   // Tipo: "restaurant" ou "delivery"
	Amount          float64            `bson:"amount" json:"amount"`                         // Valor do saque (R$)
	PixKey          string             `bson:"pix_key" json:"pix_key"`                       // Chave Pix do recebedor
	PixKeyType      string             `bson:"pix_key_type" json:"pix_key_type"`             // Tipo: CPF, CNPJ, EMAIL, PHONE, EVP
	Status          PayoutStatus       `bson:"status" json:"status"`                         // Status atual
	GatewayID       string             `bson:"gateway_id,omitempty" json:"gateway_id"`       // ID da transferencia no AbacatePay
	FailureReason   string             `bson:"failure_reason,omitempty" json:"failure_reason"` // Motivo da falha (se houver)
	BalanceBefore   float64            `bson:"balance_before" json:"balance_before"`         // Saldo antes do saque
	BalanceAfter    float64            `bson:"balance_after" json:"balance_after"`           // Saldo depois do saque
	TransactionID   primitive.ObjectID `bson:"transaction_id,omitempty" json:"transaction_id"` // ID da transacao de debito na carteira
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}
