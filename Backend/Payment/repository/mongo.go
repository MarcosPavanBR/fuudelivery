package repository

import (
	"context"
	"log"
	"time"

	"github.com/carloshomar/vercardapio/payment/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Client     *mongo.Client
	Database   *mongo.Database
	Payments   *mongo.Collection
	Chargebacks *mongo.Collection
	Wallets    *mongo.Collection
	WalletTransactions *mongo.Collection
	Evidences  *mongo.Collection
	Users      *mongo.Collection
)

func Connect() {
	cfg := config.AppConfig

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Printf("Warning: MongoDB ping failed: %v", err)
	}

	Client = client
	Database = client.Database(cfg.MongoDatabase)
	Payments = Database.Collection("payments")
	Chargebacks = Database.Collection("chargebacks")
	Wallets = Database.Collection("wallets")
	WalletTransactions = Database.Collection("wallet_transactions")
	Evidences = Database.Collection("evidences")
	Users = Database.Collection("users")

	createIndexes()
	log.Println("MongoDB connected successfully")
}

func createIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	Payments.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"order_id"}, Options: options.Index().SetUnique(true)},
		{Keys: []string{"customer_id"}},
		{Keys: []string{"establishment_id"}},
		{Keys: []string{"status"}},
		{Keys: []string{"risk_level"}},
		{Keys: []string{"created_at"}},
	})

	Chargebacks.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"payment_id"}},
		{Keys: []string{"status"}},
		{Keys: []string{"customer_id"}},
	})

	Wallets.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"user_id"}, Options: options.Index().SetUnique(true)},
	})

	WalletTransactions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"wallet_id"}},
		{Keys: []string{"created_at"}},
	})

	Evidences.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"chargeback_id"}},
	})

	Users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"email"}, Options: options.Index().SetUnique(true)},
	})
}

func MongoCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = cancel
	return ctx
}

func HexToObjectID(hex string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(hex)
}
