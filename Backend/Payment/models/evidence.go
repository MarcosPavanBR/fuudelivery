package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EvidenceType string

const (
	EvidenceScreenshot EvidenceType = "screenshot"
	EvidenceDocument   EvidenceType = "document"
	EvidencePhoto      EvidenceType = "photo"
	EvidenceText       EvidenceType = "text"
)

type Evidence struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ChargebackID  primitive.ObjectID `bson:"chargeback_id" json:"chargeback_id"`
	Type          EvidenceType       `bson:"type" json:"type"`
	Content       string             `bson:"content" json:"content"`
	FileName      string             `bson:"file_name,omitempty" json:"file_name,omitempty"`
	FileURL       string             `bson:"file_url,omitempty" json:"file_url,omitempty"`
	UploadedBy    string             `bson:"uploaded_by" json:"uploaded_by"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
}
