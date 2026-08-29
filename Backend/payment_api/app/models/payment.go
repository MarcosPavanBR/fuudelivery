package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// ============================================================================
// Payment — CORTE 4 banco-único: substitui a collection MongoDB "payments".
//
// Mapeia a tabela `payments` (sql/03_dominio_pagamentos.sql), que é o
// superset do antigo struct Mongo. Os JSON tags foram preservados para
// manter o contrato de API com apps e painéis — a única mudança visível
// é o campo `id`, que passou de hex ObjectID para numérico (string na
// resposta continua aceitável pois os consumidores tratam como opaco).
// ============================================================================

// SplitRule mantém o mesmo shape do documento legado (json tags preservados).
type SplitRule struct {
	ReceiverID   int64   `json:"receiver_id"`
	ReceiverType string  `json:"receiver_type"`
	Amount       float64 `json:"amount"`
	Percentage   float64 `json:"percentage"`
}

// SplitRules serializa as regras de split como JSONB (coluna split_rules).
type SplitRules []SplitRule

// Value implementa driver.Valuer — grava o slice como JSON no Postgres.
func (s SplitRules) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("serializar split_rules: %w", err)
	}
	return string(b), nil
}

// Scan implementa sql.Scanner — lê o JSONB de volta.
func (s *SplitRules) Scan(src interface{}) error {
	if src == nil {
		*s = SplitRules{}
		return nil
	}
	var bytes []byte
	switch v := src.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("tipo inesperado para split_rules: %T", src)
	}
	if len(bytes) == 0 {
		*s = SplitRules{}
		return nil
	}
	return json.Unmarshal(bytes, s)
}

type Payment struct {
	ID                      int64      `gorm:"primaryKey;column:id" json:"id"`
	OrderID                 string     `gorm:"column:order_id" json:"order_id"`
	CustomerID              int64      `gorm:"column:customer_id" json:"customer_id"`
	CustomerPhone           string     `gorm:"column:customer_phone" json:"customer_phone,omitempty"`
	EstablishmentID         int64      `gorm:"column:establishment_id" json:"establishment_id"`
	Amount                  float64    `gorm:"column:amount" json:"amount"`
	DeliveryAmount          float64    `gorm:"column:delivery_amount" json:"delivery_amount,omitempty"`
	Method                  string     `gorm:"column:method" json:"method"`
	Status                  string     `gorm:"column:status" json:"status"`
	PixQRCode               string     `gorm:"column:pix_qr_code" json:"pix_qr_code,omitempty"`
	PixCopyPaste            string     `gorm:"column:pix_copy_paste" json:"pix_copy_paste,omitempty"`
	QRCodeBase64            string     `gorm:"column:qr_code_base64" json:"qr_code_base64,omitempty"`
	TicketURL               string     `gorm:"column:ticket_url" json:"ticket_url,omitempty"`
	MPPaymentID             int64      `gorm:"column:mp_payment_id" json:"mp_payment_id,omitempty"`
	MPStatus                string     `gorm:"column:mp_status" json:"mp_status,omitempty"`
	AbacatePayID            string     `gorm:"column:abacatepay_id" json:"abacatepay_id,omitempty"`
	// Multi-gateway fields (migration 16)
	Gateway                 string     `gorm:"column:gateway;default:'abacatepay'" json:"gateway,omitempty"`
	GatewayTxID             string     `gorm:"column:gateway_transaction_id" json:"gateway_transaction_id,omitempty"`
	CardLastDigits          string     `gorm:"column:card_last_digits" json:"card_last_digits,omitempty"`
	CardToken               string     `gorm:"column:card_token" json:"card_token,omitempty"` // ⚠ nunca devolver em resposta de API (ver sql/03)
	Installments            int        `gorm:"column:installments" json:"installments,omitempty"`
	SplitRules              SplitRules `gorm:"column:split_rules;type:jsonb" json:"split_rules"`
	CreatedAt               time.Time  `gorm:"column:created_at" json:"created_at"`
	ConfirmedAt             *time.Time `gorm:"column:confirmed_at" json:"confirmed_at,omitempty"`
	WalletCreditedAt        *time.Time `gorm:"column:wallet_credited_at" json:"wallet_credited_at,omitempty"`
	EstablishmentCreditedAt *time.Time `gorm:"column:establishment_credited_at" json:"establishment_credited_at,omitempty"`
	ApprovedBy              string     `gorm:"column:approved_by" json:"approved_by,omitempty"`
	RefundedAt              *time.Time `gorm:"column:refunded_at" json:"refunded_at,omitempty"`
	RejectedAt              *time.Time `gorm:"column:rejected_at" json:"rejected_at,omitempty"`
	RejectedBy              string     `gorm:"column:rejected_by" json:"rejected_by,omitempty"`
	RejectionReason         string     `gorm:"column:rejection_reason" json:"rejection_reason,omitempty"`
}

// TableName fixa o nome da tabela.
func (Payment) TableName() string { return "payments" }

// IDString devolve o id em formato string para respostas/eventos que antes
// usavam ObjectID.Hex().
func (p Payment) IDString() string { return fmt.Sprintf("%d", p.ID) }
