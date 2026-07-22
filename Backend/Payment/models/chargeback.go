// Package models - chargeback.go
// Define a estrutura de dados para estornos (chargebacks) e seus tipos.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChargebackStatus representa o status de um estorno.
type ChargebackStatus string

const (
	ChargebackPending  ChargebackStatus = "pending"  // Aguardando analise
	ChargebackApproved ChargebackStatus = "approved" // Estorno aprovado
	ChargebackRejected ChargebackStatus = "rejected" // Estorno rejeitado
	ChargebackEscalated ChargebackStatus = "escalated" // Escalado para compliance
)

// ChargebackReason representa o motivo do estorno.
type ChargebackReason string

const (
	ReasonUnauthorized ChargebackReason = "unauthorized" // Transacao nao autorizada
	ReasonNotReceived  ChargebackReason = "not_received" // Cliente nao recebeu
	ReasonDefective    ChargebackReason = "defective"   // Produto/servico defeituoso
	ReasonDuplicate    ChargebackReason = "duplicate"    // Transacao duplicada
	ReasonOther        ChargebackReason = "other"        // Outros motivos
)

// Chargeback representa um estorno/disputa de pagamento.
// Um estorno e criado quando o cliente contesta uma transacao.
// O estorno passa por um ciclo: pending -> approved/rejected/escalated
// e pode ter evidencias associadas para suportar a decisao.
type Chargeback struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`               // ID unico MongoDB
	PaymentID       primitive.ObjectID `bson:"payment_id" json:"payment_id"`          // ID do pagamento original
	PaymentOrderID  string             `bson:"payment_order_id" json:"payment_order_id"` // ID do pedido (para consulta rapida)
	CustomerID      string             `bson:"customer_id" json:"customer_id"`        // ID do cliente que solicitou
	EstablishmentID string             `bson:"establishment_id" json:"establishment_id"` // ID do estabelecimento
	Amount          float64            `bson:"amount" json:"amount"`                  // Valor do estorno (R$)
	Reason          ChargebackReason   `bson:"reason" json:"reason"`                  // Motivo da disputa
	Description     string             `bson:"description,omitempty" json:"description,omitempty"` // Descricao detalhada
	Status          ChargebackStatus   `bson:"status" json:"status"`                  // Status atual
	EvidenceCount   int                `bson:"evidence_count" json:"evidence_count"`  // Quantidade de evidencias
	AssignedTo      string             `bson:"assigned_to,omitempty" json:"assigned_to,omitempty"` // Operador responsavel
	AssignedAt      *time.Time         `bson:"assigned_at,omitempty" json:"assigned_at,omitempty"` // Quando atribuido
	ResolvedBy      string             `bson:"resolved_by,omitempty" json:"resolved_by,omitempty"` // Quem resolveu
	ResolvedAt      *time.Time         `bson:"resolved_at,omitempty" json:"resolved_at,omitempty"` // Quando resolveu
	Resolution      string             `bson:"resolution,omitempty" json:"resolution,omitempty"` // Texto da decisao
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`          // Data de criacao
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`          // Data da ultima atualizacao
}
