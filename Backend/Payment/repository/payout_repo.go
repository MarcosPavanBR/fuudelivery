// Package repository - payout_repo.go
// Funcoes de acesso a dados para solicitacoes de saque (Pix payouts).
package repository

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CreatePayout insere uma nova solicitacao de saque no MongoDB.
func CreatePayout(payout *models.PayoutRequest) error {
	ctx := MongoCtx()
	payout.ID = primitive.NewObjectID()
	payout.CreatedAt = time.Now()
	payout.UpdatedAt = time.Now()
	_, err := Payouts.InsertOne(ctx, payout)
	return err
}

// UpdatePayoutStatus atualiza o status de uma solicitacao de saque.
func UpdatePayoutStatus(id primitive.ObjectID, status models.PayoutStatus, gatewayID, failureReason string) error {
	ctx := MongoCtx()
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}
	if gatewayID != "" {
		update["$set"].(bson.M)["gateway_id"] = gatewayID
	}
	if failureReason != "" {
		update["$set"].(bson.M)["failure_reason"] = failureReason
	}
	_, err := Payouts.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

// GetPayoutsByUser retorna o historico de saques de um usuario.
func GetPayoutsByUser(userID string, limit int) ([]models.PayoutRequest, error) {
	ctx := MongoCtx()
	if limit < 1 || limit > 100 {
		limit = 20
	}

	cursor, err := Payouts.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var payouts []models.PayoutRequest
	if err := cursor.All(ctx, &payouts); err != nil {
		return nil, err
	}
	return payouts, nil
}
