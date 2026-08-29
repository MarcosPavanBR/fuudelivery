package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config representa a configuração global da aplicação
type Config struct {
	Env          string
	Port         string
	JWTSecret    string
	JWTIssuer    string
	JWTAudience  string
	JWTAccessTTL time.Duration
	JWTRefreshTTL time.Duration

	// Database
	DBConnectionString string
	DBMaxOpenConns     int
	DBMaxIdleConns     int
	DBConnMaxLifetime  time.Duration
	DBConnMaxIdleTime  time.Duration

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Timeouts
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	BodyLimit    int64

	// Logging
	LogLevel string

	// Gateways de Pagamento
	PagarMeAPIKey        string
	PagarMeEncryptionKey string
	AsaasAPIKey          string
	AbacatePayAPIKey     string
	MercadoPagoToken     string

	// Observabilidade
	OTELExporterEndpoint string
}

// Load carrega a configuração a partir de variáveis de ambiente
func Load() (*Config, error) {
	cfg := &Config{
		Env:          getEnv("GO_ENV", "development"),
		Port:         getEnv("PORT", "3000"),
		JWTSecret:    getEnv("JWT_SECRET", ""),
		JWTIssuer:    getEnv("JWT_ISSUER", "fuudelivery.auth"),
		JWTAudience:  getEnv("JWT_AUDIENCE", "fuudelivery.api"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		OTELExporterEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}

	// JWT TTLs
	accessTTL := getEnv("JWT_ACCESS_TTL", "15m")
	refreshTTL := getEnv("JWT_REFRESH_TTL", "7d")

	var err error
	cfg.JWTAccessTTL, err = time.ParseDuration(accessTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_TTL: %w", err)
	}

	cfg.JWTRefreshTTL, err = time.ParseDuration(refreshTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_TTL: %w", err)
	}

	// Database
	cfg.DBConnectionString = getEnv("DB_CONNECTION_STRING", "")
	cfg.DBMaxOpenConns = getIntEnv("DB_MAX_OPEN_CONNS", 50)
	cfg.DBMaxIdleConns = getIntEnv("DB_MAX_IDLE_CONNS", 10)

	connMaxLifetime := getEnv("DB_CONN_MAX_LIFETIME", "15m")
	cfg.DBConnMaxLifetime, err = time.ParseDuration(connMaxLifetime)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_CONN_MAX_LIFETIME: %w", err)
	}

	connMaxIdleTime := getEnv("DB_CONN_MAX_IDLE_TIME", "5m")
	cfg.DBConnMaxIdleTime, err = time.ParseDuration(connMaxIdleTime)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_CONN_MAX_IDLE_TIME: %w", err)
	}

	// Redis
	cfg.RedisAddr = getEnv("REDIS_ADDR", "localhost:6379")
	cfg.RedisPassword = getEnv("REDIS_PASSWORD", "")
	cfg.RedisDB = getIntEnv("REDIS_DB", 0)

	// Timeouts
	readTimeout := getEnv("READ_TIMEOUT", "10s")
	cfg.ReadTimeout, err = time.ParseDuration(readTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid READ_TIMEOUT: %w", err)
	}

	writeTimeout := getEnv("WRITE_TIMEOUT", "10s")
	cfg.WriteTimeout, err = time.ParseDuration(writeTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid WRITE_TIMEOUT: %w", err)
	}

	idleTimeout := getEnv("IDLE_TIMEOUT", "60s")
	cfg.IdleTimeout, err = time.ParseDuration(idleTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid IDLE_TIMEOUT: %w", err)
	}

	cfg.BodyLimit = getInt64Env("BODY_LIMIT", 4*1024*1024) // 4MB

	// Gateways
	cfg.PagarMeAPIKey = getEnv("PAGARME_API_KEY", "")
	cfg.PagarMeEncryptionKey = getEnv("PAGARME_ENCRYPTION_KEY", "")
	cfg.AsaasAPIKey = getEnv("ASAAS_API_KEY", "")
	cfg.AbacatePayAPIKey = getEnv("ABACATEPAY_API_KEY", "")
	cfg.MercadoPagoToken = getEnv("MERCADOPAGO_ACCESS_TOKEN", "")

	return cfg, nil
}

// Validate valida as configurações obrigatórias
func (c *Config) Validate() error {
	var missing []string

	if c.Env == "production" {
		if c.JWTSecret == "" {
			missing = append(missing, "JWT_SECRET")
		}
		if c.DBConnectionString == "" {
			missing = append(missing, "DB_CONNECTION_STRING")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	return nil
}

// CheckRequired verifica se todas as variáveis obrigatórias estão presentes
func CheckRequired(vars ...string) error {
	var missing []string
	for _, v := range vars {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getIntEnv(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}

	return intValue
}
