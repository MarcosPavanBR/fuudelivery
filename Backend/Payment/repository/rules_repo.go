package repository

import (
	"github.com/carloshomar/fuudelivery/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const rulesDocID = "approval_rules"

func GetApprovalRules() (*models.ApprovalRules, error) {
	ctx, cancel := MongoCtx()
	defer cancel()
	collection := Database.Collection("approval_rules")
	var rules models.ApprovalRules
	err := collection.FindOne(ctx, bson.M{"_id": rulesDocID}).Decode(&rules)
	if err != nil {
		return nil, err
	}
	return &rules, nil
}

func SaveApprovalRules(rules *models.ApprovalRules) error {
	ctx, cancel := MongoCtx()
	defer cancel()
	collection := Database.Collection("approval_rules")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx,
		bson.M{"_id": rulesDocID},
		bson.M{"$set": rules},
		opts,
	)
	return err
}
