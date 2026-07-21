package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"

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

	// Chat WebSocket with JWT auth
	app.Get("/ws/chat/:orderId/:userId/:userType", websocket.New(func(c *websocket.Conn) {
		token := c.Query("token")
		if token == "" {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Authentication required"}}`))
			return
		}
		parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !parsedToken.Valid {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid token"}}`))
			return
		}
		claims, ok := parsedToken.Claims.(jwt.MapClaims)
		if !ok {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid token claims"}}`))
			return
		}
		tokenUserID, _ := claims["id"].(float64)
		urlUserID, _ := strconv.ParseInt(c.Params("userId"), 10, 64)
		if int64(tokenUserID) != urlUserID {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"User ID mismatch"}}`))
			return
		}
		chatHandlers.HandleChatWebSocket(c)
	}))

	// --- FUU PULSE: Real-time delivery location ---
	// Store latest location per order (in-memory, ephemeral)
	type DeliveryLocation struct {
		Lat       float64 `json:"lat"`
		Lng       float64 `json:"lng"`
		OrderID   string  `json:"order_id"`
		Timestamp int64   `json:"timestamp"`
	}

	var deliveryLocsMu sync.RWMutex
	deliveryLocations := make(map[string]*DeliveryLocation)
	deliveryLocsListeners := make(map[string][]*websocket.Conn)
	var deliveryLocsListenersMu sync.Mutex

	app.Get("/ws/delivery/:orderId", websocket.New(func(c *websocket.Conn) {
		orderID := c.Params("orderId")
		if orderID == "" {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"orderId required"}}`))
			return
		}

		c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"connected","payload":{"orderId":"%s"}}`, orderID)))

		deliveryLocsListenersMu.Lock()
		deliveryLocsListeners[orderID] = append(deliveryLocsListeners[orderID], c)
		deliveryLocsListenersMu.Unlock()

		defer func() {
			deliveryLocsListenersMu.Lock()
			listeners := deliveryLocsListeners[orderID]
			for i, l := range listeners {
				if l == c {
					deliveryLocsListeners[orderID] = append(listeners[:i], listeners[i+1:]...)
					break
				}
			}
			deliveryLocsListenersMu.Unlock()
		}()

		// Send current location immediately if exists
		deliveryLocsMu.RLock()
		if loc, ok := deliveryLocations[orderID]; ok {
			data, _ := json.Marshal(map[string]interface{}{"type": "location", "payload": loc})
			c.WriteMessage(websocket.TextMessage, data)
		}
		deliveryLocsMu.RUnlock()

		// Keep connection alive; ignore incoming messages
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}))

	// POST /delivery/location — deliveryman sends their GPS coordinates
	app.Post("/delivery/location", protectedRoute, func(c *fiber.Ctx) error {
		tokenUserID, err := middlewares.GetUserIDFromToken(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}

		var req struct {
			Lat     float64 `json:"lat"`
			Lng     float64 `json:"lng"`
			OrderID string  `json:"order_id"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		if req.OrderID == "" || (req.Lat == 0 && req.Lng == 0) {
			return c.Status(400).JSON(fiber.Map{"error": "order_id, lat, and lng are required"})
		}

		// Verify deliveryman is assigned to this order
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var solicitation struct {
			DeliveryMan struct {
				Id int64 `bson:"id"`
			} `bson:"deliveryman"`
		}
		err = deliveryModels.MongoDabase.Collection("solicitations").FindOne(ctx, bson.M{"order_id": req.OrderID}).Decode(&solicitation)
		if err != nil || solicitation.DeliveryMan.Id != tokenUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Not the assigned deliveryman for this order"})
		}

		loc := &DeliveryLocation{
			Lat:       req.Lat,
			Lng:       req.Lng,
			OrderID:   req.OrderID,
			Timestamp: time.Now().UnixMilli(),
		}

		deliveryLocsMu.Lock()
		deliveryLocations[req.OrderID] = loc
		deliveryLocsMu.Unlock()

		// Broadcast to all listeners for this order
		data, _ := json.Marshal(map[string]interface{}{"type": "location", "payload": loc})
		deliveryLocsListenersMu.Lock()
		for _, listener := range deliveryLocsListeners[req.OrderID] {
			listener.WriteMessage(websocket.TextMessage, data)
		}
		deliveryLocsListenersMu.Unlock()

		return c.JSON(fiber.Map{"message": "Location updated", "order_id": req.OrderID})
	})
}

func setupAuthRoutes(app *fiber.App) {
	app.Post("/users/register", authHandlers.CreateUser)
	app.Post("/users/login", authHandlers.Login)
	app.Post("/admin/bootstrap", authHandlers.BootstrapAdmin)
	app.Get("/users", adminRequired, authHandlers.ListAllUsers)
	app.Get("/users/:id", protectedRoute, authHandlers.GetUser)
	app.Delete("/users/:id", protectedRoute, authHandlers.DeleteUser)
	app.Put("/users/:id/password", protectedRoute, authHandlers.ChangePassword)

	app.Get("/establishments", authHandlers.ListEstablishments)
	app.Get("/establishments/:id", authHandlers.GetEstablishments)
	app.Post("/establishments", adminRequired, authHandlers.CreateEstablishment)
	app.Put("/establishments/status/handler/:id", protectedRoute, authHandlers.HandlerEstablishmentStatus)
	app.Put("/establishments/:id", protectedRoute, authHandlers.UpdateEstablishment)
	app.Delete("/establishments/:id", adminRequired, authHandlers.DeleteEstablishment)
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
	app.Post("/delivery/calculate-route", protectedRoute, ordersHandlers.CalculateRoute)
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
	// Mercado Pago removed — AbacatePay is the official gateway
	app.Get("/wallet/balance/:user_id", protectedRoute, paymentHandlers.GetBalance)
	app.Post("/wallet/topup", protectedRoute, paymentHandlers.TopUp)
	app.Post("/wallet/deduct", protectedRoute, paymentHandlers.DeductFromWallet)
}

func setupChatRoutes(app *fiber.App) {
	app.Get("/chat/messages/:orderId", protectedRoute, func(c *fiber.Ctx) error {
		tokenUserID, err := middlewares.GetUserIDFromToken(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		orderID := c.Params("orderId")
		if orderID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "orderId is required"})
		}

		// 1. Check if user is the customer (order owner)
		var order ordersModels.Order
		if qErr := ordersModels.DB.First(&order, orderID).Error; qErr == nil {
			if uint(tokenUserID) == order.UserID {
				return chatHandlers.GetMessages(c)
			}
			// 2. Check if user is the restaurant owner of the order's establishment
			var user models.User
			if uErr := models.DB.First(&user, tokenUserID).Error; uErr == nil {
				if user.EstablishmentID != 0 && user.EstablishmentID == order.EstablishmentID {
					return chatHandlers.GetMessages(c)
				}
			}
		}

		// 3. Check if user is the assigned deliveryman for this order
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var solicitation struct {
			DeliveryMan struct {
				Id int64 `bson:"id"`
			} `bson:"deliveryman"`
		}
		_ = deliveryModels.MongoDabase.Collection("solicitations").FindOne(ctx, bson.M{"order_id": orderID}).Decode(&solicitation)
		if solicitation.DeliveryMan.Id != 0 && solicitation.DeliveryMan.Id == tokenUserID {
			return chatHandlers.GetMessages(c)
		}

		// 4. Admin role bypass (for support/audit)
		role, _ := middlewares.GetUserRoleFromToken(c)
		if role == "admin" {
			return chatHandlers.GetMessages(c)
		}

		log.Printf("[CHAT IDOR] GetMessages denied: user=%d order=%s", tokenUserID, orderID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not a participant of this order"})
	})
	app.Post("/chat/message", protectedRoute, chatHandlers.SendMessage)
	app.Put("/chat/read/:orderId/:userId", protectedRoute, func(c *fiber.Ctx) error {
		tokenUserID, err := middlewares.GetUserIDFromToken(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		urlUserIDStr := c.Params("userId")
		var urlUserID int64
		if _, scanErr := fmt.Sscanf(urlUserIDStr, "%d", &urlUserID); scanErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid userId"})
		}
		if tokenUserID != urlUserID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot mark messages as read for another user"})
		}
		return chatHandlers.MarkAsRead(c)
	})
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

	// Root health check (for Render when healthCheckPath not configured)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "fuudelivery"})
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
