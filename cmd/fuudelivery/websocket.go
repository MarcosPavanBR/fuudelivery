package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carloshomar/fuudelivery/chat_api/app/handlers"
	deliveryModels "github.com/carloshomar/fuudelivery/delivery_api/app/models"
	"github.com/carloshomar/fuudelivery/pkg/queue"
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
)

const wsTokenPrefix = "ws:token:"
const wsTokenTTL = 5 * time.Minute

func generateWSToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := os.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", buf), nil
}

func CreateWSToken(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing Authorization header"})
	}

	tokenString := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenString = authHeader[7:]
	}

	parsedToken, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || !parsedToken.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
	}

	userIDFloat, ok := claims["id"].(float64)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user id in token"})
	}
	userID := int64(userIDFloat)

	token, err := generateWSToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate ws token"})
	}

	redisClient := queue.GetClient()
	if redisClient == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Redis unavailable"})
	}

	key := wsTokenPrefix + token
	if err := redisClient.Set(c.Context(), key, fmt.Sprintf("%d", userID), wsTokenTTL).Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to store ws token"})
	}

	return c.JSON(fiber.Map{"ws_token": token, "expires_in": int(wsTokenTTL.Seconds())})
}

// WebSocket client management (shared across services)
var wsClients = make(map[int64]*websocket.Conn)
var wsClientsMu sync.Mutex

func sendMessageToClient(userID int64, message []byte) error {
	wsClientsMu.Lock()
	conn, ok := wsClients[userID]
	wsClientsMu.Unlock()

	if !ok {
		return fmt.Errorf("no ws client for user %d", userID)
	}

	if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
		log.Println("ws write error:", err)
		wsClientsMu.Lock()
		delete(wsClients, userID)
		wsClientsMu.Unlock()
		return err
	}
	return nil
}

func setupWebSocketRoutes(app *fiber.App) {
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws-token", protectedRoute, CreateWSToken)

	app.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
		wsToken := c.Query("ws_token")
		if wsToken == "" {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Missing ws_token"}}`))
			return
		}
		redisClient := queue.GetClient()
		if redisClient == nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Redis unavailable"}}`))
			return
		}
		userIDStr, err := redisClient.Get(c.Context(), wsTokenPrefix+wsToken).Result()
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid or expired ws_token"}}`))
			return
		}
		userID, _ := strconv.ParseInt(userIDStr, 10, 64)

		clientIDStr := c.Params("id")
		clientID, err := strconv.ParseInt(clientIDStr, 10, 64)
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid client ID"}}`))
			return
		}
		if userID != clientID {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"User ID mismatch"}}`))
			return
		}

		wsClientsMu.Lock()
		wsClients[clientID] = c
		wsClientsMu.Unlock()

		defer func() {
			wsClientsMu.Lock()
			delete(wsClients, clientID)
			wsClientsMu.Unlock()
		}()

		var (
			mt   int
			msg  []byte
			err2 error
		)
		for {
			if mt, msg, err2 = c.ReadMessage(); err2 != nil {
				log.Println("read:", err2)
				break
			}
			log.Printf("recv: message_type=%d len=%d", mt, len(msg))
			if err2 = c.WriteMessage(mt, msg); err2 != nil {
				log.Println("write:", err2)
				break
			}
		}
	}))

	app.Get("/ws/chat/:orderId/:userId/:userType", websocket.New(func(c *websocket.Conn) {
		wsToken := c.Query("ws_token")
		if wsToken == "" {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Missing ws_token"}}`))
			return
		}
		redisClient := queue.GetClient()
		if redisClient == nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Redis unavailable"}}`))
			return
		}
		userIDStr, err := redisClient.Get(c.Context(), wsTokenPrefix+wsToken).Result()
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid or expired ws_token"}}`))
			return
		}
		tokenUserID, _ := strconv.ParseInt(userIDStr, 10, 64)
		urlUserID, _ := strconv.ParseInt(c.Params("userId"), 10, 64)
		if tokenUserID != urlUserID {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"User ID mismatch"}}`))
			return
		}
		handlers.HandleChatWebSocket(c)
	}))

	// --- FUU PULSE: Real-time delivery location ---
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
		wsToken := c.Query("ws_token")
		if wsToken == "" {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Missing ws_token"}}`))
			return
		}
		redisClient := queue.GetClient()
		if redisClient == nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Redis unavailable"}}`))
			return
		}
		userIDStr, err := redisClient.Get(c.Context(), wsTokenPrefix+wsToken).Result()
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid or expired ws_token"}}`))
			return
		}
		userID, _ := strconv.ParseInt(userIDStr, 10, 64)

		orderID := c.Params("orderId")
		if orderID == "" {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"orderId required"}}`))
			return
		}

		c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"connected","payload":{"orderId":"%s","user_id":%d}}`, orderID, userID)))

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

		deliveryLocsMu.RLock()
		if loc, ok := deliveryLocations[orderID]; ok {
			data, _ := json.Marshal(map[string]interface{}{"type": "location", "payload": loc})
			c.WriteMessage(websocket.TextMessage, data)
		}
		deliveryLocsMu.RUnlock()

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

		data, _ := json.Marshal(map[string]interface{}{"type": "location", "payload": loc})
		deliveryLocsListenersMu.Lock()
		active := deliveryLocsListeners[req.OrderID][:0]
		for _, listener := range deliveryLocsListeners[req.OrderID] {
			if err := listener.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Println("ws write error:", err)
				continue
			}
			active = append(active, listener)
		}
		deliveryLocsListeners[req.OrderID] = active
		deliveryLocsListenersMu.Unlock()

		return c.JSON(fiber.Map{"message": "Location updated", "order_id": req.OrderID})
	})
}

// isLocalDevOrigin libera origens de desenvolvimento local
// (localhost / 127.0.0.1 / ::1 em qualquer porta). O Fiber não suporta
// wildcard de porta no AllowOrigins, então o check é programático;
// quando retorna true o middleware ecoa o origin na resposta
// (compatível com AllowCredentials: true).
func isLocalDevOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
