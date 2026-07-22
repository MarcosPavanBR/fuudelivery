package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TransactionType string

const (
	TransactionCredit TransactionType = "credit"
	TransactionDebit  TransactionType = "debit"
)

type WalletTransaction struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WalletID      primitive.ObjectID `bson:"wallet_id" json:"wallet_id"`
	Type          TransactionType    `bson:"type" json:"type"`
	Amount        float64            `bson:"amount" json:"amount"`
	BalanceBefore float64            `bson:"balance_before" json:"balance_before"`
	BalanceAfter  float64            `bson:"balance_after" json:"balance_after"`
	Description   string             `bson:"description" json:"description"`
	ReferenceID   string             `bson:"reference_id,omitempty" json:"reference_id,omitempty"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
}

type Wallet struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      string             `bson:"user_id" json:"user_id"`
	UserType    string             `bson:"user_type" json:"user_type"`
	Balance     float64            `bson:"balance" json:"balance"`
	Currency    string             `bson:"currency" json:"currency"`
	Status      string             `bson:"status" json:"status"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
