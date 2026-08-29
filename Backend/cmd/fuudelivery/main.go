package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"fuudelivery/pkg/config"
	"fuudelivery/pkg/middleware"
	"fuudelivery/pkg/reaper"
)

func main() {
	// Carrega variáveis de ambiente do arquivo .env (desenvolvimento)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Modo de verificação de configuração
	if len(os.Args) > 1 && os.Args[1] == "--check-config" {
		if err := checkConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL configuration error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuration OK")
		os.Exit(0)
	}

	// Carrega configuração
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL failed to load configuration: %v", err)
	}

	// Valida configuração em produção
	if cfg.Env == "production" {
		if err := cfg.Validate(); err != nil {
			log.Fatalf("FATAL configuration validation failed: %v", err)
		}
	}

	// Inicializa Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Testa conexão com Redis
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("FATAL failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	log.Printf("Connected to Redis at %s", cfg.RedisAddr)

	// Inicia Stream Reaper para cada stream crítico
	reaperCtx, reaperCancel := context.WithCancel(context.Background())
	defer reaperCancel()

	startStreamReapers(reaperCtx, redisClient, cfg)

	// Configura Fiber com timeouts e limites seguros
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		BodyLimit:    int(cfg.BodyLimit),
		ErrorHandler: customErrorHandler,
	})

	// Middleware de recovery (deve ser o primeiro)
	app.Use(middleware.Recovery())

	// Middleware de contexto seguro
	app.Use(middleware.SafeContext())

	// Health checks
	app.Get("/health/live", healthLiveHandler)
	app.Get("/health/ready", healthReadyHandler(redisClient))

	// Metrics endpoint (placeholder para OpenTelemetry)
	app.Get("/metrics", metricsHandler)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")

		// Para reapers
		reaperCancel()

		// Shutdown com timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()

		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Inicia servidor
	addr := ":" + cfg.Port
	log.Printf("Starting server on %s (env=%s)", addr, cfg.Env)

	if err := app.Listen(addr); err != nil {
		log.Fatalf("FATAL server failed to start: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// startStreamReapers inicia reapers para streams críticos
func startStreamReapers(ctx context.Context, client *redis.Client, cfg *config.Config) {
	streams := []struct {
		name  string
		group string
	}{
		{"payments:events", "payment_processor"},
		{"orders:events", "order_processor"},
		{"deliveries:dispatch", "dispatch_engine"},
		{"notifications:queue", "notification_service"},
	}

	for _, stream := range streams {
		reaperInstance := reaper.NewStreamReaper(
			client,
			stream.name,
			stream.group,
			fmt.Sprintf("reaper_%s", os.Getenv("HOSTNAME")),
			30*time.Second,  // max idle time antes de claimar
			5,               // max retries antes de DLQ
			10*time.Second,  // intervalo de verificação
		)

		go reaperInstance.Start(ctx)
		log.Printf("Started StreamReaper for %s:%s", stream.name, stream.group)
	}
}

// checkConfig valida a configuração sem iniciar o servidor
func checkConfig() error {
	requiredVars := []string{
		"JWT_SECRET",
		"DB_CONNECTION_STRING",
		"REDIS_ADDR",
	}

	if err := config.CheckRequired(requiredVars...); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Testa conexão com Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer redisClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis connection failed: %w", err)
	}

	// TODO: Testar conexão com PostgreSQL
	// db, err := database.Connect(cfg.DBConnectionString)
	// if err != nil {
	//     return fmt.Errorf("PostgreSQL connection failed: %w", err)
	// }
	// defer db.Close()

	return nil
}

// healthLiveHandler verifica se o processo está vivo
func healthLiveHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "alive",
		"time":   time.Now().UTC(),
	})
}

// healthReadyHandler verifica se o serviço está pronto para receber tráfego
func healthReadyHandler(redisClient *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		// Verifica Redis
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not_ready",
				"error":  "Redis connection failed",
			})
		}

		// TODO: Verificar PostgreSQL

		return c.JSON(fiber.Map{
			"status": "ready",
			"time":   time.Now().UTC(),
		})
	}
}

// metricsHandler retorna métricas básicas (placeholder para OpenTelemetry)
func metricsHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"service":     "fuudelivery-api",
		"version":     "1.0.0",
		"environment": os.Getenv("GO_ENV"),
		"timestamp":   time.Now().UTC(),
	})
}

// customErrorHandler trata erros de forma centralizada
func customErrorHandler(c *fiber.Ctx, err error) error {
	// Log estruturado do erro
	log.Printf("[ERROR] path=%s method=%s error=%v", c.Path(), c.Method(), err)

	// Retorna erro apropriado
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"error":   message,
		"path":    c.Path(),
		"method":  c.Method(),
		"trace_id": c.Locals("trace_id"),
	})
}
