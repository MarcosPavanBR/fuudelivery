package models

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	MongoClient *mongo.Client
	MongoDabase *mongo.Database
)

func ConnectMongoDatabase() {
	mongoURI := os.Getenv("MONGO_URI")
	mongoDB := os.Getenv("PAYMENT_MONGO_DATABASE")

	if mongoURI == "" {
		log.Println("MONGO_URI não configurado, MongoDB indisponível")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURI).SetServerSelectionTimeout(30 * time.Second)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("Falha ao conectar ao MongoDB: %v", err)
		return
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Printf("Falha ao pingar MongoDB (server continuará): %v", err)
	}

	MongoClient = client
	MongoDabase = client.Database(mongoDB)
	log.Println("Conexão com o MongoDB estabelecida com sucesso!")
}
