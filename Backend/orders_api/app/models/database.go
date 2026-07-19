package models

import (
	"context"
	"fmt"
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
		database, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
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
		panic("MONGO_URI não configurado")
	}

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		panic(fmt.Sprintf("Falha ao conectar ao banco de dados MongoDB: %v", err))
	}

	err = client.Ping(context.Background(), nil)
	if err != nil {
		panic(fmt.Sprintf("Falha ao pingar o servidor MongoDB: %v", err))
	}

	fmt.Println("Conexão com o MongoDB estabelecida com sucesso!")

	MongoClient = client
	MongoDabase = client.Database(mongoDB)
}
