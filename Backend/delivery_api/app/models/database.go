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

var (
	// DB é a conexão Postgres (GORM) — fonte da verdade desde o corte 3
	// da migração banco-único (docs/ARQUITETURA-BANCO-UNICO.md).
	DB *gorm.DB

	// MongoClient/MongoDabase permanecem apenas para o dual-write legado
	// (best-effort) durante a transição. Quando o Mongo for desligado,
	// remover ConnectMongoDatabase e as chamadas de dual-write nos handlers.
	MongoClient *mongo.Client
	MongoDabase *mongo.Database
)

const pgMaxRetries = 5
const pgRetryInterval = 5 * time.Second

// ConnectPostgresDatabase conecta ao Supabase/Postgres usando a mesma env var
// do resto do monorepo (DB_CONNECTION_STRING) e o mesmo padrão de retry do
// orders_api — manter os dois iguais facilita manutenção.
func ConnectPostgresDatabase() {
	dsn := os.Getenv("DB_CONNECTION_STRING")
	if dsn == "" {
		panic("DB_CONNECTION_STRING não configurado")
	}

	var database *gorm.DB
	var err error

	for attempt := 1; attempt <= pgMaxRetries; attempt++ {
		pgDSN := dsn
		if !strings.Contains(pgDSN, "default_query_exec_mode") {
			if strings.Contains(pgDSN, "?") {
				pgDSN += "&default_query_exec_mode=simple_protocol"
			} else {
				pgDSN += "?default_query_exec_mode=simple_protocol"
			}
		}
		database, err = gorm.Open(postgres.Open(pgDSN), &gorm.Config{PrepareStmt: false})
		if err == nil {
			break
		}
		log.Printf("[DELIVERY-DB] tentativa %d/%d falhou: %v", attempt, pgMaxRetries, err)
		time.Sleep(pgRetryInterval)
	}

	if err != nil {
		panic(fmt.Sprintf("Falha ao conectar ao PostgreSQL após %d tentativas", pgMaxRetries))
	}

	// Limites de pool (ver database.go do auth_api): 5 módulos compartilham
	// o mesmo Supabase — cada pool precisa de teto.
	if sqlDB, sErr := database.DB(); sErr == nil {
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	// O schema oficial vive nos scripts SQL (sql/02_dominio_entrega.sql).
	// O AutoMigrate aqui é uma rede de segurança para ambientes novos/dev —
	// em produção quem manda é o run_all.sh.
	if err := database.AutoMigrate(&DeliverySolicitation{}); err != nil {
		panic(fmt.Sprintf("Falha no AutoMigrate de delivery_solicitations: %v", err))
	}

	DB = database
	log.Println("Conexão com o PostgreSQL estabelecida com sucesso!")
}

func ConnectMongoDatabase() {
	mongoURI := os.Getenv("MONGO_URI")
	mongoDB := os.Getenv("MONGO_DATABASE")

	if mongoURI == "" {
		// LEGADO (dual-write): sem MONGO_URI o dual-write é simplesmente
		// pulado — não é mais erro fatal desde o corte 3.
		log.Println("MONGO_URI não configurado, dual-write MongoDB desativado")
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
