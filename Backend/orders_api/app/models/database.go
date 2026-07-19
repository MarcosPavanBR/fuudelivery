package models

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB          *gorm.DB
	MongoClient *mongo.Client
	MongoDabase *mongo.Database
)

const maxRetries = 5
const retryInterval = 5 * time.Second

func ConnectPostgresDatabase() {
	dsn := os.Getenv("DB_CONNECTION_STRING")
	if dsn == "" {
		panic("DB_CONNECTION_STRING não configurado")
	}

	var database *gorm.DB
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		database, err = gorm.Open(postgres.Open(dsn), &gorm.Config{PrepareStmt: false})
		if err == nil {
			break
		}

		time.Sleep(retryInterval)
	}

	if err != nil {
		panic(fmt.Sprintf("Falha ao conectar ao banco de dados PostgreSQL após %d tentativas", maxRetries))
	}

	database.AutoMigrate(
		&Category{},
		&CategoryProducts{},
		&Product{},
		&Additional{},
		&AdditionalProducts{},
		&OrderItem{},
		&Order{},
		&Delivery{},
		&Coupon{},
		&CouponUsage{},
		&LoyaltyPoints{},
		&LoyaltyTransaction{},
		&Review{},
	)

	DB = database
}

func ConnectMongoDatabase() {
	mongoURI := os.Getenv("MONGO_URI")
	mongoDB := os.Getenv("MONGO_DATABASE")

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
		log.Printf("Falha ao pingar MongoDB (server continuará, mas MongoDB pode estar indisponível): %v", err)
	}

	MongoClient = client
	MongoDabase = client.Database(mongoDB)
	log.Println("Conexão com o MongoDB estabelecida com sucesso!")
}
