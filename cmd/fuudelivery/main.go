package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	"golang.org/x/time/rate"
	"gorm.io/gorm"

	// Models (database initialization)
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	chatModels "github.com/carloshomar/fuudelivery/chat_api/app/models"
	deliveryModels "github.com/carloshomar/fuudelivery/delivery_api/app/models"
	ordersModels "github.com/carloshomar/fuudelivery/orders_api/app/models"
	paymentModels "github.com/carloshomar/fuudelivery/payment_api/app/models"

	// Handlers
	authHandlers "github.com/carloshomar/fuudelivery/auth_api/app/handlers"
	chatHandlers "github.com/carloshomar/fuudelivery/chat_api/app/handlers"
	deliveryHandlers "github.com/carloshomar/fuudelivery/delivery_api/app/handlers"
	ordersHandlers "github.com/carloshomar/fuudelivery/orders_api/app/handlers"
	paymentHandlers "github.com/carloshomar/fuudelivery/payment_api/app/handlers"

	// Middleware
	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"

	// Dispatch engine
	dispatchServices "github.com/carloshomar/fuudelivery/delivery_api/app/services"
	// Batch expiry
	orderServices "github.com/carloshomar/fuudelivery/orders_api/app/services"

	// Queue + Health + Upload + Metrics + Search
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/carloshomar/fuudelivery/pkg/gateway/abacatepay"
	"github.com/carloshomar/fuudelivery/pkg/gateway/asaas"
	"github.com/carloshomar/fuudelivery/pkg/gateway/mercadopago"
	"github.com/carloshomar/fuudelivery/pkg/gateway/pagarme"
	"github.com/carloshomar/fuudelivery/pkg/health"
	"github.com/carloshomar/fuudelivery/pkg/metrics"
	"github.com/carloshomar/fuudelivery/pkg/queue"
	"github.com/carloshomar/fuudelivery/pkg/search"
	"github.com/carloshomar/fuudelivery/pkg/upload"
)

// Rate limiter por IP usando golang.org/x/time/rate (token bucket).
// Entries stale sao limpas periodicamente para evitar memory leak.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	ipLimiters   = make(map[string]*ipLimiter)
	ipLimitersMu sync.Mutex
)

func getIPLimiter(ip string, rps rate.Limit, burst int) *rate.Limiter {
	ipLimitersMu.Lock()
	defer ipLimitersMu.Unlock()

	li, exists := ipLimiters[ip]
	if !exists {
		li = &ipLimiter{limiter: rate.NewLimiter(rps, burst)}
		ipLimiters[ip] = li
	}
	li.lastSeen = time.Now()
	return li.limiter
}

func startRateLimitCleanup() {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			cutoff := time.Now().Add(-10 * time.Minute)
			ipLimitersMu.Lock()
			for ip, li := range ipLimiters {
				if li.lastSeen.Before(cutoff) {
					delete(ipLimiters, ip)
				}
			}
			ipLimitersMu.Unlock()
		}
	}()
}

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

// === DISPATCH ENGINE (global instances) ===
var (
	courierStore    *dispatchServices.CourierStore
	matchingEngine  *dispatchServices.MatchingEngine
	dispatchHandler *deliveryHandlers.DispatchHandler
	calibrationJob  *dispatchServices.AutoCalibrationJob
	splitDecayJob   *dispatchServices.SplitDecayJob
)

func initDispatchEngine(db *gorm.DB) {
	courierStore = dispatchServices.NewCourierStore()

	// Zone resolver: consulta PostgreSQL via GORM
	zoneResolver := &zoneDBResolver{DB: db}

	matchingEngine = dispatchServices.NewMatchingEngine(courierStore, zoneResolver)

	// Callback: quando um pedido e matchado, publica no canal de delivery_updates
	matchingEngine.OnMatch = func(orderID string, courierID int64) {
		data, _ := json.Marshal(map[string]interface{}{
			"type":       "order_matched",
			"order_id":   orderID,
			"courier_id": courierID,
			"matched_at": time.Now().UTC(),
		})
		if err := queue.Publish("delivery_updates", data); err != nil {
			log.Printf("[QUEUE] ERRO ao publicar order_matched (%s): %v", orderID, err)
		}
	}

	// Callback: fallback comunitario ativado
	matchingEngine.OnFallback = func(orderID string, zoneName string) {
		log.Printf("[FALLBACK] Order %s needs community fallback in zone %q", orderID, zoneName)
		data, _ := json.Marshal(map[string]interface{}{
			"type":      "community_fallback",
			"order_id":  orderID,
			"zone_name": zoneName,
			"time":      time.Now().UTC(),
		})
		if err := queue.Publish("delivery_updates", data); err != nil {
			log.Printf("[QUEUE] ERRO ao publicar community_fallback (%s): %v", orderID, err)
		}
	}

	// Inicia retry loop a cada 30s
	matchingEngine.StartRetryLoop(30 * time.Second)

	// Inicia cleanup de couriers stale a cada 5min
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			courierStore.CleanupStale(300) // 5 minutos
		}
	}()

	// Cria handler HTTP (corte 3: dispatch não usa mais Mongo)
	dispatchHandler = deliveryHandlers.NewDispatchHandler(courierStore, matchingEngine)

	// === Job de calibracao automatica ===
	calConfig := dispatchServices.DefaultCalibrationConfig()
	calConfig.Interval = 24 * time.Hour
	calibrationJob = dispatchServices.NewAutoCalibrationJob(calConfig, matchingEngine, zoneResolver)

	// Callback: quando uma zona e calibrada, persiste o novo raio no banco
	calibrationJob.SetOnCalibrate(func(result dispatchServices.CalibrationResult) {
		if result.OldRadiusKm == result.NewRadiusKm {
			return
		}
		if db == nil {
			return
		}
		if err := db.Model(&models.Zone{}).Where("id = ?", result.ZoneID).Update("radius_km", result.NewRadiusKm).Error; err != nil {
			log.Printf("[CALIBRATION] Failed to update radius for zone %d: %v", result.ZoneID, err)
		}
	})

	// Funcao de busca de zonas para o job
	fetchZones := func() []dispatchServices.ZoneMetadata {
		if db == nil {
			return nil
		}
		var zones []models.Zone
		if err := db.Where("is_active = ?", true).Find(&zones).Error; err != nil {
			log.Printf("[CALIBRATION] Failed to fetch zones: %v", err)
			return nil
		}
		result := make([]dispatchServices.ZoneMetadata, 0, len(zones))
		for _, z := range zones {
			result = append(result, dispatchServices.ZoneMetadata{
				ID:                    z.ID,
				Name:                  z.Name,
				MinRadiusKm:           z.MinRadiusKm,
				RadiusKm:              z.RadiusKm,
				MaxRadiusKm:           z.MaxRadiusKm,
				PeakRadiusMultiplier:  z.PeakRadiusMultiplier,
				PeakHourStart:         z.PeakHourStart,
				PeakHourEnd:           z.PeakHourEnd,
				CitySize:              z.CitySize,
				DensityCouriersPerKm2: z.DensityCouriersPerKm2,
				MinDeliveryFee:        z.MinDeliveryFee,
				SurgeMultiplier:       z.SurgeMultiplier,
				MinCouriersThreshold:  z.MinCouriersThreshold,
				AllowBatching:         z.AllowBatching,
			})
		}
		return result
	}

	// Inicia o job de calibracao
	calibrationJob.Start(fetchZones)

	// Inicia recalculo periodico de densidade (a cada 15min)
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			zones := fetchZones()
			if len(zones) == 0 {
				continue
			}
			// Converte ZoneMetadata para ZoneInfo
			zoneInfos := make([]dispatchServices.ZoneInfo, len(zones))
			for i, z := range zones {
				// Calcula centroide real a partir dos estabelecimentos da zona
				var centerLat, centerLng float64
				if db != nil {
					var ests []models.Establishment
					db.Select("lat, long").Where("zone_id = ? AND lat != 0", z.ID).Find(&ests)
					if len(ests) > 0 {
						sumLat, sumLng := 0.0, 0.0
						for _, e := range ests {
							sumLat += e.Lat
							sumLng += e.Long
						}
						centerLat = sumLat / float64(len(ests))
						centerLng = sumLng / float64(len(ests))
					}
				}
				zoneInfos[i] = dispatchServices.ZoneInfo{
					ID:        z.ID,
					CenterLat: centerLat,
					CenterLng: centerLng,
					RadiusKm:  z.RadiusKm,
				}
			}
			courierStore.RecalculateAllDensities(zoneInfos)
		}
	}()

	// === Job de decaimento de split ===
	splitDecayConfig := dispatchServices.DefaultSplitDecayConfig()
	splitDecayJob = dispatchServices.NewSplitDecayJob(splitDecayConfig, &splitMetricsProvider{DB: db})

	// Callback: persiste o novo split no banco
	splitDecayJob.SetOnDecay(func(result dispatchServices.SplitDecayResult) {
		if !result.Applied {
			return
		}
		if db == nil {
			return
		}
		now := time.Now()
		updates := map[string]interface{}{
			"split_current_platform_pct":      result.NewPlatformPct,
			"split_current_establishment_pct": result.NewEstablishmentPct,
			"split_last_adjusted_at":          now,
		}
		if err := db.Model(&models.Zone{}).Where("id = ?", result.ZoneID).Updates(updates).Error; err != nil {
			log.Printf("[SPLIT_DECAY] Failed to update split for zone %d: %v", result.ZoneID, err)
		} else {
			log.Printf("[SPLIT_DECAY] Zone %d split updated: %.1f%% -> %.1f%%",
				result.ZoneID, result.OldPlatformPct, result.NewPlatformPct)
		}
	})

	// Funcao de busca de dados de split das zonas
	fetchSplitZones := func() []dispatchServices.ZoneSplitData {
		if db == nil {
			return nil
		}
		var zones []models.Zone
		if err := db.Where("is_active = ?", true).Find(&zones).Error; err != nil {
			log.Printf("[SPLIT_DECAY] Failed to fetch zones: %v", err)
			return nil
		}
		result := make([]dispatchServices.ZoneSplitData, 0, len(zones))
		for _, z := range zones {
			result = append(result, dispatchServices.ZoneSplitData{
				ID:                           z.ID,
				Name:                         z.Name,
				SplitCurrentPlatformPct:      z.SplitCurrentPlatformPct,
				SplitCurrentEstablishmentPct: z.SplitCurrentEstablishmentPct,
				SplitInitialPlatformPct:      z.SplitInitialPlatformPct,
				SplitInitialEstablishmentPct: z.SplitInitialEstablishmentPct,
				SplitTargetPlatformPct:       z.SplitTargetPlatformPct,
				SplitTargetEstablishmentPct:  z.SplitTargetEstablishmentPct,
				SplitStepMonths:              z.SplitStepMonths,
				SplitStepPlatformPct:         z.SplitStepPlatformPct,
				SplitStepEstablishmentPct:    z.SplitStepEstablishmentPct,
				SplitMinMonthlyOrders:        z.SplitMinMonthlyOrders,
				SplitMinActiveCouriers:       z.SplitMinActiveCouriers,
				SplitLastAdjustedAt:          z.SplitLastAdjustedAt,
				CreatedAt:                    z.CreatedAt,
			})
		}
		return result
	}

	// Inicia o job de decaimento
	splitDecayJob.Start(fetchSplitZones)

	log.Println("[DISPATCH] Engine initialized: courier store + matching engine + calibration job + split decay + retry loop")
}

// splitMetricsProvider implementa dispatchServices.ZoneMetricsProvider usando GORM.
type splitMetricsProvider struct {
	DB *gorm.DB
}

func (s *splitMetricsProvider) GetMonthlyOrders(zoneID uint) int {
	// Conta pedidos dos ÚLTIMOS 30 DIAS para estabelecimentos vinculados
	// a esta zona. Usa order_documents (tabela ativa desde o corte 5) que
	// possui created_at; a tabela legada "orders" não tem coluna temporal.
	if s.DB == nil {
		return 0
	}
	var count int64
	err := s.DB.Raw(
		"SELECT COUNT(*) FROM order_documents "+
			"JOIN establishments ON establishments.id = order_documents.establishment_id "+
			"WHERE establishments.zone_id = ? "+
			"AND order_documents.created_at >= NOW() - INTERVAL '30 days'", zoneID,
	).Scan(&count).Error
	if err != nil {
		// Coluna establishment_id pode nao existir ainda (migration pendente)
		log.Printf("[SPLIT_DECAY] GetMonthlyOrders fallback: %v", err)
		return 0
	}
	return int(count)
}

func (s *splitMetricsProvider) GetActiveCouriers(zoneID uint) int {
	// Conta entregadores com status 'available' ou 'busy' vinculados a esta zona
	if s.DB == nil {
		return 0
	}
	var count int64
	err := s.DB.Model(&models.DeliveryMan{}).
		Where("zone_id = ? AND status IN ('available', 'busy')", zoneID).
		Count(&count).Error
	if err != nil {
		log.Printf("[SPLIT_DECAY] GetActiveCouriers fallback: %v", err)
		return 0
	}
	return int(count)
}

// zoneDBResolver implementa dispatchServices.ZoneResolver usando GORM.
type zoneDBResolver struct {
	DB *gorm.DB
}

func (z *zoneDBResolver) ResolveByLatLng(lat, lng float64) (uint, string, float64, error) {
	if z.DB == nil {
		return 0, "Default", 10.0, nil
	}

	var zones []models.Zone
	if err := z.DB.Where("is_active = ?", true).Find(&zones).Error; err != nil || len(zones) == 0 {
		return 0, "Default", 10.0, nil
	}

	// Zona única ativa = deployment simples; mantém comportamento anterior.
	if len(zones) == 1 {
		z0 := zones[0]
		return z0.ID, z0.Name, z0.RadiusKm, nil
	}

	// Multi-zona: casa pelo prefixo geohash (match mais longo vence).
	point := encodeGeohash(lat, lng, 8)
	bestIdx, bestLen := -1, 0
	for i, zn := range zones {
		p := strings.ToLower(strings.TrimSpace(zn.GeohashPrefix))
		if p == "" || len(p) > len(point) {
			continue
		}
		if strings.HasPrefix(point, p) && len(p) > bestLen {
			bestIdx, bestLen = i, len(p)
		}
	}
	if bestIdx >= 0 {
		zb := zones[bestIdx]
		return zb.ID, zb.Name, zb.RadiusKm, nil
	}

	// Nenhuma zona casou geometricamente — cai na primeira ativa (comportamento
	// antigo), mas agora VISÍVEL: sem este log o desvio passaria despercebido.
	log.Printf("[ZONE] WARN: lat=%.5f,lng=%.5f não casou com geohash de zona alguma (%d ativas); usando %q",
		lat, lng, len(zones), zones[0].Name)
	zf := zones[0]
	return zf.ID, zf.Name, zf.RadiusKm, nil
}

// encodeGeohash codifica (lat,lng) na string geohash padrão (base32) com a
// precisão pedida. Implementação mínima sem dependência externa — suficiente
// para casar prefixes de zona (precisão 5 ≈ 4.9km, 6 ≈ 1.2km, 8 ≈ 38m).
func encodeGeohash(lat, lng float64, precision int) string {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"
	latMin, latMax := -90.0, 90.0
	lngMin, lngMax := -180.0, 180.0

	var hash []byte
	even := true
	bit := 0
	chIdx := 0

	for len(hash) < precision {
		if even {
			mid := (lngMin + lngMax) / 2
			if lng >= mid {
				chIdx = chIdx<<1 | 1
				lngMin = mid
			} else {
				chIdx <<= 1
				lngMax = mid
			}
		} else {
			mid := (latMin + latMax) / 2
			if lat >= mid {
				chIdx = chIdx<<1 | 1
				latMin = mid
			} else {
				chIdx <<= 1
				latMax = mid
			}
		}
		even = !even
		bit++
		if bit == 5 {
			hash = append(hash, base32[chIdx])
			bit = 0
			chIdx = 0
		}
	}
	return string(hash)
}

func (z *zoneDBResolver) GetDeliveryFee(zoneID uint, distanceKm float64) float64 {
	if z.DB == nil {
		return 5.0
	}
	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return 5.0
	}
	fee := zone.MinDeliveryFee
	if distanceKm > 3.0 {
		fee += (distanceKm - 3.0) * 1.5
	}
	return fee
}

func (z *zoneDBResolver) GetSurgeMultiplier(zoneID uint) float64 {
	if z.DB == nil {
		return 1.0
	}
	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return 1.0
	}
	return zone.SurgeMultiplier
}

func (z *zoneDBResolver) GetMinCouriersThreshold(zoneID uint) int {
	if z.DB == nil {
		return 3
	}
	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return 3
	}
	return zone.MinCouriersThreshold
}

func (z *zoneDBResolver) AllowsBatching(zoneID uint) bool {
	if z.DB == nil {
		return true
	}
	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return true
	}
	return zone.AllowBatching
}

func (z *zoneDBResolver) GetZoneMetadata(zoneID uint) *dispatchServices.ZoneMetadata {
	meta := &dispatchServices.ZoneMetadata{
		MinRadiusKm:          2.0,
		RadiusKm:             5.0,
		MaxRadiusKm:          15.0,
		PeakRadiusMultiplier: 0.7,
		PeakHourStart:        "11:00",
		PeakHourEnd:          "14:00",
		MinDeliveryFee:       5.0,
		SurgeMultiplier:      1.0,
		MinCouriersThreshold: 3,
		AllowBatching:        true,
	}

	if z.DB == nil || zoneID == 0 {
		return meta
	}

	var zone models.Zone
	if err := z.DB.First(&zone, zoneID).Error; err != nil {
		return meta
	}

	meta.ID = zone.ID
	meta.Name = zone.Name
	meta.MinRadiusKm = zone.MinRadiusKm
	meta.RadiusKm = zone.RadiusKm
	meta.MaxRadiusKm = zone.MaxRadiusKm
	meta.PeakRadiusMultiplier = zone.PeakRadiusMultiplier
	meta.PeakHourStart = zone.PeakHourStart
	meta.PeakHourEnd = zone.PeakHourEnd
	meta.CitySize = zone.CitySize
	meta.DensityCouriersPerKm2 = zone.DensityCouriersPerKm2
	meta.MinDeliveryFee = zone.MinDeliveryFee
	meta.SurgeMultiplier = zone.SurgeMultiplier
	meta.MinCouriersThreshold = zone.MinCouriersThreshold
	meta.AllowBatching = zone.AllowBatching

	return meta
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
		if uid != 0 && (uid == s.UserID || uid == s.EstablishmentID || (s.DeliveryManID != 0 && uid == s.DeliveryManID)) {
			return true
		}
		if estID != 0 && estID == s.EstablishmentID {
			return true
		}
		if phone != "" && phone == s.UserPhone {
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
		if int64(tokenUserID) != clientID {
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
		urlUserID, _ := strconv.ParseInt(c.Params("userId"), 10, 64)
		if int64(tokenUserID) != urlUserID {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"User ID mismatch"}}`))
			return
		}
		// IDOR: autenticar não basta — o usuário precisa participar do pedido.
		if !wsCanAccessOrder(claims, c.Params("orderId")) {
			c.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","payload":{"message":"Forbidden"}}`))
			return
		}
		chatHandlers.HandleChatWebSocket(c)
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

// rateLimitByIdentifierMiddleware cria um middleware que limita requests
// por identificador de conta (user_type:identifier), não por IP.
//
// Protege contra brute-force distribuído: um atacante com múltiplos IPs
// tentando a mesma conta atinge o teto rápido. O body JSON é lido aqui
// e armazenado em c.Locals("parsedBody") para o handler não precisar
// parseá-lo duas vezes.
//
// maxPerMinute é o limite de requests por minuto por identificador.
func rateLimitByIdentifierMiddleware(maxPerMinute int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Lê o body para extrair user_type + identifier.
		var body struct {
			UserType   string `json:"user_type"`
			Identifier string `json:"identifier"`
		}
		if err := c.BodyParser(&body); err != nil {
			// Body malformado — deixa o handler retornar o erro 400.
			return c.Next()
		}

		userType := strings.TrimSpace(body.UserType)
		identifier := strings.TrimSpace(body.Identifier)
		if userType == "" || identifier == "" {
			// Campos faltando — handler vai retornar 400.
			return c.Next()
		}

		if !rateLimitByIdentifier(userType, identifier, maxPerMinute) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Muitas tentativas para esta conta. Aguarde alguns minutos e tente novamente.",
			})
		}

		return c.Next()
	}
}

func setupAuthRoutes(app *fiber.App) {
	app.Get("/csrf-token", authHandlers.GetCSRFToken)
	app.Post("/users/register", rateLimitMiddleware(5), authHandlers.CreateUser)
	app.Post("/users/login", rateLimitMiddleware(10), authHandlers.Login)
	app.Post("/auth/refresh", rateLimitMiddleware(30), authHandlers.RefreshToken)
	app.Post("/auth/logout", rateLimitMiddleware(10), authHandlers.Logout)
	app.Post("/auth/session", rateLimitMiddleware(10), authHandlers.SessionLogin)
	app.Post("/auth/session/refresh", rateLimitMiddleware(30), authHandlers.SessionRefresh)
	app.Post("/auth/session/logout", rateLimitMiddleware(10), authHandlers.SessionLogout)
	// Ticket de curta duração (60s) para WebSockets: o JWT fica SÓ no header
	// Authorization desta chamada e o WS conecta com ?ticket= — nada de JWT
	// na query string (vazava em logs de proxy). Ver resolveWSTicket.
	app.Post("/auth/ws-ticket", protectedRoute, rateLimitMiddleware(20), func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			auth = auth[7:]
		}
		ticket, tErr := IssueWSTicket(auth)
		if tErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		return c.JSON(fiber.Map{"ticket": ticket, "expires_in": 60})
	})

	// Reset de senha assistido: o suporte gera o código no WebAdmin e informa
	// por telefone/WhatsApp (não há serviço de email; clientes só têm phone).
	// O usuário usa o código na página pública /resetar-senha do WebRestaurant.
	//
	// Rate limit em DUAS camadas:
	//   1. rateLimitMiddleware(N) — por IP (protege contra flooding de uma fonte)
	//   2. rateLimitByIdentifier — por conta (protege contra brute-force
	//      distribuído: múltiplos IPs tentando a mesma conta)
	app.Post("/admin/password-reset/code", adminRequired, rateLimitMiddleware(10), rateLimitByIdentifierMiddleware(3), authHandlers.GenerateAdminResetCode)
	app.Post("/auth/reset-password", rateLimitMiddleware(5), rateLimitByIdentifierMiddleware(10), authHandlers.ResetPassword)
	app.Post("/users", adminRequired, authHandlers.CreateUserAdmin)
	app.Post("/admin/bootstrap", rateLimitMiddleware(3), authHandlers.BootstrapAdmin)
	app.Get("/users", adminRequired, authHandlers.ListAllUsers)
	app.Get("/users/:id", protectedRoute, authHandlers.GetUser)
	app.Put("/users/:id", protectedRoute, authHandlers.UpdateUser)
	app.Delete("/users/:id", protectedRoute, authHandlers.DeleteUser)
	app.Put("/users/:id/password", protectedRoute, authHandlers.ChangePassword)

	app.Get("/establishments", authHandlers.ListEstablishments)
	app.Get("/establishments/ranked", authHandlers.ListEstablishmentsRanked)
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
	app.Put("/establishments/:id/wallet", protectedRoute, authHandlers.UpdateEstablishmentWallet)

	app.Post("/delivery-man/login", rateLimitMiddleware(10), authHandlers.LoginDeliveryMan)
	app.Post("/delivery-man/register", rateLimitMiddleware(5), authHandlers.CreateDeliveryMan)
	app.Get("/delivery-man", adminRequired, authHandlers.ListAllDeliveryMen)
	app.Put("/delivery-man/:id", adminRequired, authHandlers.UpdateDeliveryMan)
	app.Delete("/delivery-man/:id", adminRequired, authHandlers.DeleteDeliveryMan)
	app.Put("/delivery-man/:id/wallet", protectedRoute, authHandlers.UpdateDeliveryManWallet)

	// === Rotas de Cliente (AppComida) ===
	app.Post("/clients/register", rateLimitMiddleware(5), authHandlers.RegisterClient)
	app.Post("/clients/login", rateLimitMiddleware(10), authHandlers.LoginClient)

	// Cadastro público de restaurante (WebRestaurant)
	app.Post("/establishments/register", rateLimitMiddleware(3), authHandlers.RegisterEstablishment)
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
	// Rate limit 30/min na criação de pedidos: é a rota que grava, notifica
	// e dispara dispatch — abuso direto impacta o banco e a fila.
	app.Post("/orders", protectedRoute, rateLimitMiddleware(30), func(c *fiber.Ctx) error {
		return ordersHandlers.CreateOrder(c, sendMessageToClient)
	})
	app.Put("/orders/status", protectedRoute, func(c *fiber.Ctx) error {
		return ordersHandlers.UpdateOrderStatus(c, sendMessageToClient)
	})
	app.Get("/orders/all", adminRequired, ordersHandlers.ListAllOrders)
	app.Get("/orders/repeat/:orderId", protectedRoute, ordersHandlers.RepeatOrder)
	app.Get("/orders/list-phone/:phone", protectedRoute, ordersHandlers.ListOrdersByPhone)
	app.Get("/orders/:establishmentId", protectedRoute, ordersHandlers.ListOrdersByEstablishmentID)
	app.Get("/orders/:establishmentId/:phoneNumber", protectedRoute, ordersHandlers.ListOrdersByEstablishmentIDAndPhone)
	app.Post("/coupons", protectedRoute, rateLimitMiddleware(20), ordersHandlers.CreateCoupon)
	app.Post("/coupons/validate", protectedRoute, rateLimitMiddleware(30), ordersHandlers.ValidateCoupon)
	app.Post("/coupons/apply", protectedRoute, rateLimitMiddleware(30), ordersHandlers.ApplyCoupon)
	app.Get("/coupons", protectedRoute, ordersHandlers.ListCoupons)
	app.Get("/coupons/:id", protectedRoute, ordersHandlers.GetCoupon)
	app.Delete("/coupons/:id", protectedRoute, ordersHandlers.DeleteCoupon)
	app.Post("/coupons/referral", protectedRoute, ordersHandlers.GenerateReferralCoupon)
	app.Post("/coupons/calculate", protectedRoute, ordersHandlers.CalculateDiscount)
	app.Get("/qrcode/:establishmentId", ordersHandlers.GenerateTableQRCode)
	app.Post("/orders/schedule", protectedRoute, ordersHandlers.ScheduleOrder)
	app.Post("/notifications/register", protectedRoute, rateLimitMiddleware(20), ordersHandlers.RegisterPushToken)
	app.Post("/loyalty/earn", protectedRoute, rateLimitMiddleware(20), ordersHandlers.EarnPoints)
	app.Post("/loyalty/redeem", protectedRoute, rateLimitMiddleware(20), ordersHandlers.RedeemPoints)
	app.Get("/loyalty/balance/:phone", protectedRoute, ordersHandlers.GetLoyaltyBalance)
	app.Get("/loyalty/history/:phone", protectedRoute, ordersHandlers.GetLoyaltyHistory)
	app.Get("/loyalty/calculate", protectedRoute, ordersHandlers.CalculateLoyaltyDiscount)
	app.Post("/reviews", protectedRoute, ordersHandlers.CreateReview)
	app.Get("/reviews/establishment/:id", protectedRoute, ordersHandlers.GetEstablishmentReviews)
	app.Get("/reviews/product/:id", protectedRoute, ordersHandlers.GetProductReviews)
	app.Put("/reviews/respond/:id", protectedRoute, ordersHandlers.RespondToReview)
	app.Get("/reviews/user/:phone", protectedRoute, ordersHandlers.GetUserReviews)
	app.Get("/reviews/rating/:establishmentId", protectedRoute, ordersHandlers.GetEstablishmentRating)
	app.Post("/orders/pickup-code/generate", protectedRoute, ordersHandlers.GeneratePickupCode)
	app.Post("/orders/pickup-code/validate", protectedRoute, ordersHandlers.ValidatePickupCode)
	app.Get("/orders/pickup-code/:id", protectedRoute, ordersHandlers.GetPickupCode)

	// === Rotas de Batch (batching de pedidos) ===
	batches := app.Group("/batches", adminRequired)
	batches.Post("/", ordersHandlers.CreateBatch)
	batches.Get("/:id", ordersHandlers.GetBatch)
	batches.Post("/:id/assign", ordersHandlers.AssignBatch)
	batches.Post("/:id/complete", ordersHandlers.CompleteBatch)
	batches.Post("/:id/add-order", ordersHandlers.AddOrderToBatch)
	batches.Get("/zone/:zoneId", ordersHandlers.ListBatchesByZone)
	batches.Post("/:id/force-expire", ordersHandlers.ForceExpireBatch)
}

func setupDeliveryRoutes(app *fiber.App) {
	app.Get("/solicitation-orders", protectedRoute, deliveryHandlers.GetApprovedSolicitations)
	app.Put("/solicitation-orders/hand-shake", protectedRoute, deliveryHandlers.HandShakeDeliveryman)
	app.Get("/deliveryman/has-active/:id", protectedRoute, deliveryHandlers.GetOrdersByDeliverymanID)
	app.Post("/deliveryman/status", protectedRoute, func(c *fiber.Ctx) error {
		return deliveryHandlers.UpdateOrderStatusByDeliverymanID(c, sendMessageToClient)
	})
	app.Get("/deliveryman/extrato/:id", protectedRoute, deliveryHandlers.GetExtrato)
}

func setupZoneRoutes(app *fiber.App) {
	app.Get("/zones", adminRequired, authHandlers.ListZones)
	app.Get("/zones/all", adminRequired, authHandlers.ListAllZones)
	app.Get("/zones/:id", adminRequired, authHandlers.GetZone)
	app.Post("/zones", adminRequired, authHandlers.CreateZone)
	app.Put("/zones/:id", adminRequired, authHandlers.UpdateZone)
	app.Delete("/zones/:id", adminRequired, authHandlers.DeleteZone)
	app.Post("/zones/:id/calibrate", adminRequired, authHandlers.CalibrateZone)
}

func setupDispatchRoutes(app *fiber.App) {
	dispatch := app.Group("/dispatch", protectedRoute)

	// Localizacao do entregador
	// 120/min: chega a cada poucos segundos por courier em movimento, mas
	// precisa de teto contra abuso.
	dispatch.Post("/location", rateLimitMiddleware(120), dispatchHandler.UpdateLocation)
	dispatch.Post("/status", rateLimitMiddleware(30), dispatchHandler.SetCourierStatus)

	// Matching
	dispatch.Post("/trigger", dispatchHandler.TriggerDispatch)
	dispatch.Get("/nearby", dispatchHandler.NearbyCouriers)

	// Dead-letter queue e metricas
	dispatch.Get("/dlq", adminRequired, dispatchHandler.GetDLQ)
	dispatch.Get("/status", adminRequired, dispatchHandler.GetDispatchStatus)
}

// paymentRouterMiddleware injeta o router de pagamento no contexto Fiber.
func paymentRouterMiddleware(router *gateway.Router) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("payment_router", router)
		return c.Next()
	}
}

func setupPaymentRoutes(app *fiber.App, router *gateway.Router) {
	paymentGroup := app.Group("/payments", paymentRouterMiddleware(router))
	walletGroup := app.Group("/wallets", paymentRouterMiddleware(router))
	// Admin — painel Financeiro do WebAdmin
	paymentGroup.Get("/all", adminRequired, paymentHandlers.ListAllPayments)
	paymentGroup.Get("/", adminRequired, paymentHandlers.ListAllPayments)
	paymentGroup.Get("/stats", adminRequired, paymentHandlers.GetPaymentStats)
	walletGroup.Get("/balance/:user_id", protectedRoute, paymentHandlers.GetBalance)
	walletGroup.Get("/establishment/balance", protectedRoute, paymentHandlers.GetEstablishmentWallet)
	walletGroup.Get("/establishment/transactions", protectedRoute, paymentHandlers.GetEstablishmentTransactions)
	walletGroup.Post("/topup", protectedRoute, rateLimitMiddleware(20), paymentHandlers.TopUp)
	walletGroup.Post("/deduct", protectedRoute, rateLimitMiddleware(20), paymentHandlers.DeductFromWallet)
	walletGroup.Post("/establishment/withdraw", protectedRoute, rateLimitMiddleware(20), paymentHandlers.EstablishmentWithdraw)
	paymentGroup.Get("/chargebacks", adminRequired, paymentHandlers.ListChargebacks)
	paymentGroup.Post("/:id/approve", adminRequired, rateLimitMiddleware(20), paymentHandlers.ApprovePayment)
	paymentGroup.Post("/:id/reject", adminRequired, rateLimitMiddleware(20), paymentHandlers.RejectPayment)
	// Rate limit 20/min nos endpoints de dinheiro (proteção contra abuso/custo)
	paymentGroup.Post("/pix/generate", protectedRoute, rateLimitMiddleware(20), paymentHandlers.GeneratePIX)
	// Tokenização de cartão REMOVIDA: o endpoint recebia PAN/CVV crus e
	// devolvia um "token" local sem valor no gateway (risco PCI puro). O
	// cartão volta quando houver tokenização server-side do AbacatePay.
	paymentGroup.Post("/card/charge", protectedRoute, rateLimitMiddleware(20), paymentHandlers.ChargeCard)
	paymentGroup.Post("/process", protectedRoute, rateLimitMiddleware(20), paymentHandlers.ProcessPayment)
	// Split rules definem como o dinheiro é dividido — só admin.
	paymentGroup.Post("/split", adminRequired, rateLimitMiddleware(20), paymentHandlers.ProcessSplit)
	paymentGroup.Post("/webhook", rateLimitMiddleware(100), paymentHandlers.HandlePaymentWebhook)
	// Status da cobrança por pedido (polling do app do cliente pós-PIX).
	paymentGroup.Get("/order/:order_id", protectedRoute, rateLimitMiddleware(30), paymentHandlers.GetPaymentByOrder)
	paymentGroup.Get("/reports/establishment/:id", protectedRoute, paymentHandlers.GetEstablishmentReport)
	paymentGroup.Post("/asaas/wallet/create", protectedRoute, rateLimitMiddleware(20), paymentHandlers.CreateAsaasWallet)
	paymentGroup.Get("/asaas/wallet/:walletId/status", adminRequired, paymentHandlers.GetAsaasWalletStatus)
	paymentGroup.Post("/asaas/payment/split", protectedRoute, rateLimitMiddleware(20), paymentHandlers.CreateAsaasSplitPayment)
}

func setupSponsoredRoutes(app *fiber.App) {
	// Rotas de patrocínio (admin)
	sponsored := app.Group("/sponsored", adminRequired)
	sponsored.Get("/", authHandlers.ListSponsoredListings)
	sponsored.Get("/:id", authHandlers.GetSponsoredListing)
	sponsored.Post("/", authHandlers.CreateSponsoredListing)
	sponsored.Put("/:id", authHandlers.UpdateSponsoredListing)
	sponsored.Post("/:id/cancel", authHandlers.CancelSponsoredListing)
	sponsored.Post("/:id/renew", authHandlers.RenewSponsoredListing)

	// Rotas públicas/de consulta
	sponsored.Get("/by-establishment/:id", protectedRoute, authHandlers.GetEstablishmentSponsorship)
	sponsored.Get("/by-zone/:id", authHandlers.ListSponsoredByZone)

	// === Endpoint público de destaque (não requer auth) ===
	// GET /establishments/featured?zone_id=1&limit=8
	app.Get("/establishments/featured", authHandlers.GetFeaturedEstablishments)
}

func setupSubscriptionRoutes(app *fiber.App) {
	subscriptions := app.Group("/subscriptions")

	// Rotas do usuario (protegidas)
	subscriptions.Get("/me", protectedRoute, authHandlers.GetUserSubscription)
	subscriptions.Post("/", protectedRoute, authHandlers.CreateSubscription)
	subscriptions.Post("/cancel", protectedRoute, authHandlers.CancelSubscription)
	subscriptions.Post("/renew", protectedRoute, authHandlers.RenewSubscription)

	// Rotas de admin
	subscriptions.Get("/", adminRequired, authHandlers.ListSubscriptions)
	subscriptions.Put("/:id", adminRequired, authHandlers.AdminUpdateSubscription)
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

		var order ordersModels.Order
		if qErr := ordersModels.DB.First(&order, orderID).Error; qErr == nil {
			if uint(tokenUserID) == order.UserID {
				return chatHandlers.GetMessages(c)
			}
			var user models.User
			if uErr := models.DB.First(&user, tokenUserID).Error; uErr == nil {
				if user.EstablishmentID != 0 && user.EstablishmentID == order.EstablishmentID {
					return chatHandlers.GetMessages(c)
				}
			}
		}

		// Corte 3: leitura do entregador atribuído direto do Postgres.
		var solicitation deliveryModels.DeliverySolicitation
		_ = deliveryModels.DB.Where("order_id = ?", orderID).First(&solicitation).Error
		if solicitation.DeliveryManID != 0 && solicitation.DeliveryManID == tokenUserID {
			return chatHandlers.GetMessages(c)
		}

		role, _ := middlewares.GetUserRoleFromToken(c)
		if role == "admin" {
			return chatHandlers.GetMessages(c)
		}

		log.Printf("[CHAT IDOR] GetMessages denied: user=%d order=%s", tokenUserID, orderID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not a participant of this order"})
	})
	// IDOR + anti-spoofing: antes qualquer usuário autenticado podia postar
	// como QUALQUER remetente em QUALQUER pedido (o handler confiava 100% no
	// corpo da requisição). Agora: só participantes do pedido e o remetente é
	// sempre quem o token diz que é.
	app.Post("/chat/message", protectedRoute, rateLimitMiddleware(30), func(c *fiber.Ctx) error {
		token, tErr := middlewares.ValidateJWT(c)
		if tErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
		}
		var req struct {
			OrderID string `json:"order_id"`
		}
		if pErr := json.Unmarshal(c.Body(), &req); pErr != nil || req.OrderID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "order_id is required"})
		}
		if !wsCanAccessOrder(claims, req.OrderID) {
			log.Printf("[CHAT IDOR] SendMessage denied: order=%s", req.OrderID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You are not a participant of this order"})
		}
		tokenUserID, _ := middlewares.GetUserIDFromToken(c)
		var body map[string]interface{}
		if jErr := json.Unmarshal(c.Body(), &body); jErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
		}
		// Remetente vem SEMPRE do token — nunca do corpo. Nome resolvido do
		// banco para não exibir o nome forjado pelo cliente.
		body["sender_id"] = tokenUserID
		body["sender_name"] = senderNameForChat(tokenUserID)
		if role, _ := middlewares.GetUserRoleFromToken(c); role != "" {
			body["sender_type"] = role
		}
		fixedBody, _ := json.Marshal(body)
		c.Request().SetBody(fixedBody)
		return chatHandlers.SendMessage(c)
	})
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

// validateRequiredEnv verifica se as variaveis de ambiente essenciais estao presentes.
// Em producao, falha rapidamente (exit) se algo critico estiver faltando:
// com JWT_SECRET vazio os tokens passam a ser assinados/validados com chave
// HMAC vazia, o que permite forjar qualquer identidade — inclusive admin.
func validateRequiredEnv() {
	critical := []string{"JWT_SECRET", "DB_CONNECTION_STRING"}
	missing := false
	for _, key := range critical {
		if os.Getenv(key) == "" {
			log.Printf("[ENV] CRITICAL: %s nao configurado", key)
			missing = true
		}
	}
	if missing {
		if os.Getenv("GO_ENV") == "production" {
			log.Fatalf("[ENV] Encerrando: variaveis criticas ausentes em producao (%v)", critical)
		}
		log.Println("[ENV] Configuracao incompleta — seguindo em modo dev.")
	}

	// Reject known JWT_SECRET placeholders — they allow token forgery
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret != "" {
		nknownPlaceholders := []string{
			"change-this-to-a-random-64-char-string",
			"change-me",
			"secret",
			"your-secret-key",
			"super-secret",
			"123456",
		}
		for _, placeholder := range knownPlaceholders {
			if jwtSecret == placeholder {
				log.Fatalf("[ENV] CRITICAL: JWT_SECRET is a known placeholder (%q). Generate a real secret: openssl rand -hex 32", placeholder)
			}
		}
		if len(jwtSecret) < 32 {
			log.Printf("[ENV] WARNING: JWT_SECRET is only %d chars — recommended minimum is 32", len(jwtSecret))
		}
	}

	// Em producao, valida tambem as de pagamento
	if os.Getenv("GO_ENV") == "production" {
		prodRequired := []string{"ABACATE_PAY_API_KEY", "REDIS_URL"}
		for _, key := range prodRequired {
			if os.Getenv(key) == "" {
				log.Printf("[ENV] WARNING: %s nao configurado — funcionalidade limitada", key)
			}
		}
	}
}

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

	// CSRF protection — valida token em mutações.
	app.Use(func(c *fiber.Ctx) error {
		method := c.Method()
		if method == "GET" || method == "OPTIONS" || method == "HEAD" {
			return c.Next()
		}
		csrfToken := c.Get("X-CSRF-Token")
		if csrfToken == "" {
			csrfToken = c.Cookies("csrf_token")
		}
		if csrfToken == "" || len(csrfToken) < 32 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "CSRF token missing or invalid"})
		}
		return c.Next()
	})

	// Health check — reuses the Redis client from the queue singleton
	// HTTP 503 when Postgres is down OR no payment gateway is configured.
	// Redis/batches degradation returns HTTP 200 with status "degraded".
	app.Get("/health", func(c *fiber.Ctx) error {
		redisClient := queue.GetClient()

		postgresCheck := health.DatabaseCheck(models.DB)
		redisCheck := health.RedisCheck(redisClient)
		redisGeoCheck := health.RedisGeoCheck(redisClient)
		batchesCheck := health.BatchCheck(ordersModels.DB)
		gatewaysCheck := health.GatewayCheck()

		// On cold start (DB not yet initialized), return 200 so Render health
		// checks pass during the DB initialization window (up to 125s).
		if models.DB == nil {
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
	// Sem a env var configurada (ex.: dev local), o endpoint fica aberto.
	app.Get("/metrics", func(c *fiber.Ctx) error {
		if want := os.Getenv("METRICS_TOKEN"); want != "" {
			if c.Get("Authorization") != "Bearer "+want {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
			}
		}
		return metrics.Handler(c)
	})

	// Busca full-text basica (Fase 3 — construcao nova): GET /search?q=...
	// Busca estabelecimentos e produtos no PostgreSQL (ILIKE + scoring).
	app.Get("/search", search.NewHandler(models.DB))

	// Mount all routes
	setupWebSocketRoutes(app)
	setupAuthRoutes(app)
	setupOrdersRoutes(app)
	setupDeliveryRoutes(app)
	setupZoneRoutes(app)
	setupDispatchRoutes(app)
	setupSponsoredRoutes(app)
	setupSubscriptionRoutes(app)
	// Initialize payment gateway router with fallback chain
	pagarmeGW, _ := pagarme.NewGateway()
	asaasGW, _ := asaas.NewGateway()
	abacatepayGW, _ := abacatepay.NewGateway()
	mpGW, _ := mercadopago.NewGateway()
	paymentRouter = gateway.NewRouter(pagarmeGW, asaasGW, abacatepayGW, mpGW)
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
	}()

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

// startQueueListeners consome as filas de status do monolito.
// Usa SubscribeFunc (em vez de Subscribe) para que handlers que retornam erro
// ativem o retry (maxRetries) e a dead-letter queue do pkg/queue — mensagens
// malformadas ou com falha de envio não são perdidas em silêncio.
func startQueueListeners() {
	queue.SubscribeFunc("order_updates", func(msg []byte) error {
		return processStatusUpdate("order_updates", msg)
	})

	queue.SubscribeFunc("delivery_updates", func(msg []byte) error {
		return processStatusUpdate("delivery_updates", msg)
	})

	queue.SubscribeFunc("payment_updates", func(msg []byte) error {
		return processStatusUpdate("payment_updates", msg)
	})
}

// statusEvent representa os campos relevantes das mensagens publicadas nas
// filas order_updates/delivery_updates/payment_updates (ex.: o dispatch engine
// publica order_matched com order_id + courier_id).
type statusEvent struct {
	Type      string `json:"type"`
	OrderID   string `json:"order_id"`
	CourierID int64  `json:"courier_id"`
	ClientID  int64  `json:"client_id"`
	UserID    int64  `json:"user_id"`
}

// resolveStatusRecipient define o destinatário WebSocket da mensagem:
// client_id explícito → user_id → courier_id (somente na fila de delivery).
// Retorna 0 quando a mensagem é informativa (sem destinatário).
func resolveStatusRecipient(queueName string, evt *statusEvent) int64 {
	recipient := evt.ClientID
	if recipient == 0 {
		recipient = evt.UserID
	}
	if recipient == 0 && queueName == "delivery_updates" {
		recipient = evt.CourierID
	}
	return recipient
}

// processStatusUpdate decodifica uma mensagem de status da fila, registra no
// log e notifica o cliente WebSocket do destinatário quando identificável.
// Retorna erro apenas quando a mensagem está malformada ou a notificação
// falha — o pkg/queue então re-tenta (maxRetries) e move a mensagem para a DLQ.
func processStatusUpdate(queueName string, msg []byte) error {
	var evt statusEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		return fmt.Errorf("[QUEUE] %s: mensagem inválida: %w", queueName, err)
	}

	log.Printf("[QUEUE] %s: %s", queueName, string(msg))

	// Mensagens sem destinatário (ex.: community_fallback) são informativas —
	// apenas log, sem erro, para não ir para a DLQ indevidamente.
	recipient := resolveStatusRecipient(queueName, &evt)
	if recipient == 0 {
		return nil
	}

	if err := sendMessageToClient(recipient, msg); err != nil {
		return fmt.Errorf("[QUEUE] %s: falha ao notificar cliente %d: %w", queueName, recipient, err)
	}
	return nil
}

// isLocalDevOrigin libera origens de desenvolvimento local
// (localhost / 127.0.0.1 / ::1 em qualquer porta). O Fiber não suporta
// wildcard de porta no AllowOrigins, então o check é programático;
// quando retorna true o middleware ecoa o origin na resposta
// (compatível com AllowCredentials: true).
func isLocalDevOrigin(origin string) bool {
	// Em PRODUÇÃO devolve sempre false: localhost-any-port com credentials
	// permitia qualquer app local do usuário autenticar contra a API.
	if origin == "" {
		return false
	}
	if os.Getenv("GO_ENV") == "production" {
		return false
	}
	if os.Getenv("GO_ENV") == "" {
		log.Println("[CORS] WARNING: GO_ENV não definido — assumindo development (localhost permitido)")
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
