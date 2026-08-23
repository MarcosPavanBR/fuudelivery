package models

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB é a conexão GORM com o Postgres (Supabase).
// CORTE 2 (banco-único): chat_messages migrou de Mongo para Postgres —
// esta conexão é agora a fonte primária do chat_api.
var DB *gorm.DB

// MongoClient / MongoDabase permanecem apenas para o dual-write temporário
// (escrita legada best-effort durante a transição). Remover quando o Mongo
// for desligado.
var (
	MongoClient *mongo.Client
	MongoDabase *mongo.Database
)

const maxRetries = 5
const retryInterval = 5 * time.Second

// ConnectPostgresDatabase conecta ao Postgres com retry e roda o AutoMigrate
// do ChatMessage. Mesmo padrão de orders_api/app/models/database.go.
func ConnectPostgresDatabase() {
	dsn := os.Getenv("DB_CONNECTION_STRING")
	if dsn == "" {
		log.Println("[CHAT] DB_CONNECTION_STRING não configurado — chat em Postgres indisponível")
		return
	}

	var database *gorm.DB
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		pgDSN := dsn
		if !strings.Contains(pgDSN, "default_query_exec_mode") {
			if strings.Contains(pgDSN, "?") {
				pgDSN += "&default_query_exec_mode=simple_protocol"
			} else {
				pgDSN += "?default_query_exec_mode=simple_protocol"
			}
		}

		database, err = gorm.Open(postgres.Open(pgDSN), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("[CHAT] Tentativa %d/%d falhou ao conectar Postgres: %v", attempt, maxRetries, err)
		time.Sleep(retryInterval)
	}

	if err != nil {
		log.Printf("[CHAT][CRITICAL] Falha ao conectar Postgres após %d tentativas: %v", maxRetries, err)
		return
	}

	DB = database

	// AutoMigrate garante a tabela mesmo se sql/04 ainda não rodou no banco.
	if err := DB.AutoMigrate(&ChatMessage{}); err != nil {
		log.Printf("[CHAT][CRITICAL] Falha no AutoMigrate de chat_messages: %v", err)
		return
	}

	fmt.Println("Conexão com o PostgreSQL (chat_api) estabelecida com sucesso!")
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

	if err := client.Ping(ctx, nil); err != nil {
		log.Printf("Falha ao pingar MongoDB (server continuará): %v", err)
	}

	MongoClient = client
	MongoDabase = client.Database(mongoDB)
	log.Println("Conexão com o MongoDB estabelecida com sucesso!")
}
