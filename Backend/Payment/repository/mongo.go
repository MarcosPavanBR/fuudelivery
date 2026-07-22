// Package repository gerencia a conexao e operacoes com o MongoDB.
// Cada arquivo responsavel por um dominio (payment, chargeback, wallet, etc.)
// funcoes de CRUD e consultas complexas.
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

// Variaveis globais que representam as colecoes do MongoDB.
// Inicializadas na funcao Connect().
var (
	Client              *mongo.Client         // Cliente MongoDB
	Database            *mongo.Database       // Banco de dados ativo
	Payments            *mongo.Collection     // Colecao de pagamentos
	Chargebacks         *mongo.Collection     // Colecao de estornos
	Wallets             *mongo.Collection     // Colecao de carteiras
	WalletTransactions  *mongo.Collection     // Colecao de transacoes das carteiras
	Evidences           *mongo.Collection     // Colecao de evidencias
	Users               *mongo.Collection     // Colecao de usuarios
)

// Connect estabelece a conexao com o MongoDB, inicializa as colecoes
// e cria os indices necessarios para performance.
// Em caso de falha, o servico e encerrado com log.Fatal.
func Connect() {
	cfg := config.AppConfig

	// Timeout de 30 segundos para conexao inicial
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Configura o cliente MongoDB com a URI do .env
	clientOptions := options.Client().ApplyURI(cfg.MongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Verifica se a conexao esta funcional (ping)
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Printf("Warning: MongoDB ping failed: %v", err)
	}

	// Inicializa variaveis globais com as colecoes
	Client = client
	Database = client.Database(cfg.MongoDatabase)
	Payments = Database.Collection("payments")
	Chargebacks = Database.Collection("chargebacks")
	Wallets = Database.Collection("wallets")
	WalletTransactions = Database.Collection("wallet_transactions")
	Evidences = Database.Collection("evidences")
	Users = Database.Collection("users")

	// Cria indices para otimizar consultas
	createIndexes()
	log.Println("MongoDB connected successfully")
}

// createIndexes cria indices nas colecoes para acelerar consultas.
// Indices em campos de filtro (status, customer_id, etc.) melhoram
// significativamente a performance em colecoes grandes.
func createIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Indices da colecao payments
	// order_id e unico para evitar pagamentos duplicados
	Payments.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"order_id"}, Options: options.Index().SetUnique(true)},
		{Keys: []string{"customer_id"}},
		{Keys: []string{"establishment_id"}},
		{Keys: []string{"status"}},
		{Keys: []string{"risk_level"}},
		{Keys: []string{"created_at"}},
	})

	// Indices da colecao chargebacks
	Chargebacks.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"payment_id"}},
		{Keys: []string{"status"}},
		{Keys: []string{"customer_id"}},
	})

	// Indices da colecao wallets
	// user_id e unico para garantir uma carteira por usuario
	Wallets.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"user_id"}, Options: options.Index().SetUnique(true)},
	})

	// Indices da colecao wallet_transactions
	WalletTransactions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"wallet_id"}},
		{Keys: []string{"created_at"}},
	})

	// Indices da colecao evidences
	Evidences.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"chargeback_id"}},
	})

	// Indices da colecao users
	Users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"email"}, Options: options.Index().SetUnique(true)},
	})
}

// MongoCtx retorna um contexto com timeout de 5 segundos para operacoes MongoDB.
// Evita que requisicoes fiquem travadas em caso de lentidao do banco.
func MongoCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = cancel
	return ctx
}

// HexToObjectID converte uma string hexadecimal (24 caracteres) em ObjectID do MongoDB.
// Usado para extrair IDs das URLs (ex: /payments/:id).
func HexToObjectID(hex string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(hex)
}
