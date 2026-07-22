package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SplitRule struct {
	ReceiverID   int64   `bson:"receiver_id" json:"receiver_id"`
	ReceiverType string  `bson:"receiver_type" json:"receiver_type"`
	Amount       float64 `bson:"amount" json:"amount"`
	Percentage   float64 `bson:"percentage" json:"percentage"`
}

type Payment struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderID         string             `bson:"order_id" json:"order_id"`
	CustomerID      int64              `bson:"customer_id" json:"customer_id"`
	CustomerPhone   string             `bson:"customer_phone,omitempty" json:"customer_phone,omitempty"`
	EstablishmentID int64              `bson:"establishment_id" json:"establishment_id"`
	Amount          float64            `bson:"amount" json:"amount"`
	DeliveryAmount  float64            `bson:"delivery_amount,omitempty" json:"delivery_amount,omitempty"`
	Method          string             `bson:"method" json:"method"`
	Status          string             `bson:"status" json:"status"`
	PixQRCode       string             `bson:"pix_qr_code,omitempty" json:"pix_qr_code,omitempty"`
	PixCopyPaste    string             `bson:"pix_copy_paste,omitempty" json:"pix_copy_paste,omitempty"`
	QRCodeBase64    string             `bson:"qr_code_base64,omitempty" json:"qr_code_base64,omitempty"`
	TicketURL       string             `bson:"ticket_url,omitempty" json:"ticket_url,omitempty"`
	MPPaymentID     int64              `bson:"mp_payment_id,omitempty" json:"mp_payment_id,omitempty"`
	MPStatus        string             `bson:"mp_status,omitempty" json:"mp_status,omitempty"`
	AbacatePayID    string             `bson:"abacatepay_id,omitempty" json:"abacatepay_id,omitempty"`
	CardLastDigits  string             `bson:"card_last_digits,omitempty" json:"card_last_digits,omitempty"`
	CardToken       string             `bson:"card_token,omitempty" json:"card_token,omitempty"`
	Installments    int                `bson:"installments,omitempty" json:"installments,omitempty"`
	SplitRules      []SplitRule        `bson:"split_rules" json:"split_rules"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	ConfirmedAt     *time.Time         `bson:"confirmed_at,omitempty" json:"confirmed_at,omitempty"`
	WalletCreditedAt *time.Time        `bson:"wallet_credited_at,omitempty" json:"wallet_credited_at,omitempty"`
}
