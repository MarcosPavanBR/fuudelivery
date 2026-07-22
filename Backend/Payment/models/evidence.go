// Package models - evidence.go
// Define a estrutura de evidencias para processos de estorno.
// Evidencias sao documentos, screenshots ou textos que suportam
// a decisao de aprovar ou rejeitar um estorno.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EvidenceType classifica o tipo de evidencia.
type EvidenceType string

const (
	EvidenceScreenshot EvidenceType = "screenshot" // Captura de tela
	EvidenceDocument   EvidenceType = "document"   // Documento (PDF, imagem, etc)
	EvidencePhoto      EvidenceType = "photo"      // Fotografia
	EvidenceText       EvidenceType = "text"       // Texto livre (justificativa)
)

// Evidence representa uma evidencia anexada a um estorno.
// Uma evidencia e composta por um tipo, conteudo e opcionalmente
// um arquivo (nome + URL) para download.
type Evidence struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`          // ID unico MongoDB
	ChargebackID primitive.ObjectID `bson:"chargeback_id" json:"chargeback_id"` // ID do estorno pai
	Type         EvidenceType       `bson:"type" json:"type"`                 // Tipo da evidencia
	Content      string             `bson:"content" json:"content"`           // Conteudo textual/descricao
	FileName     string             `bson:"file_name,omitempty" json:"file_name,omitempty"` // Nome do arquivo (se houver)
	FileURL      string             `bson:"file_url,omitempty" json:"file_url,omitempty"`   // URL de download do arquivo
	UploadedBy   string             `bson:"uploaded_by" json:"uploaded_by"`   // ID do usuario que enviou
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`     // Data de criacao
}
