package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	// Models (database initialization)
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	chatModels "github.com/carloshomar/fuudelivery/chat_api/app/models"
	deliveryModels "github.com/carloshomar/fuudelivery/delivery_api/app/models"
	ordersModels "github.com/carloshomar/fuudelivery/orders_api/app/models"
	paymentModels "github.com/carloshomar/fuudelivery/payment_api/app/models"

	// Handlers
	authHandlers "github.com/carloshomar/fuudelivery/auth_api/app/handlers"
	ordersHandlers "github.com/carloshomar/fuudelivery/orders_api/app/handlers"
	paymentHandlers "github.com/carloshomar/fuudelivery/payment_api/app/handlers"

	// Middleware
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"

	// Dispatch engine

	// Batch expiry
	orderServices "github.com/carloshomar/fuudelivery/orders_api/app/services"

	// Queue + Health + Upload + Metrics + Search
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/carloshomar/fuudelivery/pkg/health"
	"github.com/carloshomar/fuudelivery/pkg/metrics"
	"github.com/carloshomar/fuudelivery/pkg/queue"
	"github.com/carloshomar/fuudelivery/pkg/search"
	"github.com/carloshomar/fuudelivery/pkg/upload"
)

func main() {
	godotenv.Load()

	// Valida ambiente antes de inicializar
	validateRequiredEnv()

	// Payment router (initialized before route setup)
	var paymentRouter *gateway.Router

	// Create Fiber app EARLY so /health is available before DB connections.
	// Render health check has a 30s timeout; DB connections (5 modules × 5
	// retries × 5s) can take up to 125s. Registering /health first lets the
	// deploy succeed while DBs connect in the background.
	app := fiber.New(fiber.Config{
		Prefork:       false,
		CaseSensitive: true,
		StrictRouting: false,
		// Configura proxy confiavel (Render usa range interno 10.0.0.0/8).
		// ProxyHeader define de qual header ler o IP real do cliente.
		// EnableTrustedProxyCheck: garante que so confia em proxies configurados.
		// NOTA: NAO usar 0.0.0.0/0 — aceitaria X-Forwarded-For spoofed de
		// qualquer origem, anulando o rate limiting por IP.
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		ProxyHeader:             "X-Forwarded-For",
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())

	// Metricas HTTP (contadores por rota+status) — expostas em GET /metrics
	app.Use(metrics.Middleware())

	// CORS: origens permitidas. Lista canônica em references/URLS.md.
	// Comportamento:
	//   - ALLOWED_ORIGINS (env, render.yaml) SOMA com os defaults — nunca
	//     remove os domínios de produção ao adicionar uma origem nova.
	//   - Em desenvolvimento (GO_ENV != production) libera-se localhost em
	//     qualquer porta (isLocalDevOrigin). Em produção NENHUM localhost
	//     vale: credentials + localhost é superfície de ataque CSRF.
	//   - Domínios de preview (ex.: *.daytonaproxy01.net) devem ser
	//     adicionados EXPLICITAMENTE via ALLOWED_ORIGINS, nunca como wildcard.
	defaultOrigins := []string{
		"https://fuudelivery-web.onrender.com",
		"https://fuudelivery-admin-lv7f.onrender.com",
	}
	allowedOrigins := append([]string{}, defaultOrigins...)
	if extra := os.Getenv("ALLOWED_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			if o = strings.TrimSpace(o); o != "" {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
	}
	// Remove duplicatas preservando a ordem.
	seen := make(map[string]struct{}, len(allowedOrigins))
	unique := allowedOrigins[:0]
	for _, o := range allowedOrigins {
		if _, dup := seen[o]; dup {
			continue
		}
		seen[o] = struct{}{}
		unique = append(unique, o)
	}
	allowedOrigins = unique

	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(allowedOrigins, ","),
		AllowOriginsFunc: isLocalDevOrigin,
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-CSRF-Token",
	}))

	// Content Security Policy — previne execução de scripts arbitrários.
	app.Use(func(c *fiber.Ctx) error {
		c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; connect-src 'self' wss: https:; font-src 'self' data:")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		return c.Next()
	})

	// CSRF protection — double-submit cookie: em mutações de browser
	// (cookie csrf_token/access_token presentes), o header X-CSRF-Token
	// precisa igualar o cookie. Um site malicioso consegue fazer o browser
	// enviar o cookie, mas não consegue ler o valor para colocar no header —
	// ataque bloqueado. Requisições Bearer-only (apps mobile) e webhooks de
	// gateway ficam fora do escopo. Implementação/testes: cmd/fuudelivery/csrf.go
	app.Use(csrfMiddleware)

	// Health check — reuses the Redis client from the queue singleton
	// HTTP 503 when Postgres is down OR no payment gateway is configured.
	// Redis/batches degradation returns HTTP 200 with status "degraded".
	app.Get("/health", func(c *fiber.Ctx) error {
		redisClient := queue.GetClient()

		postgresCheck := health.DatabaseCheck(models.DB)
		redisCheck := health.RedisCheck(redisClient)
		redisGeoCheck := health.RedisGeoCheck(redisClient)
		batchesCheck := health.BatchCheck(ordersModels.DB)
		// Fonte da verdade é a cadeia montada no router, não as env vars:
		// credencial presente com construtor falhando deixa a cadeia vazia,
		// e o /health precisa dizer "down" nesse caso (503), não "up".
		//
		// O nil check não é decorativo: este handler é registrado de
		// propósito ANTES do resto da inicialização (para o Render conseguir
		// bater no /health durante os até 125s de conexão com os bancos), e
		// paymentRouter só é atribuído lá embaixo. Chamar Gateways() num
		// *Router nil daria panic dentro do próprio health check.
		var registeredGateways []string
		if paymentRouter != nil {
			registeredGateways = paymentRouter.Gateways()
		}
		gatewaysCheck := health.GatewayCheck(registeredGateways)

		// On cold start (DB not yet initialized), return 200 so Render health
		// checks pass during the DB initialization window (up to 125s). Vale
		// até a inicialização INTEIRA terminar (readiness.go), não só o banco
		// do auth.
		if models.DB == nil || stillStarting(time.Now()) {
			return c.Status(200).JSON(fiber.Map{
				"status":  "starting",
				"service": "fuudelivery",
				"version": "1.0.0",
				"message": "database initializing, please retry",
				"time":    time.Now().UTC(),
			})
		}

		// Critical checks: Postgres + at least one payment gateway.
		criticalStatus := health.OverallStatus(postgresCheck, gatewaysCheck)
		// All checks: includes Redis, batches and gateways
		allStatus := health.OverallStatus(postgresCheck, redisCheck, redisGeoCheck, batchesCheck, gatewaysCheck)

		statusCode := 200
		if criticalStatus != "up" {
			statusCode = 503
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"status":  allStatus,
			"service": "fuudelivery",
			"version": "1.0.0",
			"checks": fiber.Map{
				"postgres":         postgresCheck,
				"redis":            redisCheck,
				"redis_geo":        redisGeoCheck,
				"batches":          batchesCheck,
				"payment_gateways": gatewaysCheck,
			},
			"time": time.Now().UTC(),
		})
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "fuudelivery"})
	})

	// Metricas em formato Prometheus text (para Prometheus/Grafana/BetterStack/UptimeRobot)
	// GET /metrics — protegido por bearer token (env METRICS_TOKEN).
	//
	// Em PRODUÇÃO, sem METRICS_TOKEN configurado o endpoint NÃO serve (403).
	// A regra completa e o porquê estão em metrics_auth.go.
	app.Get("/metrics", func(c *fiber.Ctx) error {
		ok, motivo := metricsAuthorized(
			os.Getenv("GO_ENV"),
			os.Getenv("METRICS_TOKEN"),
			c.Get("Authorization"),
		)
		if !ok {
			if motivo == metricsDeniedNoToken {
				// LOUD: quem subiu em produção precisa saber que as métricas
				// estão inacessíveis por falta de configuração, e não por bug.
				log.Printf("[METRICS] 403 — %s. Configure METRICS_TOKEN para habilitar o endpoint.", motivo)
			}
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
		}
		return metrics.Handler(c)
	})

	// Busca full-text basica (Fase 3 — construcao nova): GET /search?q=...
	// Busca estabelecimentos e produtos no PostgreSQL (ILIKE + scoring).
	app.Get("/search", search.NewHandler(func() *gorm.DB { return models.DB }))

	// Pedido ↔ entrega (fila do entregador). Antes das rotas: nenhuma
	// mudança de status pode chegar sem a ponte ligada.
	wireCourierFlow()

	// Mount all routes
	setupWebSocketRoutes(app)
	setupAuthRoutes(app)
	setupOrdersRoutes(app)
	setupDeliveryRoutes(app)
	setupZoneRoutes(app)
	setupDispatchRoutes(app)
	setupSponsoredRoutes(app)
	setupSubscriptionRoutes(app)
	// Initialize payment gateway router with fallback chain.
	//
	// Os erros dos construtores NÃO podem ser descartados. abacatepay e
	// mercadopago retornam (nil, err) quando falta credencial; passar esse
	// ponteiro nil direto pro NewRouter (que recebe a interface Gateway)
	// cria um "typed nil": a interface guarda (tipo=*XGateway, valor=nil) e
	// portanto é != nil. O gateway entra no roteador, SupportsMethod até
	// funciona (receiver nil), mas CreateTransaction desreferencia g.client
	// e dá panic — abortando a cadeia de fallback inteira. Por isso a
	// checagem é no erro, antes da conversão pra interface: um `if gw != nil`
	// depois de virar interface não pegaria.
	paymentRouter = gateway.NewRouter(buildPaymentGateways()...)
	paymentRouter.SetStrategy(gateway.StrategyOrdered)
	setupPaymentRoutes(app, paymentRouter)
	setupChatRoutes(app)

	// Upload de imagens (Supabase Storage)
	app.Post("/upload/:entity", upload.HandleImageUpload)
	app.Post("/upload/:entity/:entityId", upload.HandleImageUpload)

	// Initialize databases in background goroutine so /health is available
	// immediately. Render health check has a 30s timeout; DB connections
	// (5 modules × 5 retries × 5s) can take up to 125s.
	go func() {
		models.ConnectDatabase()
		ordersModels.ConnectPostgresDatabase()
		deliveryModels.ConnectPostgresDatabase()
		paymentModels.ConnectPostgresDatabase()
		chatModels.ConnectPostgresDatabase()

		// Initialize message queue
		queue.Init()

		// Initialize storage (Supabase Storage para upload de imagens)
		upload.Init()

		// Start batch expiry job
		batchExpiryConfig := orderServices.DefaultBatchExpiryConfig()
		batchExpiryManager := orderServices.NewBatchExpiryManager(ordersModels.DB, batchExpiryConfig)
		batchExpiryManager.Start()

		// Wire loyalty points
		paymentHandlers.OnPaymentApproved = ordersHandlers.EarnPointsForOrder

		// Initialize dispatch engine (courier store + matching engine + handler)
		initDispatchEngine(models.DB)

		// Tudo conectado: o /health passa a responder com os checks reais.
		initDone.Store(true)

		// Reconciliação de pagamentos: a rede de segurança do caminho do
		// dinheiro. Sobe AQUI, dentro da goroutine de inicialização, porque
		// depende dos bancos conectados — e a primeira passada acontece logo
		// na subida de propósito: o restart pode ter sido exatamente o que
		// interrompeu uma liquidação no meio.
		go paymentHandlers.StartPaymentReconciliation(5 * time.Minute)
	}()

	// Tickets de WebSocket emitidos e nunca consumidos ficariam no mapa para
	// sempre: sem esta limpeza, qualquer usuário logado cresce a memória do
	// processo chamando POST /auth/ws-ticket em loop.
	cleanupWSTickets()

	// Start background workers
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startQueueListeners()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		startRefreshTokenCleanup() // limpa tokens expirados a cada 24h
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		startRateLimitCleanup()
	}()

	// Graceful shutdown
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutting down...")
		queue.CloseQueue()
		app.ShutdownWithTimeout(10 * time.Second)
		wg.Wait()
		log.Println("All background workers stopped")
	}()

	log.Printf("FUUDELIVERY server starting on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// startRefreshTokenCleanup remove refresh tokens expirados do banco uma vez
// por dia. Sem isso a tabela refresh_tokens cresceria indefinidamente.
func startRefreshTokenCleanup() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		middlewares.CleanupExpiredRefreshTokens()
		authHandlers.CleanupExpiredPasswordResets() // códigos expirados/usados do reset assistido
	}
}
