package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChargebackStatus string

const (
	ChargebackPending  ChargebackStatus = "pending"
	ChargebackApproved ChargebackStatus = "approved"
	ChargebackRejected ChargebackStatus = "rejected"
	ChargebackEscalated ChargebackStatus = "escalated"
)

type ChargebackReason string

const (
	ReasonUnauthorized  ChargebackReason = "unauthorized"
	ReasonNotReceived   ChargebackReason = "not_received"
	ReasonDefective     ChargebackReason = "defective"
	ReasonDuplicate     ChargebackReason = "duplicate"
	ReasonOther         ChargebackReason = "other"
)

type Chargeback struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PaymentID     primitive.ObjectID `bson:"payment_id" json:"payment_id"`
	PaymentOrderID string            `bson:"payment_order_id" json:"payment_order_id"`
	CustomerID    string             `bson:"customer_id" json:"customer_id"`
	EstablishmentID string           `bson:"establishment_id" json:"establishment_id"`
	Amount        float64            `bson:"amount" json:"amount"`
	Reason        ChargebackReason   `bson:"reason" json:"reason"`
	Description   string             `bson:"description,omitempty" json:"description,omitempty"`
	Status        ChargebackStatus   `bson:"status" json:"status"`
	EvidenceCount int                `bson:"evidence_count" json:"evidence_count"`
	AssignedTo    string             `bson:"assigned_to,omitempty" json:"assigned_to,omitempty"`
	AssignedAt    *time.Time         `bson:"assigned_at,omitempty" json:"assigned_at,omitempty"`
	ResolvedBy    string             `bson:"resolved_by,omitempty" json:"resolved_by,omitempty"`
	ResolvedAt    *time.Time         `bson:"resolved_at,omitempty" json:"resolved_at,omitempty"`
	Resolution    string             `bson:"resolution,omitempty" json:"resolution,omitempty"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}
