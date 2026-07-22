// Package models define as estruturas de dados (structs) e constantes
// usadas por todo o Payment Service. Cada arquivo representa um dominio:
// - payment.go: Pagamentos e seus status/risco
// - chargeback.go: Estornos e disputas
// - wallet.go: Carteiras e transacoes
// - evidence.go: Evidencias para estornos
// - user.go: Usuarios e autenticacao
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PaymentStatus representa o ciclo de vida de um pagamento.
// Um pagamento pode transitar entre esses status durante seu processamento.
type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"   // Aguardando analise de risco
	PaymentApproved  PaymentStatus = "approved"   // Aprovado (auto ou manual)
	PaymentRejected  PaymentStatus = "rejected"   // Rejeitado por risco ou operador
	PaymentCancelled PaymentStatus = "cancelled"  // Cancelado pelo usuario
	PaymentRefunded  PaymentStatus = "refunded"   // Estornado (devolvido ao cliente)
	PaymentDisputed  PaymentStatus = "disputed"   // Em disputa (chargeback)
)

// PaymentMethod representa o metodo de pagamento utilizado.
type PaymentMethod string

const (
	PaymentMethodPix  PaymentMethod = "pix"  // Pagamento instantaneo via PIX
	PaymentMethodCard PaymentMethod = "card" // Pagamento via cartao de credito/debito
)

// RiskLevel classifica o nivel de risco de um pagamento.
// Determina se o pagamento precisa de aprovacao manual.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"      // Score 0-39: aprovacao automatica
	RiskMedium   RiskLevel = "medium"   // Score 40-69: pode precisar de revisao
	RiskHigh     RiskLevel = "high"     // Score 70-89: requer aprovacao manual
	RiskCritical RiskLevel = "critical" // Score 90-100: bloqueado, nao processa
)

// Payment e a estrutura principal que representa um pagamento no sistema.
// Contem todas as informacoes necessarias para processamento, auditoria
// e decisao de aprovacao.
type Payment struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`              // ID unico MongoDB
	OrderID          string             `bson:"order_id" json:"order_id"`              // ID do pedido no sistema principal
	CustomerID       string             `bson:"customer_id" json:"customer_id"`        // ID do cliente
	CustomerName     string             `bson:"customer_name" json:"customer_name"`    // Nome do cliente
	CustomerEmail    string             `bson:"customer_email" json:"customer_email"`  // Email do cliente
	CustomerPhone    string             `bson:"customer_phone,omitempty" json:"customer_phone,omitempty"` // Telefone (opcional)
	EstablishmentID  string             `bson:"establishment_id" json:"establishment_id"` // ID do restaurante/estabelecimento
	EstablishmentName string           `bson:"establishment_name" json:"establishment_name"` // Nome do estabelecimento
	Amount           float64            `bson:"amount" json:"amount"`                  // Valor do pagamento (R$)
	DeliveryAmount   float64            `bson:"delivery_amount,omitempty" json:"delivery_amount,omitempty"` // Valor da taxa de entrega
	Method           PaymentMethod      `bson:"method" json:"method"`                  // Metodo de pagamento
	Status           PaymentStatus      `bson:"status" json:"status"`                  // Status atual
	RiskLevel        RiskLevel          `bson:"risk_level" json:"risk_level"`          // Nivel de risco calculado
	RiskScore        float64            `bson:"risk_score" json:"risk_score"`          // Score numerico 0-100
	RequiresApproval bool              `bson:"requires_approval" json:"requires_approval"` // Se precisa aprovacao manual
	ApprovedBy       string             `bson:"approved_by,omitempty" json:"approved_by,omitempty"` // Quem aprovou
	ApprovedAt       *time.Time         `bson:"approved_at,omitempty" json:"approved_at,omitempty"` // Quando aprovou
	RejectionReason  string             `bson:"rejection_reason,omitempty" json:"rejection_reason,omitempty"` // Motivo da rejeicao
	RejectedBy       string             `bson:"rejected_by,omitempty" json:"rejected_by,omitempty"` // Quem rejeitou
	RejectedAt       *time.Time         `bson:"rejected_at,omitempty" json:"rejected_at,omitempty"` // Quando rejeitou
	Reference        string             `bson:"reference,omitempty" json:"reference,omitempty"` // Referencia externa
	GatewayID        string             `bson:"gateway_id,omitempty" json:"gateway_id,omitempty"` // ID no gateway (AbacatePay)
	GatewayStatus    string             `bson:"gateway_status,omitempty" json:"gateway_status,omitempty"` // Status no gateway
	Metadata         map[string]string  `bson:"metadata,omitempty" json:"metadata,omitempty"` // Dados adicionais flexiveis
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`          // Data de criacao
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`          // Data da ultima atualizacao
}

// PaymentFilter e usada para filtrar pagamentos nas consultas.
// Cada campo corresponde a um query parameter na URL.
type PaymentFilter struct {
	Status          string `query:"status"`           // Filtrar por status
	RiskLevel       string `query:"risk_level"`       // Filtrar por nivel de risco
	EstablishmentID string `query:"establishment_id"` // Filtrar por estabelecimento
	CustomerID      string `query:"customer_id"`      // Filtrar por cliente
	Method          string `query:"method"`           // Filtrar por metodo
	DateFrom        string `query:"date_from"`        // Data inicial (YYYY-MM-DD)
	DateTo          string `query:"date_to"`          // Data final (YYYY-MM-DD)
	Page            int    `query:"page"`             // Pagina atual (default: 1)
	Limit           int    `query:"limit"`            // Itens por pagina (default: 20)
}
