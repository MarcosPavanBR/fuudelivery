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
	"github.com/carloshomar/vercardapio/payment/repository"
)

func main() {
	config.Load()
	repository.Connect()

	app := fiber.New(fiber.Config{
		AppName: "FuuPayment Service",
	})

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "payment"})
	})

	ph := handlers.NewPaymentHandler()
	ch := handlers.NewChargebackHandler()
	wh := handlers.NewWalletHandler()
	uh := handlers.NewUserHandler()
	ah := handlers.NewApprovalHandler()

	api := app.Group("/api")
	_ = middleware.AuthRequired()

	auth := api.Group("/auth")
	auth.Post("/login", uh.Login)

	payments := api.Group("/payments")
	payments.Get("/", ph.ListPayments)
	payments.Get("/stats", ph.GetStats)
	payments.Get("/:id", ph.GetPayment)
	payments.Post("/", ph.CreatePayment)
	payments.Post("/:id/approve", ph.ApprovePayment)
	payments.Post("/:id/reject", ph.RejectPayment)

	approvals := api.Group("/approvals")
	approvals.Get("/queue", ah.GetQueue)
	approvals.Get("/auto-approved", ah.GetAutoApproved)
	approvals.Get("/rules", ah.GetRules)
	approvals.Put("/rules", ah.UpdateRules)

	chargebacks := api.Group("/chargebacks")
	chargebacks.Get("/", ch.ListChargebacks)
	chargebacks.Get("/stats", ch.GetStats)
	chargebacks.Get("/:id", ch.GetChargeback)
	chargebacks.Post("/", ch.CreateChargeback)
	chargebacks.Post("/:id/approve", ch.ApproveChargeback)
	chargebacks.Post("/:id/reject", ch.RejectChargeback)
	chargebacks.Post("/:id/evidence", ch.AddEvidence)
	chargebacks.Get("/:id/evidence", ch.GetEvidences)

	wallets := api.Group("/wallets")
	wallets.Get("/:user_id", wh.GetBalance)
	wallets.Get("/:user_id/transactions", wh.GetTransactions)
	wallets.Post("/:user_id/credit", wh.Credit)
	wallets.Post("/:user_id/debit", wh.Debit)
	wallets.Get("/:user_id/get-or-create", wh.GetOrCreate)

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

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down...")
		app.Shutdown()
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	log.Printf("Payment Service starting on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
