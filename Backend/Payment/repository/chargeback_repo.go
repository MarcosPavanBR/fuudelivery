// Package repository - chargeback_repo.go
// Funcoes de acesso a dados para a colecao de estornos (chargebacks).
package repository

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateChargeback insere um novo estorno no MongoDB.
// Gera automaticamente o ObjectID e os timestamps.
func CreateChargeback(chargeback *models.Chargeback) error {
	ctx := MongoCtx()
	chargeback.ID = primitive.NewObjectID()
	chargeback.CreatedAt = time.Now()
	chargeback.UpdatedAt = time.Now()
	_, err := Chargebacks.InsertOne(ctx, chargeback)
	return err
}

// GetChargebackByID busca um estorno pelo seu ObjectID.
func GetChargebackByID(id primitive.ObjectID) (*models.Chargeback, error) {
	ctx := MongoCtx()
	var chargeback models.Chargeback
	err := Chargebacks.FindOne(ctx, bson.M{"_id": id}).Decode(&chargeback)
	if err != nil {
		return nil, err
	}
	return &chargeback, nil
}

// UpdateChargebackStatus atualiza o status de um estorno e campos adicionais.
// Usado para aprovar, rejeitar ou escalar estornos.
func UpdateChargebackStatus(id primitive.ObjectID, status models.ChargebackStatus, updates bson.M) error {
	ctx := MongoCtx()
	updates["status"] = status
	updates["updated_at"] = time.Now()
	_, err := Chargebacks.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	return err
}

// ListChargebacks lista estornos com filtro por status e paginacao.
// Retorna a lista, total e erro (se houver).
func ListChargebacks(status string, page, limit int) ([]models.Chargeback, int64, error) {
	ctx := MongoCtx()
	query := bson.M{}
	if status != "" {
		query["status"] = status
	}

	// Conta total para paginacao
	total, err := Chargebacks.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	// Configura paginacao (default: pagina 1, 20 itens)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	skip := int64((page - 1) * limit)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}). // Mais recentes primeiro
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := Chargebacks.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var chargebacks []models.Chargeback
	if err := cursor.All(ctx, &chargebacks); err != nil {
		return nil, 0, err
	}

	return chargebacks, total, nil
}
