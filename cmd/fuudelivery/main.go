package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	// Models (database initialization)
	"github.com/carloshomar/vercardapio/auth_api/app/models"
	ordersModels "github.com/carloshomar/vercardapio/orders_api/app/models"
	deliveryModels "github.com/carloshomar/vercardapio/delivery_api/app/models"
	paymentModels "github.com/carloshomar/vercardapio/payment_api/app/models"
	chatModels "github.com/carloshomar/vercardapio/chat_api/app/models"

	// Handlers
	authHandlers "github.com/carloshomar/vercardapio/auth_api/app/handlers"
	ordersHandlers "github.com/carloshomar/vercardapio/orders_api/app/handlers"
	deliveryHandlers "github.com/carloshomar/vercardapio/delivery_api/app/handlers"
	paymentHandlers "github.com/carloshomar/vercardapio/payment_api/app/handlers"
	chatHandlers "github.com/carloshomar/vercardapio/chat_api/app/handlers"

	// Middleware
	"github.com/carloshomar/vercardapio/auth_api/app/middlewares"

	// Queue
	"github.com/carloshomar/fuudelivery/pkg/queue"
	"github.com/carloshomar/fuudelivery/pkg/health"
)

// WebSocket client management (shared across services)
var wsClients = make(map[int64]*websocket.Conn)
var wsClientsMu sync.Mutex

func sendMessageToClient(clientID int64, message []byte) error {
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	if client, ok := wsClients[clientID]; ok {
		return client.WriteMessage(websocket.TextMessage, message)
	}
	log.Printf("[WS] Message for client %d: %s", clientID, string(message))
	return nil
}

func protectedRoute(c *fiber.Ctx) error {
	_, err := middlewares.ValidateJWT(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	return c.Next()
}

func adminRequired(c *fiber.Ctx) error {
	_, err := middlewares.ValidateJWT(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	role, err := middlewares.GetUserRoleFromToken(c)
	if err != nil || role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}
	return c.Next()
}

func setupWebSocketRoutes(app *fiber.App) {
	// Orders WebSocket
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
		clientIDStr := c.Params("id")
		clientID, _ := strconv.ParseInt(clientIDStr, 10, 64)

		wsClientsMu.Lock()
		wsClients[clientID] = c
		wsClientsMu.Unlock()

		defer func() {
			wsClientsMu.Lock()
			delete(wsClients, clientID)
			wsClientsMu.Unlock()
		}()

		var (
			mt  int
			msg []byte
			err error
		)
		for {
			if mt, msg, err = c.ReadMessage(); err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", msg)
			if err = c.WriteMessage(mt, msg); err != nil {
				log.Println("write:", err)
				break
			}
		}
	}))

	// Chat WebSocket
	app.Get("/ws/chat/:orderId/:userId/:userType", websocket.New(chatHandlers.HandleChatWebSocket))
}

func setupAuthRoutes(app *fiber.App) {
	app.Post("/users/register", authHandlers.CreateUser)
	app.Post("/users/login", authHandlers.Login)
	app.Post("/admin/bootstrap", authHandlers.BootstrapAdmin)
	app.Get("/users", adminRequired, authHandlers.ListAllUsers)
	app.Get("/users/:id", protectedRoute, authHandlers.GetUser)
	app.Put("/users/:id/password", protectedRoute, authHandlers.ChangePassword)

	app.Get("/establishments", authHandlers.ListEstablishments)
	app.Get("/establishments/:id", authHandlers.GetEstablishments)
	app.Put("/establishments/status/handler/:id", protectedRoute, authHandlers.HandlerEstablishmentStatus)
	app.Put("/establishments/:id", protectedRoute, authHandlers.UpdateEstablishment)
	app.Get("/establishments/:id/users", protectedRoute, authHandlers.GetUserByEstablishment)

	app.Get("/establishments/:id/hours", authHandlers.GetBusinessHours)
	app.Post("/establishments/hours", protectedRoute, authHandlers.UpsertBusinessHours)
	app.Post("/establishments/hours/bulk", protectedRoute, authHandlers.BulkUpdateBusinessHours)
	app.Get("/establishments/:id/is-open", authHandlers.CheckEstablishmentOpen)

	app.Post("/delivery-man/login", authHandlers.LoginDeliveryMan)
	app.Post("/delivery-man/register", authHandlers.CreateDeliveryMan)
	app.Get("/delivery-man", adminRequired, authHandlers.ListAllDeliveryMen)
}

func setupOrdersRoutes(app *fiber.App) {
	app.Get("/ping", ordersHandlers.Ping)
	app.Get("/products/all/:establishmentId", ordersHandlers.GetByEstablishmentIdWithRelations)
	app.Get("/products/:establishmentId", ordersHandlers.GetByEstablishmentId)

	app.Post("/products/create", protectedRoute, ordersHandlers.CreateProduct)
	app.Delete("/products/delete/:id", protectedRoute, ordersHandlers.DeleteProduct)
	app.Post("/products/multi-create", protectedRoute, ordersHandlers.CreateMultProducts)
	app.Put("/products/update/:id", protectedRoute, ordersHandlers.UpdateProduct)

	app.Post("/categories/create", protectedRoute, ordersHandlers.CreateCategories)
	app.Get("/categories/:establishmentId", ordersHandlers.GetCategories)
	app.Post("/categories/product", protectedRoute, ordersHandlers.CreateProductCategorie)
	app.Delete("/categories/:id", protectedRoute, ordersHandlers.DeleteCategory)
	app.Put("/categories/:id", protectedRoute, ordersHandlers.UpdateCategory)
	app.Get("/categories/product/:establishmentId", ordersHandlers.GetCategoriesWithProducts)

	app.Post("/additional", protectedRoute, ordersHandlers.CreateAdditional)
	app.Get("/additional/:id", ordersHandlers.ListAdditional)
	app.Put("/additional/:id", protectedRoute, ordersHandlers.UpdateAdditional)
	app.Delete("/additional/:id", protectedRoute, ordersHandlers.DeleteAdditional)
	app.Post("/additional/product", protectedRoute, ordersHandlers.CreateProductToAdditional)

	app.Post("/delivery", protectedRoute, ordersHandlers.InsertDelivery)
	app.Post("/delivery/calculate-delivery-value", protectedRoute, ordersHandlers.CalculateDeliveryValue)
	app.Get("/delivery/value/:establishmentId", ordersHandlers.GetDeliveryByEstablishmentID)

	app.Post("/orders", protectedRoute, func(c *fiber.Ctx) error {
		return ordersHandlers.CreateOrder(c, sendMessageToClient)
	})
	app.Put("/orders/status", protectedRoute, func(c *fiber.Ctx) error {
		return ordersHandlers.UpdateOrderStatus(c, sendMessageToClient)
	})
	app.Get("/orders/all", adminRequired, ordersHandlers.ListAllOrders)
	app.Get("/orders/repeat/:orderId", ordersHandlers.RepeatOrder)
	app.Get("/orders/list-phone/:phone", ordersHandlers.ListOrdersByPhone)
	app.Get("/orders/:establishmentId", ordersHandlers.ListOrdersByEstablishmentID)
	app.Get("/orders/:establishmentId/:phoneNumber", ordersHandlers.ListOrdersByEstablishmentIDAndPhone)

	app.Post("/coupons", protectedRoute, ordersHandlers.CreateCoupon)
	app.Post("/coupons/validate", ordersHandlers.ValidateCoupon)
	app.Post("/coupons/apply", protectedRoute, ordersHandlers.ApplyCoupon)
	app.Get("/coupons", ordersHandlers.ListCoupons)
	app.Get("/coupons/:id", ordersHandlers.GetCoupon)
	app.Delete("/coupons/:id", protectedRoute, ordersHandlers.DeleteCoupon)
	app.Post("/coupons/referral", protectedRoute, ordersHandlers.GenerateReferralCoupon)
	app.Post("/coupons/calculate", ordersHandlers.CalculateDiscount)

	app.Get("/qrcode/:establishmentId", ordersHandlers.GenerateTableQRCode)
	app.Post("/orders/schedule", protectedRoute, ordersHandlers.ScheduleOrder)
	app.Post("/notifications/register", protectedRoute, ordersHandlers.RegisterPushToken)

	app.Post("/loyalty/earn", protectedRoute, ordersHandlers.EarnPoints)
	app.Post("/loyalty/redeem", protectedRoute, ordersHandlers.RedeemPoints)
	app.Get("/loyalty/balance/:phone", ordersHandlers.GetLoyaltyBalance)
	app.Get("/loyalty/history/:phone", ordersHandlers.GetLoyaltyHistory)
	app.Get("/loyalty/calculate", ordersHandlers.CalculateLoyaltyDiscount)

	app.Post("/reviews", protectedRoute, ordersHandlers.CreateReview)
	app.Get("/reviews/establishment/:id", ordersHandlers.GetEstablishmentReviews)
	app.Get("/reviews/product/:id", ordersHandlers.GetProductReviews)
	app.Put("/reviews/respond/:id", protectedRoute, ordersHandlers.RespondToReview)
	app.Get("/reviews/user/:phone", ordersHandlers.GetUserReviews)
	app.Get("/reviews/rating/:establishmentId", ordersHandlers.GetEstablishmentRating)

	app.Post("/orders/pickup-code/generate", protectedRoute, ordersHandlers.GeneratePickupCode)
	app.Post("/orders/pickup-code/validate", protectedRoute, ordersHandlers.ValidatePickupCode)
	app.Get("/orders/pickup-code/:id", protectedRoute, ordersHandlers.GetPickupCode)
}

func setupDeliveryRoutes(app *fiber.App) {
	app.Get("/solicitation-orders", deliveryHandlers.GetApprovedSolicitations)
	app.Put("/solicitation-orders/hand-shake", protectedRoute, deliveryHandlers.HandShakeDeliveryman)
	app.Get("/deliveryman/has-active/:id", deliveryHandlers.GetOrdersByDeliverymanID)
	app.Post("/deliveryman/status", protectedRoute, func(c *fiber.Ctx) error {
		return deliveryHandlers.UpdateOrderStatusByDeliverymanID(c, sendMessageToClient)
	})
	app.Get("/deliveryman/extrato/:id", deliveryHandlers.GetExtrato)
}

func setupPaymentRoutes(app *fiber.App) {
	app.Get("/payments/all", adminRequired, paymentHandlers.ListAllPayments)
	app.Post("/payments/pix/generate", protectedRoute, paymentHandlers.GeneratePIX)
	app.Post("/payments/card/tokenize", protectedRoute, paymentHandlers.TokenizeCard)
	app.Post("/payments/card/charge", protectedRoute, paymentHandlers.ChargeCard)
	app.Post("/payments/process", protectedRoute, paymentHandlers.ProcessPayment)
	app.Post("/payments/split", protectedRoute, paymentHandlers.ProcessSplit)
	app.Post("/payments/webhook", paymentHandlers.HandlePaymentWebhook)
	app.Post("/payments/mercadopago/webhook", paymentHandlers.MercadoPagoWebhook)
	app.Get("/wallet/balance/:user_id", protectedRoute, paymentHandlers.GetBalance)
	app.Post("/wallet/topup", protectedRoute, paymentHandlers.TopUp)
	app.Post("/wallet/deduct", protectedRoute, paymentHandlers.DeductFromWallet)
}

func setupChatRoutes(app *fiber.App) {
	app.Get("/chat/messages/:orderId", protectedRoute, chatHandlers.GetMessages)
	app.Post("/chat/message", protectedRoute, chatHandlers.SendMessage)
	app.Put("/chat/read/:orderId", protectedRoute, chatHandlers.MarkAsRead)
}

func main() {
	godotenv.Load()

	// Initialize databases
	models.ConnectDatabase()
	ordersModels.ConnectPostgresDatabase()
	ordersModels.ConnectMongoDatabase()
	deliveryModels.ConnectMongoDatabase()
	paymentModels.ConnectMongoDatabase()
	chatModels.ConnectMongoDatabase()

	// Initialize message queue
	queue.Init()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		Prefork:       false,
		CaseSensitive: true,
		StrictRouting: false,
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://fuudelivery-web.onrender.com,https://fuudelivery-admin-lv7f.onrender.com,http://localhost:3000,http://localhost:3001",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "fuudelivery",
			"version": "1.0.0",
			"checks": fiber.Map{
				"postgres": health.DatabaseCheck(models.DB),
				"mongodb":  health.MongoCheck(ordersModels.MongoClient),
			},
			"time": time.Now().UTC(),
		})
	})

	// Mount all routes
	setupWebSocketRoutes(app)
	setupAuthRoutes(app)
	setupOrdersRoutes(app)
	setupDeliveryRoutes(app)
	setupPaymentRoutes(app)
	setupChatRoutes(app)

	// Start queue listeners in background
	go startQueueListeners()

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
		app.ShutdownWithTimeout(10 * time.Second)
	}()

	log.Printf("FUUDELIVERY server starting on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func startQueueListeners() {
	queue.Subscribe("order_updates", func(msg []byte) {
		log.Printf("[QUEUE] Order update: %s", string(msg))
	})

	queue.Subscribe("delivery_updates", func(msg []byte) {
		log.Printf("[QUEUE] Delivery update: %s", string(msg))
	})

	queue.Subscribe("payment_updates", func(msg []byte) {
		log.Printf("[QUEUE] Payment update: %s", string(msg))
	})
}
