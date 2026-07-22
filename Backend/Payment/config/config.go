// Package config responsavel por carregar e disponibilizar
// as configuracoes do Payment Service a partir de variaveis de ambiente.
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config armazena todas as configuracoes do servico.
// Cada campo corresponde a uma variavel de ambiente.
type Config struct {
	MongoURI           string // URI de conexao com MongoDB (MONGO_URI)
	MongoDatabase      string // Nome do banco de dados (PAYMENT_MONGO_DATABASE)
	RabbitConnection   string // URL de conexao RabbitMQ (RABBIT_CONNECTION)
	RabbitPaymentQueue string // Nome da fila de pagamentos (RABBIT_PAYMENT_QUEUE)
	JWTSecret          string // Chave secreta para tokens JWT (JWT_SECRET)
	Port               string // Porta do servidor HTTP (PORT)
}

// AppConfig e a instancia global de configuracao.
// Inicializada pela funcao Load().
var AppConfig *Config

// Load carrega as variaveis de ambiente do arquivo .env
// e inicializa a variavel global AppConfig.
// Se JWT_SECRET nao estiver configurado, o servico nao sobe (seguranca).
func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable is required. Generate one with: openssl rand -hex 32")
	}

	AppConfig = &Config{
		MongoURI:           getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:      getEnv("PAYMENT_MONGO_DATABASE", "payment"),
		RabbitConnection:   getEnv("RABBIT_CONNECTION", "amqp://guest:guest@localhost:5672/"),
		RabbitPaymentQueue: getEnv("RABBIT_PAYMENT_QUEUE", "payment_queue"),
		JWTSecret:          jwtSecret,
		Port:               getEnv("PORT", "8084"),
	}
}

// getEnv retorna o valor da variavel de ambiente especificada.
// Se a variavel nao existir, retorna o valor fallback.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
