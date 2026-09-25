package main

// WebSocket: tickets de conexão, conexões por cliente, autorização de
// acesso a pedido e as rotas /ws/*.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	// Models (database initialization)
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	deliveryModels "github.com/carloshomar/fuudelivery/delivery_api/app/models"
	ordersModels "github.com/carloshomar/fuudelivery/orders_api/app/models"

	// Handlers

	chatHandlers "github.com/carloshomar/fuudelivery/chat_api/app/handlers"

	// Middleware
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	// Dispatch engine
	// Batch expiry
	// Queue + Health + Upload + Metrics + Search
)

// parseWSToken valida e decodifica um JWT para uso em WebSocket.
// Valida SigningMethod HMAC (HS256) para evitar ataques de algorithm confusion,
// consistente com o middleware HTTP (auth_api/app/middlewares/jwt.go).
func parseWSToken(tokenStr string) (jwt.MapClaims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT secret not configured")
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// === WS TICKET STORE ===
// Em vez de passar o JWT na query string dos WebSockets (que vaza em logs de
// proxy), o cliente primeiro chama POST /auth/ws-ticket com o JWT no header
// Authorization, recebe um ticket de 60s, e conecta ao WS com ?ticket=<ticket>.
type wsTicket struct {
	Claims    jwt.MapClaims
	ExpiresAt time.Time
}

var (
	wsTickets   = make(map[string]*wsTicket)
	wsTicketsMu sync.Mutex
)

// generateWSTicket cria um ticket aleatório de 32 bytes (hex = 64 chars).
func generateWSTicket() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// IssueWSTicket valida um JWT via Authorization header e retorna um ticket
// de 60s. Chamado pelo endpoint POST /auth/ws-ticket.
func IssueWSTicket(jwtToken string) (string, error) {
	claims, err := parseWSToken(jwtToken)
	if err != nil {
		return "", fmt.Errorf("invalid token")
	}
	ticket, err := generateWSTicket()
	if err != nil {
		return "", fmt.Errorf("failed to generate ticket")
	}
	wsTicketsMu.Lock()
	wsTickets[ticket] = &wsTicket{
		Claims:    claims,
		ExpiresAt: time.Now().Add(60 * time.Second),
	}
	wsTicketsMu.Unlock()
	return ticket, nil
}

// resolveWSTicket consome um ticket (uso único) e retorna os claims.
// Suporta também ?token=<jwt> para backwards compat durante rolling deploy
// (deprecated: será removido em versão futura).
func resolveWSTicket(queryToken, queryTicket string) (jwt.MapClaims, error) {
	// Caminho novo: ticket
	if queryTicket != "" {
		wsTicketsMu.Lock()
		t, ok := wsTickets[queryTicket]
		if ok {
			delete(wsTickets, queryTicket) // uso único
		}
		wsTicketsMu.Unlock()
		if !ok || time.Now().After(t.ExpiresAt) {
			return nil, fmt.Errorf("invalid or expired ticket")
		}
		return t.Claims, nil
	}
	// JWT in query string removed for security (leaks in proxy logs).
	// All clients must use POST /auth/ws-ticket to get a short-lived ticket.
	return nil, fmt.Errorf("authentication required: use POST /auth/ws-ticket first")
}

// wsUserIDMatches decide se o claim "id" do token autoriza a URL cujo id
// é urlID. A regra existe porque o cliente WebSocket passou de user.sub
// (string) para user.id (numérico) quando a sessão migrou para
// GET /auth/session: tokens de sessão antigas ainda circulam com id em
// STRING, e o dono legítimo não pode tomar "User ID mismatch" por causa do
// tipo. Um id desalinhado com a URL, porém, é IDOR e é barrado em qualquer
// formato.
func wsUserIDMatches(claims jwt.MapClaims, urlID string) bool {
	urlNum, err := strconv.ParseInt(urlID, 10, 64)
	if err != nil {
		return false
	}
	var claimID int64
	switch v := claims["id"].(type) {
	case float64:
		claimID = int64(v)
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return false
		}
		claimID = n
	default:
		return false
	}
	return claimID == urlNum
}

// cleanupWSTickets remove tickets expirados periodicamente (1/min).
func cleanupWSTickets() {
	go func() {
		for {
			time.Sleep(time.Minute)
			cutoff := time.Now()
			wsTicketsMu.Lock()
			for k, t := range wsTickets {
				if t.ExpiresAt.Before(cutoff) {
					delete(wsTickets, k)
				}
			}
			wsTicketsMu.Unlock()
		}
	}()
}

// WebSocket client management (shared across services)

// safeConn serializa escritas em UMA conexão WebSocket.
// Por quê: gorilla/fasthttp ws não permite WriteMessage concorrente no mesmo
// conn ("concurrent write to websocket connection"). Sem o wrapper, o push da
// fila (sendMessageToClient) e o echo/read-loop do próprio handler escreviam
// no mesmo conn de goroutines diferentes — janela rara de panic.
// wsMessageWriter é o subconjunto de *websocket.Conn usado pelo safeConn —
// existe para os testes injetarem um writer falso sob -race.
type wsMessageWriter interface {
	WriteMessage(messageType int, data []byte) error
}

type safeConn struct {
	conn wsMessageWriter
	mu   sync.Mutex
}

func (s *safeConn) WriteMessage(messageType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(messageType, data)
}

// Close fecha a conexão subjacente (usado no slot-steal do /ws/:id).
// A asserção mantém o campo como interface para os testes usarem fakes.
func (s *safeConn) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.conn.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}

var wsClients = make(map[int64]*safeConn)
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

// claimsParticipateInSolicitation decide se o token participa do pedido já
// despachado. Clientes, usuários de loja e entregadores vêm de tabelas
// diferentes, com sequências de id independentes — então o "id" do token só
// pode ser comparado com o id do MESMO tipo de conta:
//   - entregador (token sem role, emitido por GenerateJWTDeliveryMan) ↔ delivery_man_id;
//   - cliente (token com role e sem establishment_id) ↔ user_id;
//   - loja: pelo claim establishment_id, nunca pelo id do usuário.
//
// Antes, id do token era comparado com user_id, establishment_id e
// delivery_man_id ao mesmo tempo: o cliente de id 5 via o chat e a posição
// do entregador de todo pedido da loja 5 ou do entregador 5.
func claimsParticipateInSolicitation(claims jwt.MapClaims, s deliveryModels.DeliverySolicitation) bool {
	tokenUserID, _ := claims["id"].(float64)
	uid := int64(tokenUserID)
	role, _ := claims["role"].(string)
	phone, _ := claims["phone"].(string)
	estID := int64(0)
	if v, ok := claims["establishment_id"].(float64); ok {
		estID = int64(v)
	}

	if estID != 0 && estID == s.EstablishmentID {
		return true
	}
	if uid != 0 {
		isDeliveryMan := role == "" || role == "deliveryman"
		if isDeliveryMan && s.DeliveryManID != 0 && uid == s.DeliveryManID {
			return true
		}
		if !isDeliveryMan && estID == 0 && uid == s.UserID {
			return true
		}
	}
	// Telefone vale só para cliente: o entregador também tem phone no token.
	if role != "" && phone != "" && phone == s.UserPhone {
		return true
	}
	return false
}

// chatUserTypeFromClaims decide o sender_type do chat pelo token, com o
// mesmo mapeamento do push: cliente → "client", usuário de loja (tem
// establishment_id) → "restaurant", token sem role (GenerateJWTDeliveryMan)
// → "deliveryman", admin → "admin".
func chatUserTypeFromClaims(claims jwt.MapClaims) string {
	role, _ := claims["role"].(string)
	switch role {
	case "client", "admin":
		return role
	case "":
		return "deliveryman"
	}
	if v, ok := claims["establishment_id"].(float64); ok && v > 0 {
		return "restaurant"
	}
	return role
}

// wsCanAccessOrder autoriza um token JWT a acessar dados em tempo real de um
// pedido (WebSocket de localização da entrega e de chat).
//
// Defesa contra IDOR: autenticar não basta — o usuário só pode acompanhar
// pedidos dos quais PARTICIPA. Os participantes são resolvidos de
// delivery_solicitations (corte 3); se o pedido ainda não foi despachado,
// cai para order_documents (corte 5) validando estabelecimento e telefone do
// cliente. Admin sempre passa, com log de auditoria em toda negação.
func wsCanAccessOrder(claims jwt.MapClaims, orderID string) bool {
	if role, _ := claims["role"].(string); role == "admin" {
		return true
	}

	tokenUserID, _ := claims["id"].(float64)
	uid := int64(tokenUserID)
	phone, _ := claims["phone"].(string)
	estID := int64(0)
	if v, ok := claims["establishment_id"].(float64); ok {
		estID = int64(v)
	}

	var s deliveryModels.DeliverySolicitation
	err := deliveryModels.DB.
		Select("user_id", "user_phone", "establishment_id", "delivery_man_id").
		Where("order_id = ?", orderID).
		First(&s).Error
	if err == nil {
		if claimsParticipateInSolicitation(claims, s) {
			return true
		}
	} else if err == gorm.ErrRecordNotFound {
		// Pedido ainda sem solicitação de entrega (não despachado): valida
		// estabelecimento e cliente direto do pedido.
		var doc ordersModels.OrderDocument
		if err2 := ordersModels.DB.
			Select("establishment_id", "user_phone").
			Where("legacy_id = ?", orderID).
			First(&doc).Error; err2 == nil {
			if estID != 0 && estID == doc.EstablishmentID {
				return true
			}
			if phone != "" && phone == doc.UserPhone {
				return true
			}
		}
	} else {
		log.Printf("[WS-AUTH] erro consultando participação do pedido %s: %v", orderID, err)
	}

	log.Printf("[WS-AUTH] acesso negado: user %d (est %d, role/phone verificados) tentou acessar pedido %s", uid, estID, orderID)
	return false
}

// senderNameForChat resolve o nome de exibição do remetente direto do banco
// (client → user → entregador). O nome enviado pelo cliente nunca é usado.
func senderNameForChat(userID int64) string {
	var client models.Client
	if err := models.DB.Select("name").First(&client, userID).Error; err == nil && client.Name != "" {
		return client.Name
	}
	var user models.User
	if err := models.DB.Select("name").First(&user, userID).Error; err == nil && user.Name != "" {
		return user.Name
	}
	var dm models.DeliveryMan
	if err := models.DB.Select("name").First(&dm, userID).Error; err == nil {
		return dm.Name
	}
	return ""
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
		claims, err := resolveWSTicket(c.Query("token"), c.Query("ticket"))
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid or expired ticket"}}`))
			return
		}
		tokenUserID, _ := claims["id"].(float64)

		clientIDStr := c.Params("id")
		clientID, err := strconv.ParseInt(clientIDStr, 10, 64)
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid client ID"}}`))
			return
		}
		_ = tokenUserID // regra de comparação extraída em wsUserIDMatches (testável)
		if !wsUserIDMatches(claims, clientIDStr) {
			role, _ := claims["role"].(string)
			if role != "admin" {
				c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"User ID mismatch"}}`))
				return
			}
		}

		sc := &safeConn{conn: c}
		wsClientsMu.Lock()
		// Slot-steal: se outra conexão (aba antiga) já ocupa o slot, fecha-a
		// antes de sobrescrever — senão a referência antiga vira lixo vivo
		// (conn aberta que ninguém mais remove do mapa).
		if old, ok := wsClients[clientID]; ok && old != sc {
			_ = old.Close()
		}
		wsClients[clientID] = sc
		wsClientsMu.Unlock()

		defer func() {
			wsClientsMu.Lock()
			// Só remove se ainda somos nós (evita apagar a conn de uma
			// aba nova que assumiu o slot enquanto esta morria).
			if cur, ok := wsClients[clientID]; ok && cur == sc {
				delete(wsClients, clientID)
			}
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
			log.Printf("recv: %s", msg)
			if err2 = sc.WriteMessage(mt, msg); err2 != nil {
				log.Println("write:", err2)
				break
			}
		}
	}))

	// Chat WebSocket with JWT auth
	app.Get("/ws/chat/:orderId/:userId/:userType", websocket.New(func(c *websocket.Conn) {
		claims, err := resolveWSTicket(c.Query("token"), c.Query("ticket"))
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid or expired ticket"}}`))
			return
		}
		tokenUserID, _ := claims["id"].(float64)
		_ = tokenUserID // regra de comparação extraída em wsUserIDMatches (testável)
		if !wsUserIDMatches(claims, c.Params("userId")) {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"User ID mismatch"}}`))
			return
		}
		// IDOR: autenticar não basta — o usuário precisa participar do pedido.
		if !wsCanAccessOrder(claims, c.Params("orderId")) {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Forbidden"}}`))
			return
		}
		// O tipo do remetente vem do token, nunca do :userType da URL.
		chatHandlers.HandleChatWebSocketAs(c, chatUserTypeFromClaims(claims))
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
	deliveryLocsListeners := make(map[string][]*safeConn)
	var deliveryLocsListenersMu sync.Mutex

	app.Get("/ws/delivery/:orderId", websocket.New(func(c *websocket.Conn) {
		claims, err := resolveWSTicket(c.Query("token"), c.Query("ticket"))
		if err != nil {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Invalid or expired ticket"}}`))
			return
		}

		orderID := c.Params("orderId")
		if orderID == "" {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"orderId required"}}`))
			return
		}

		// IDOR: autenticar não basta — só participantes do pedido (cliente,
		// estabelecimento, entregador atribuído, admin) podem ver a localização.
		if !wsCanAccessOrder(claims, orderID) {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Forbidden"}}`))
			return
		}

		sc := &safeConn{conn: c}
		c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"connected","payload":{"orderId":"%s"}}`, orderID)))

		deliveryLocsListenersMu.Lock()
		deliveryLocsListeners[orderID] = append(deliveryLocsListeners[orderID], sc)
		deliveryLocsListenersMu.Unlock()

		defer func() {
			deliveryLocsListenersMu.Lock()
			listeners := deliveryLocsListeners[orderID]
			for i, l := range listeners {
				if l == sc {
					deliveryLocsListeners[orderID] = append(listeners[:i], listeners[i+1:]...)
					break
				}
			}
			deliveryLocsListenersMu.Unlock()
		}()

		deliveryLocsMu.RLock()
		if loc, ok := deliveryLocations[orderID]; ok {
			data, _ := json.Marshal(map[string]interface{}{"type": "location", "payload": loc})
			sc.WriteMessage(websocket.TextMessage, data)
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

		// Corte 3: leitura do entregador atribuído direto do Postgres.
		var solicitation deliveryModels.DeliverySolicitation
		err = deliveryModels.DB.Where("order_id = ?", req.OrderID).First(&solicitation).Error
		if err != nil || solicitation.DeliveryManID != tokenUserID {
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
		// Poda: mantém no máximo 500 entradas — o mapa crescia indefinidamente
		// (o único "cleanup" era o restart do processo).
		if len(deliveryLocations) > 500 {
			for k, v := range deliveryLocations {
				if time.Since(time.UnixMilli(v.Timestamp)) > 2*time.Hour {
					delete(deliveryLocations, k)
					if len(deliveryLocations) <= 400 {
						break
					}
				}
			}
		}
		deliveryLocsMu.Unlock()

		data, _ := json.Marshal(map[string]interface{}{"type": "location", "payload": loc})
		deliveryLocsListenersMu.Lock()
		listeners := append([]*safeConn(nil), deliveryLocsListeners[req.OrderID]...)
		deliveryLocsListenersMu.Unlock()
		// Escreve FORA do lock: I/O sob mutex serializava todos os pushes
		// de posição entre si.
		for _, listener := range listeners {
			_ = listener.WriteMessage(websocket.TextMessage, data)
		}

		return c.JSON(fiber.Map{"message": "Location updated", "order_id": req.OrderID})
	})
}
