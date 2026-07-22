package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentApproved  PaymentStatus = "approved"
	PaymentRejected  PaymentStatus = "rejected"
	PaymentCancelled PaymentStatus = "cancelled"
	PaymentRefunded  PaymentStatus = "refunded"
	PaymentDisputed  PaymentStatus = "disputed"
)

type PaymentMethod string

const (
	PaymentMethodPix  PaymentMethod = "pix"
	PaymentMethodCard PaymentMethod = "card"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Payment struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderID         string             `bson:"order_id" json:"order_id"`
	CustomerID      string             `bson:"customer_id" json:"customer_id"`
	CustomerName    string             `bson:"customer_name" json:"customer_name"`
	CustomerEmail   string             `bson:"customer_email" json:"customer_email"`
	CustomerPhone   string             `bson:"customer_phone,omitempty" json:"customer_phone,omitempty"`
	EstablishmentID string             `bson:"establishment_id" json:"establishment_id"`
	EstablishmentName string           `bson:"establishment_name" json:"establishment_name"`
	Amount          float64            `bson:"amount" json:"amount"`
	DeliveryAmount  float64            `bson:"delivery_amount,omitempty" json:"delivery_amount,omitempty"`
	Method          PaymentMethod      `bson:"method" json:"method"`
	Status          PaymentStatus      `bson:"status" json:"status"`
	RiskLevel       RiskLevel          `bson:"risk_level" json:"risk_level"`
	RiskScore       float64            `bson:"risk_score" json:"risk_score"`
	RequiresApproval bool              `bson:"requires_approval" json:"requires_approval"`
	ApprovedBy      string             `bson:"approved_by,omitempty" json:"approved_by,omitempty"`
	ApprovedAt      *time.Time         `bson:"approved_at,omitempty" json:"approved_at,omitempty"`
	RejectionReason string             `bson:"rejection_reason,omitempty" json:"rejection_reason,omitempty"`
	RejectedBy      string             `bson:"rejected_by,omitempty" json:"rejected_by,omitempty"`
	RejectedAt      *time.Time         `bson:"rejected_at,omitempty" json:"rejected_at,omitempty"`
	Reference       string             `bson:"reference,omitempty" json:"reference,omitempty"`
	GatewayID       string             `bson:"gateway_id,omitempty" json:"gateway_id,omitempty"`
	GatewayStatus   string             `bson:"gateway_status,omitempty" json:"gateway_status,omitempty"`
	Metadata        map[string]string  `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}

type PaymentFilter struct {
	Status          string `query:"status"`
	RiskLevel       string `query:"risk_level"`
	EstablishmentID string `query:"establishment_id"`
	CustomerID      string `query:"customer_id"`
	Method          string `query:"method"`
	DateFrom        string `query:"date_from"`
	DateTo          string `query:"date_to"`
	Page            int    `query:"page"`
	Limit           int    `query:"limit"`
}
