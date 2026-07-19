package models

import (
	"context"
	"fmt"
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
	mongoDB := os.Getenv("MONGO_DATABASE")

	if mongoURI == "" {
		panic("MONGO_URI não configurado")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		panic(fmt.Sprintf("Falha ao conectar ao MongoDB: %v", err))
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		panic(fmt.Sprintf("Falha ao pingar MongoDB: %v", err))
	}

	fmt.Println("Conexão com o MongoDB estabelecida com sucesso!")

	MongoClient = client
	MongoDabase = client.Database(mongoDB)
}
