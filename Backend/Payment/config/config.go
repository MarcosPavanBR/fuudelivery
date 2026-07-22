package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoURI           string
	MongoDatabase      string
	RabbitConnection   string
	RabbitPaymentQueue string
	JWTSecret          string
	Port               string
}

var AppConfig *Config

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	AppConfig = &Config{
		MongoURI:           getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:      getEnv("PAYMENT_MONGO_DATABASE", "payment"),
		RabbitConnection:   getEnv("RABBIT_CONNECTION", "amqp://guest:guest@localhost:5672/"),
		RabbitPaymentQueue: getEnv("RABBIT_PAYMENT_QUEUE", "payment_queue"),
		JWTSecret:          getEnv("JWT_SECRET", "fuu-jwt-secret-2026"),
		Port:               getEnv("PORT", "8084"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
