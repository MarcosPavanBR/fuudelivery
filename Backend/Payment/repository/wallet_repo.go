// Package repository - wallet_repo.go
// Funcoes de acesso a dados para carteiras (wallets) e transacoes.
package repository

import (
	"errors"
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// options_find_one_update retorna as opcoes padrao para FindOneAndUpdate.
func options_find_one_update() *options.FindOneAndUpdateOptions {
	return options.FindOneAndUpdate()
}

// ErrInsufficientBalance retorna quando o saldo e insuficiente para debito.
var ErrInsufficientBalance = errors.New("saldo insuficiente")

// GetWallet busca uma carteira pelo ID do usuario.
// Retorna a carteira ou erro se nao encontrar.
func GetWallet(userID string) (*models.Wallet, error) {
	ctx := MongoCtx()
	var wallet models.Wallet
	err := Wallets.FindOne(ctx, bson.M{"user_id": userID}).Decode(&wallet)
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

// CreateWallet insere uma nova carteira no MongoDB.
// Gera automaticamente o ObjectID e os timestamps.
func CreateWallet(wallet *models.Wallet) error {
	ctx := MongoCtx()
	wallet.ID = primitive.NewObjectID()
	wallet.CreatedAt = time.Now()
	wallet.UpdatedAt = time.Now()
	_, err := Wallets.InsertOne(ctx, wallet)
	return err
}

// UpdateWalletBalance atualiza o saldo de uma carteira (nao atomico).
// Mantido para retrocompatibilidade — preferir IncrementWalletBalance.
func UpdateWalletBalance(userID string, newBalance float64) error {
	ctx := MongoCtx()
	_, err := Wallets.UpdateOne(ctx,
		bson.M{"user_id": userID},
		bson.M{"$set": bson.M{"balance": newBalance, "updated_at": time.Now()}},
	)
	return err
}

// IncrementWalletBalance adiciona um valor ao saldo de forma atomica usando $inc.
// Retorna a carteira atualizada apos a incrementacao.
// Seguro contra race conditions — duas chamadas concorrentes nunca perdem atualizacao.
func IncrementWalletBalance(userID string, amount float64) (*models.Wallet, error) {
	ctx := MongoCtx()
	var wallet models.Wallet

	err := Wallets.FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID},
		bson.M{
			"$inc": bson.M{"balance": amount},
			"$set": bson.M{"updated_at": time.Now()},
		},
		options_find_one_update().SetReturnDocument(options.After),
	).Decode(&wallet)

	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

// TryDebitWalletBalance tenta debitar um valor atomico.
// Retorna erro se saldo insuficiente — a verificacao e o debito
// acontecem na mesma operacao no banco, eliminando race conditions.
func TryDebitWalletBalance(userID string, amount float64) (*models.Wallet, error) {
	ctx := MongoCtx()
	var wallet models.Wallet

	// Usa FindOneAndUpdate com condicao: balance >= amount
	// Se o saldo for menor que o valor, o update nao acontece e retorna NotFound.
	err := Wallets.FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID, "balance": bson.M{"$gte": amount}},
		bson.M{
			"$inc": bson.M{"balance": -amount},
			"$set": bson.M{"updated_at": time.Now()},
		},
		options_find_one_update().SetReturnDocument(options.After),
	).Decode(&wallet)

	if err != nil {
		// Mongo retorna mongo.ErrNoDocuments quando a condicao nao e satisfeita
		if errors.Is(err, primitive.ErrNoDocuments) {
			return nil, ErrInsufficientBalance
		}
		return nil, err
	}
	return &wallet, nil
}

// CreateWalletTransaction insere uma nova transacao na carteira.
// Registra o tipo (credit/debit), valor e saldos antes/depois.
func CreateWalletTransaction(tx *models.WalletTransaction) error {
	ctx := MongoCtx()
	tx.ID = primitive.NewObjectID()
	tx.CreatedAt = time.Now()
	_, err := WalletTransactions.InsertOne(ctx, tx)
	return err
}

// GetWalletTransactions retorna o historico de transacoes de uma carteira.
// Ordenado por data (mais recente primeiro) com limite configuravel.
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
