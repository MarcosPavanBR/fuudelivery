package repository

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetWallet(userID string) (*models.Wallet, error) {
	ctx := MongoCtx()
	var wallet models.Wallet
	err := Wallets.FindOne(ctx, bson.M{"user_id": userID}).Decode(&wallet)
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func CreateWallet(wallet *models.Wallet) error {
	ctx := MongoCtx()
	wallet.ID = primitive.NewObjectID()
	wallet.CreatedAt = time.Now()
	wallet.UpdatedAt = time.Now()
	_, err := Wallets.InsertOne(ctx, wallet)
	return err
}

func UpdateWalletBalance(userID string, newBalance float64) error {
	ctx := MongoCtx()
	_, err := Wallets.UpdateOne(ctx,
		bson.M{"user_id": userID},
		bson.M{"$set": bson.M{"balance": newBalance, "updated_at": time.Now()}},
	)
	return err
}

func CreateWalletTransaction(tx *models.WalletTransaction) error {
	ctx := MongoCtx()
	tx.ID = primitive.NewObjectID()
	tx.CreatedAt = time.Now()
	_, err := WalletTransactions.InsertOne(ctx, tx)
	return err
}

func GetWalletTransactions(walletID primitive.ObjectID, limit int) ([]models.WalletTransaction, error) {
	ctx := MongoCtx()
	if limit < 1 || limit > 100 {
		limit = 50
	}

	cursor, err := WalletTransactions.Find(ctx,
		bson.M{"wallet_id": walletID},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var txs []models.WalletTransaction
	if err := cursor.All(ctx, &txs); err != nil {
		return nil, err
	}

	return txs, nil
}
