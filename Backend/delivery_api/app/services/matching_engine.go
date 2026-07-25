package services

import (
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"github.com/carloshomar/vercardapio/delivery_api/app/dto"
)

// UnmatchedOrder representa um pedido que nao encontrou entregador.
type UnmatchedOrder struct {
	OrderID          string  `json:"order_id"`
	EstablishmentLat float64 `json:"establishment_lat"`
	EstablishmentLng float64 `json:"establishment_lng"`
	ZoneID           uint    `json:"zone_id"`
	CreatedAt        int64   `json:"created_at"`
	RetryCount       int     `json:"retry_count"`
	LastAttemptAt    int64   `json:"last_attempt_at"`
}

// DLQStore gerencia a dead-letter queue de pedidos nao casados.
type DLQStore struct {
	mu      sync.Mutex
	orders  []*UnmatchedOrder
	maxSize int
}

// NewDLQStore cria uma nova DLQ com tamanho maximo.
func NewDLQStore(maxSize int) *DLQStore {
	return &DLQStore{
		orders:  make([]*UnmatchedOrder, 0, maxSize),
		maxSize: maxSize,
	}
}

// Push adiciona um pedido nao casado na DLQ.
func (d *DLQStore) Push(order *UnmatchedOrder) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.orders) >= d.maxSize {
		d.orders = d.orders[1:]
	}
	d.orders = append(d.orders, order)
	log.Printf("[DLQ] Order %s added to unmatched queue (total: %d)", order.OrderID, len(d.orders))
}

// PopNext retorna o proximo pedido para retry, ou nil se vazio.
func (d *DLQStore) PopNext() *UnmatchedOrder {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	for i, o := range d.orders {
		if o.RetryCount >= 3 {
			continue
		}
		if now-o.LastAttemptAt < 30000 { // 30s entre retries
			continue
		}
		d.orders = append(d.orders[:i], d.orders[i+1:]...)
		return o
	}
	return nil
}

// Len retorna o tamanho atual da DLQ.
func (d *DLQStore) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.orders)
}

// List retorna todos os pedidos na DLQ (para debugging).
func (d *DLQStore) List() []*UnmatchedOrder {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make([]*UnmatchedOrder, len(d.orders))
	copy(result, d.orders)
	return result
}

// MatchResult representa o resultado de uma tentativa de matching.
type MatchResult struct {
	Matched      bool    `json:"matched"`
	CourierID    int64   `json:"courier_id,omitempty"`
	CourierName  string  `json:"courier_name,omitempty"`
	DistanceKm   float64 `json:"distance_km,omitempty"`
	BatchID      string  `json:"batch_id,omitempty"`
	Fallback     bool    `json:"fallback,omitempty"`
	StageReached int     `json:"stage_reached"` // 1, 2, ou 3 (qual estagio da busca progressiva matchou)
}

// ZoneMetadata carrega metadados completos de uma zona para o motor de matching.
type ZoneMetadata struct {
	ID                    uint
	Name                  string
	MinRadiusKm           float64
	RadiusKm              float64
	MaxRadiusKm           float64
	PeakRadiusMultiplier  float64
	PeakHourStart         string
	PeakHourEnd           string
	CitySize              string
	DensityCouriersPerKm2 float64
	MinDeliveryFee         float64
	SurgeMultiplier       float64
	MinCouriersThreshold  int
	AllowBatching         bool
}

// IsPeakHour verifica se agora esta dentro do horario de pico da zona.
func (z *ZoneMetadata) IsPeakHour() bool {
	if z.PeakHourStart == "" || z.PeakHourEnd == "" {
		return false
	}
	now := time.Now().Format("15:04")
	return now >= z.PeakHourStart && now <= z.PeakHourEnd
}

// GetEffectiveRadius retorna o raio base ajustado por horario de pico.
func (z *ZoneMetadata) GetEffectiveRadius() float64 {
	if z.IsPeakHour() {
		adjusted := z.RadiusKm * z.PeakRadiusMultiplier
		if adjusted < z.MinRadiusKm {
			return z.MinRadiusKm
		}
		return adjusted
	}
	return z.RadiusKm
}

// GetRadiusStages retorna os 3 estagios de busca progressiva.
func (z *ZoneMetadata) GetRadiusStages() [3]float64 {
	base := z.GetEffectiveRadius()
	return [3]float64{
		base,
		math.Min(base*1.7, z.MaxRadiusKm),
		z.MaxRadiusKm,
	}
}

// GetSuggestedRadiusByDensity calcula raio ideal pela formula de Poisson.
func (z *ZoneMetadata) GetSuggestedRadiusByDensity(targetCouriers int) float64 {
	if z.DensityCouriersPerKm2 <= 0 {
		return z.GetEffectiveRadius()
	}
	suggested := math.Sqrt(float64(targetCouriers) / (math.Pi * z.DensityCouriersPerKm2))
	if suggested < z.MinRadiusKm {
		return z.MinRadiusKm
	}
	if suggested > z.MaxRadiusKm {
		return z.MaxRadiusKm
	}
	return suggested
}

// ZoneResolver resolve coordenadas geograficas em uma zona.
type ZoneResolver interface {
	ResolveByLatLng(lat, lng float64) (zoneID uint, zoneName string, radiusKm float64, err error)
	GetDeliveryFee(zoneID uint, distanceKm float64) float64
	GetSurgeMultiplier(zoneID uint) float64
	GetMinCouriersThreshold(zoneID uint) int
	AllowsBatching(zoneID uint) bool
	// Metadados completos da zona para o motor de matching
	GetZoneMetadata(zoneID uint) *ZoneMetadata
}

// MatchingEngine coordena o processo de matching de pedidos com entregadores.
type MatchingEngine struct {
	CourierStore *CourierStore
	ZoneResolver ZoneResolver
	DLQ          *DLQStore

	// Callback para notificar fallback comunitario
	OnFallback func(orderID string, zoneName string)
	// Callback para notificar match
	OnMatch func(orderID string, courierID int64)

	// Metricas para calibracao
	mu              sync.RWMutex
	matchTimeMs     []float64 // historico de tempos de match
	unmatchedOrders int64     // total de pedidos nao matchados
	totalOrders     int64     // total de pedidos processados
}

// NewMatchingEngine cria uma nova instancia do motor de matching.
func NewMatchingEngine(courierStore *CourierStore, zoneResolver ZoneResolver) *MatchingEngine {
	return &MatchingEngine{
		CourierStore: courierStore,
		ZoneResolver: zoneResolver,
		DLQ:          NewDLQStore(1000),
		matchTimeMs:  make([]float64, 0, 1000),
	}
}

// AttemptMatch tenta encontrar um entregador para um pedido usando
// busca progressiva em 3 estagios + densidade de Poisson.
//
// Algoritmo:
//  1. Resolve a zona do estabelecimento e carrega metadados (raios, pico, densidade)
//  2. Se ha densidade suficiente, calcula raio ideal por Poisson
//  3. Busca progressiva: estagio 1 (raio efetivo), estagio 2 (1.7x), estagio 3 (max)
//  4. Se encontrou candidato em qualquer estagio, faz match com scoring
//  5. Se nenhum candidato + poucos entregadores → fallback comunitario
//  6. Se nenhum candidato → DLQ para retry
func (m *MatchingEngine) AttemptMatch(order *dto.OrderDTO) *MatchResult {
	startTime := time.Now()
	result := &MatchResult{Matched: false, StageReached: 0}
	targetCouriers := 3 // numero desejado de candidatos por busca

	// 1. Resolve zona e carrega metadados
	zoneMeta := m.ZoneResolver.GetZoneMetadata(0)
	zoneID, zoneName, _, err := m.ZoneResolver.ResolveByLatLng(
		order.Establishment.Lat,
		order.Establishment.Long,
	)
	if err == nil && zoneID > 0 {
		zoneMeta = m.ZoneResolver.GetZoneMetadata(zoneID)
	} else {
		zoneName = "Default"
	}

	// 2. Calcula raios de busca
	stages := zoneMeta.GetRadiusStages()
	densityRadius := zoneMeta.GetSuggestedRadiusByDensity(targetCouriers)

	// Se a densidade sugere um raio maior que o estagio 1, usa como partida
	if densityRadius > stages[0] && densityRadius < stages[2] {
		stages[0] = densityRadius
	}

	log.Printf("[MATCH] Order %s zone=%q stages=[%.1fkm, %.1fkm, %.1fkm] density=%.2f/km² peak=%v",
		order.OrderId, zoneName, stages[0], stages[1], stages[2],
		zoneMeta.DensityCouriersPerKm2, zoneMeta.IsPeakHour())

	// 3. Busca progressiva em 3 estagios
	var bestCandidate *CourierLocation
	var bestDistance float64

	for stage := 0; stage < 3; stage++ {
		radius := stages[stage]
		if radius <= 0 {
			continue
		}

		candidates := m.CourierStore.FindNearby(
			order.Establishment.Lat,
			order.Establishment.Long,
			radius,
			10,
		)

		if len(candidates) > 0 {
			bestCandidate = candidates[0] // ja ordenado por score
			bestDistance = haversineKm(
				order.Establishment.Lat, order.Establishment.Long,
				bestCandidate.Lat, bestCandidate.Lng,
			)
			result.StageReached = stage + 1
			log.Printf("[MATCH] Order %s found %d candidates at stage %d (%.1fkm)",
				order.OrderId, len(candidates), stage+1, radius)
			break
		}

		log.Printf("[MATCH] Order %s no candidates at stage %d (%.1fkm)",
			order.OrderId, stage+1, radius)
	}

	// 4. Match encontrado
	if bestCandidate != nil {
		result.Matched = true
		result.CourierID = bestCandidate.DeliverymanID
		result.CourierName = bestCandidate.Name
		result.DistanceKm = bestDistance

		m.CourierStore.SetOrdersCount(bestCandidate.DeliverymanID, bestCandidate.CurrentOrders+1)

		if m.OnMatch != nil {
			m.OnMatch(order.OrderId, bestCandidate.DeliverymanID)
		}

		elapsed := time.Since(startTime).Seconds() * 1000
		m.recordMatchTime(elapsed)

		log.Printf("[MATCH] Order %s -> courier %d (%.1fkm, score=%.2f, stage=%d, %.0fms)",
			order.OrderId, bestCandidate.DeliverymanID, result.DistanceKm,
			float64(bestCandidate.score), result.StageReached, elapsed)

		m.recordOrder(false)
		return result
	}

	// 5. Nenhum entregador — fallback comunitario?
	minCouriers := zoneMeta.MinCouriersThreshold
	available := m.CourierStore.CountAvailable(
		order.Establishment.Lat, order.Establishment.Long, stages[2],
	)

	if available < minCouriers {
		result.Fallback = true
		log.Printf("[MATCH] Order %s: only %d couriers available (min %d) — community fallback in zone %q",
			order.OrderId, available, minCouriers, zoneName)

		if m.OnFallback != nil {
			m.OnFallback(order.OrderId, zoneName)
		}
	}

	// 6. DLQ para retry
	m.DLQ.Push(&UnmatchedOrder{
		OrderID:          order.OrderId,
		EstablishmentLat: order.Establishment.Lat,
		EstablishmentLng: order.Establishment.Long,
		ZoneID:           zoneID,
		CreatedAt:        time.Now().UnixMilli(),
		RetryCount:       0,
		LastAttemptAt:    time.Now().UnixMilli(),
	})

	surge := zoneMeta.SurgeMultiplier
	fee := zoneMeta.MinDeliveryFee
	log.Printf("[MATCH] Order %s unmatched -> DLQ (zone=%s, surge=%.1fx, min_fee=%.2f, stage=%d)",
		order.OrderId, zoneName, surge, fee, result.StageReached)

	m.recordOrder(true)
	return result
}

// RetryUnmatched tenta re-casar pedidos na DLQ.
func (m *MatchingEngine) RetryUnmatched() int {
	retried := 0
	for {
		order := m.DLQ.PopNext()
		if order == nil {
			break
		}

		dto := &dto.OrderDTO{
			OrderId: order.OrderID,
			Establishment: dto.EstablishmentDTO{
				Lat:  order.EstablishmentLat,
				Long: order.EstablishmentLng,
			},
		}

		result := m.AttemptMatch(dto)
		if result.Matched {
			retried++
		} else {
			order.RetryCount++
			order.LastAttemptAt = time.Now().UnixMilli()
			if order.RetryCount < 3 {
				m.DLQ.Push(order)
			} else {
				log.Printf("[DLQ] Order %s expired after %d retries", order.OrderID, order.RetryCount)
			}
		}
	}
	return retried
}

// StartRetryLoop inicia goroutine de retry periodico.
func (m *MatchingEngine) StartRetryLoop(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			retried := m.RetryUnmatched()
			if retried > 0 {
				log.Printf("[MATCH] Retry loop: %d orders re-matched from DLQ", retried)
			}
		}
	}()
}

// CheckBatching avalia se um novo pedido cabe no lote de um entregador em rota.
func (m *MatchingEngine) CheckBatching(existingOrderLat, existingOrderLng, newOrderLat, newOrderLng, restaurantLat, restaurantLng float64) bool {
	const maxDetourKm = 3.0
	distToExisting := haversineKm(restaurantLat, restaurantLng, existingOrderLat, existingOrderLng)
	distToNew := haversineKm(restaurantLat, restaurantLng, newOrderLat, newOrderLng)
	distBetween := haversineKm(existingOrderLat, existingOrderLng, newOrderLat, newOrderLng)

	return distBetween <= maxDetourKm && distToNew <= distToExisting*1.5
}

// --- Metricas para calibracao ---

func (m *MatchingEngine) recordMatchTime(ms float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.matchTimeMs = append(m.matchTimeMs, ms)
	if len(m.matchTimeMs) > 10000 {
		m.matchTimeMs = m.matchTimeMs[len(m.matchTimeMs)-10000:]
	}
}

func (m *MatchingEngine) recordOrder(unmatched bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalOrders++
	if unmatched {
		m.unmatchedOrders++
	}
}

// GetUnmatchedRate retorna a taxa de pedidos nao matchados (0.0 a 1.0).
func (m *MatchingEngine) GetUnmatchedRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.totalOrders == 0 {
		return 0
	}
	return float64(m.unmatchedOrders) / float64(m.totalOrders)
}

// GetMatchTimeP90 retorna o percentil 90 do tempo de matching em ms.
func (m *MatchingEngine) GetMatchTimeP90() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.matchTimeMs) == 0 {
		return 0
	}
	sorted := make([]float64, len(m.matchTimeMs))
	copy(sorted, m.matchTimeMs)
	// Simple sort for small slices; for production use a proper sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	idx := int(float64(len(sorted)) * 0.9)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// GetTotalOrders retorna o total de pedidos processados.
func (m *MatchingEngine) GetTotalOrders() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalOrders
}

// ToJSON serializa MatchResult como JSON.
func (m *MatchResult) ToJSON() []byte {
	data, _ := json.Marshal(m)
	return data
}

// DefaultZoneResolver implementa ZoneResolver com valores fixos.
type DefaultZoneResolver struct{}

func (d *DefaultZoneResolver) ResolveByLatLng(lat, lng float64) (uint, string, float64, error) {
	return 0, "Default", 10.0, nil
}
func (d *DefaultZoneResolver) GetDeliveryFee(zoneID uint, distanceKm float64) float64 {
	return 5.0 + math.Max(0, distanceKm-3.0)*1.5
}
func (d *DefaultZoneResolver) GetSurgeMultiplier(zoneID uint) float64 {
	return 1.0
}
func (d *DefaultZoneResolver) GetMinCouriersThreshold(zoneID uint) int {
	return 3
}
func (d *DefaultZoneResolver) AllowsBatching(zoneID uint) bool {
	return true
}
func (d *DefaultZoneResolver) GetZoneMetadata(zoneID uint) *ZoneMetadata {
	return &ZoneMetadata{
		RadiusKm:              10.0,
		MaxRadiusKm:           15.0,
		MinRadiusKm:           2.0,
		PeakRadiusMultiplier:  0.7,
		MinDeliveryFee:        5.0,
		SurgeMultiplier:       1.0,
		MinCouriersThreshold:  3,
		AllowBatching:         true,
		DensityCouriersPerKm2: 0,
	}
}
