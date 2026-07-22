// Package main e o ponto de entrada do microservico Payment Service.
//
// Este servico e responsavel por:
// - Processar pagamentos recebidos do gateway (AbacatePay)
// - Calcular score de risco para cada pagamento
// - Tomar decisoes de aprovacao (auto, manual, compliance, bloqueado)
// - Gerenciar carteiras (wallets) de restaurantes e entregadores
// - Processar estornos (chargebacks) e suas evidencias
// - Consumir mensagens da fila RabbitMQ para confirmar creditos
//
// Porta padrao: 8084
// Endpoints de saude: GET /health
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"github.com/carloshomar/vercardapio/payment/config"
	"github.com/carloshomar/vercardapio/payment/consumers"
	"github.com/carloshomar/vercardapio/payment/handlers"
	"github.com/carloshomar/vercardapio/payment/middleware"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

// main inicializa o servico Payment.
// Ordem de inicializacao:
// 1. Carrega configuracoes do .env
// 2. Conecta ao MongoDB
// 3. Configura o servidor HTTP com Fiber
// 4. Registra todas as rotas da API
// 5. Inicia o consumer RabbitMQ em goroutine
// 6. Configura graceful shutdown
// 7. Inicia o servidor HTTP
// bootstrapAdminUser cria o usuario admin no banco se nao existir.
// NUNCA reseta a senha em reinicios — isso so acontece na criacao inicial.
func bootstrapAdminUser() {
	existing, _ := repository.GetUserByEmail("admin@email.com")
	if existing != nil {
		log.Println("Admin user already exists, skipping bootstrap")
		return
	}

	// Usuario nao existe: cria com senha do ambiente ou default
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "changeme"
		log.Println("Warning: No ADMIN_PASSWORD env var, using default password. Set ADMIN_PASSWORD in Render dashboard.")
	}

	admin := &models.User{
		Email:    "admin@email.com",
		Name:     "Payment Admin",
		Password: password,
		Role:     models.RoleAdmin,
		Active:   true,
	}
	if err := repository.CreateUser(admin); err != nil {
		log.Printf("Warning: Failed to create admin user: %v", err)
		return
	}
	log.Println("Admin user created: admin@email.com (set ADMIN_PASSWORD env var to change)")
}

func main() {
	// 1. Carrega variaveis de ambiente do arquivo .env
	config.Load()

	// 2. Conecta ao MongoDB e cria indices necessarios
	repository.Connect()

	// 2.1. Bootstrap admin user se nao existir
	bootstrapAdminUser()

	// 3. Cria instancia do Fiber com nome do servico
	app := fiber.New(fiber.Config{
		AppName: "FuuPayment Service",
	})

	// 4. Configura middleware global
	app.Use(logger.New()) // Log de cada requisicao no console
	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://fuudelivery-web.onrender.com,https://fuudelivery-admin-lv7f.onrender.com,https://fuudelivery-payment-panel.onrender.com",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// 5. Rota de health check para monitoramento
	// Usado pelo Render para verificar se o servico esta saudavel
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "payment"})
	})

	// 6. Inicializa handlers (cada um responsavel por um dominio)
	ph := handlers.NewPaymentHandler()     // Pagamentos
	ch := handlers.NewChargebackHandler()  // Estornos
	wh := handlers.NewWalletHandler()      // Carteiras
	uh := handlers.NewUserHandler()        // Autenticacao
	ah := handlers.NewApprovalHandler()    // Aprovacoes

	// 7. Configura grupo de rotas da API com autenticacao JWT
	api := app.Group("/api")

	// === Rotas de Autenticacao (publicas) ===
	auth := api.Group("/auth")
	auth.Post("/login", uh.Login)

	// === Rotas Protegidas (requerem token JWT) ===
	payments := api.Group("/payments", middleware.AuthRequired())
	payments.Get("/", ph.ListPayments)
	payments.Get("/stats", ph.GetStats)
	payments.Get("/:id", ph.GetPayment)
	payments.Post("/", ph.CreatePayment)
	payments.Post("/:id/approve", ph.ApprovePayment)
	payments.Post("/:id/reject", ph.RejectPayment)

	approvals := api.Group("/approvals", middleware.AuthRequired())
	approvals.Get("/queue", ah.GetQueue)
	approvals.Get("/auto-approved", ah.GetAutoApproved)
	approvals.Get("/rules", ah.GetRules)
	approvals.Put("/rules", ah.UpdateRules)

	chargebacks := api.Group("/chargebacks", middleware.AuthRequired())
	chargebacks.Get("/", ch.ListChargebacks)
	chargebacks.Get("/stats", ch.GetStats)
	chargebacks.Get("/:id", ch.GetChargeback)
	chargebacks.Post("/", ch.CreateChargeback)
	chargebacks.Post("/:id/approve", ch.ApproveChargeback)
	chargebacks.Post("/:id/reject", ch.RejectChargeback)
	chargebacks.Post("/:id/evidence", ch.AddEvidence)
	chargebacks.Get("/:id/evidence", ch.GetEvidences)

	wallets := api.Group("/wallets", middleware.AuthRequired())
	wallets.Get("/:user_id", wh.GetBalance)
	wallets.Get("/:user_id/transactions", wh.GetTransactions)
	wallets.Post("/:user_id/credit", wh.Credit)
	wallets.Post("/:user_id/debit", wh.Debit)
	wallets.Get("/:user_id/get-or-create", wh.GetOrCreate)

	// 8. Inicia consumer RabbitMQ em goroutine separada
	// O consumer escuta a fila de pagamentos e credit automaticamente
	// na carteira quando um pagamento e aprovado
	go func() {
		consumer, err := consumers.NewPaymentConsumer()
		if err != nil {
			log.Printf("Warning: Failed to start payment consumer: %v", err)
		} else {
			defer consumer.Stop()
			if err := consumer.Start(); err != nil {
				log.Printf("Warning: Failed to start consumer: %v", err)
			}
		}
	}()

	// 9. Configura graceful shutdown
	// Captura sinais SIGINT (Ctrl+C) e SIGTERM (kill do Docker)
	// para encerrar o servico graciosamente
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down...")
		app.Shutdown() // Aguarda requisicoes em andamento finalizarem
	}()

	// 10. Inicia servidor HTTP na porta configurada
	// Porta padrao: 8084 (configuravel via variavel PORT)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	log.Printf("Payment Service starting on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
