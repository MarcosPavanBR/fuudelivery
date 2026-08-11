package repository

import (
	"time"

	"github.com/carloshomar/fuudelivery/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func CreateEvidence(evidence *models.Evidence) error {
	ctx, cancel := MongoCtx()
	defer cancel()
	evidence.ID = primitive.NewObjectID()
	evidence.CreatedAt = time.Now()
	_, err := Evidences.InsertOne(ctx, evidence)
	return err
}

func GetEvidencesByChargeback(chargebackID primitive.ObjectID) ([]models.Evidence, error) {
	ctx, cancel := MongoCtx()
	defer cancel()
	cursor, err := Evidences.Find(ctx, bson.M{"chargeback_id": chargebackID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var evidences []models.Evidence
	if err := cursor.All(ctx, &evidences); err != nil {
		return nil, err
	}
	return evidences, nil
}
