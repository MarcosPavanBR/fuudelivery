package services

import (
	"math"
	"sort"
	"sync"
	"time"
)

// CourierLocation representa a posicao atual de um entregador.
type CourierLocation struct {
	DeliverymanID int64   `json:"deliveryman_id"`
	Name          string  `json:"name"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	LastUpdate    int64   `json:"last_update"` // unix millis
	Status        string  `json:"status"`      // available, busy, offline

	// Capacidade atual de batching
	CurrentOrders int `json:"current_orders"`
	MaxOrders     int `json:"max_orders"` // maximo de pedidos simultaneos

	// Score interno (nao serializado)
	score float64
}

// CourierStore armazena localizacoes de entregadores em memoria
// com operacoes GEO-like (GEOADD, GEORADIUS) usando Haversine.
type CourierStore struct {
	mu       sync.RWMutex
	couriers map[int64]*CourierLocation

	// Densidade estimada de entregadores ativos por km², indexada por zoneID
	densityByZone map[uint]float64
	// Date da ultima atualizacao de densidade por zoneID
	densityUpdatedAt map[uint]int64
}

// NewCourierStore cria um novo armazenamento de entregadores.
func NewCourierStore() *CourierStore {
	return &CourierStore{
		couriers:         make(map[int64]*CourierLocation),
		densityByZone:    make(map[uint]float64),
		densityUpdatedAt: make(map[uint]int64),
	}
}

// UpdateLocation atualiza (ou insere) a localizacao de um entregador.
func (s *CourierStore) UpdateLocation(deliverymanID int64, name string, lat, lng float64, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.couriers[deliverymanID]; ok {
		existing.Lat = lat
		existing.Lng = lng
		existing.LastUpdate = time.Now().UnixMilli()
		existing.Status = status
		if name != "" {
			existing.Name = name
		}
	} else {
		s.couriers[deliverymanID] = &CourierLocation{
			DeliverymanID: deliverymanID,
			Name:          name,
			Lat:           lat,
			Lng:           lng,
			LastUpdate:    time.Now().UnixMilli(),
			Status:        status,
			CurrentOrders: 0,
			MaxOrders:     3,
		}
	}
}

// RemoveCourier remove um entregador do store.
func (s *CourierStore) RemoveCourier(deliverymanID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.couriers, deliverymanID)
}

// SetCourierStatus atualiza o status de um entregador.
func (s *CourierStore) SetCourierStatus(deliverymanID int64, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.couriers[deliverymanID]; ok {
		c.Status = status
		c.LastUpdate = time.Now().UnixMilli()
	}
}

// GetCourier retorna a localizacao de um entregador especifico.
func (s *CourierStore) GetCourier(deliverymanID int64) *CourierLocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.couriers[deliverymanID]
}

// SetOrdersCount atualiza a contagem de pedidos ativos de um entregador.
func (s *CourierStore) SetOrdersCount(deliverymanID int64, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.couriers[deliverymanID]; ok {
		c.CurrentOrders = count
	}
}

// haversineKm calcula distancia em km entre dois pontos.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// FindNearby retorna entregadores disponiveis dentro de um raio (km),
// ordenados por score ponderado. Equivalente ao GEORADIUS do Redis.
// Score combina: distancia (peso 0.6), capacidade de lote (0.3), tempo ocioso (-0.1).
func (s *CourierStore) FindNearby(lat, lng, radiusKm float64, limit int) []*CourierLocation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []*CourierLocation
	now := time.Now().UnixMilli()

	for _, c := range s.couriers {
		if c.Status != "available" {
			continue
		}
		if c.CurrentOrders >= c.MaxOrders {
			continue
		}

		dist := haversineKm(lat, lng, c.Lat, c.Lng)
		if dist > radiusKm {
			continue
		}

		// Score ponderado: quanto menor, melhor
		distScore := dist / radiusKm
		capScore := float64(c.CurrentOrders) / float64(c.MaxOrders)
		idleHours := float64(now-c.LastUpdate) / 3600000.0
		idleScore := math.Min(idleHours/2.0, 1.0)

		c.score = distScore*0.6 + capScore*0.3 - idleScore*0.1
		candidates = append(candidates, c)
	}

	// Ordena por score (menor = melhor)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates
}

// CountAvailable retorna quantos entregadores disponiveis estao dentro do raio.
func (s *CourierStore) CountAvailable(lat, lng, radiusKm float64) int {
	return len(s.FindNearby(lat, lng, radiusKm, 0))
}

// CountTotalByZone retorna o numero total de entregadores (qualquer status) em uma zona.
// Usa uma heuristica simples: se o entregador estiver dentro do raio padrao da zona.
func (s *CourierStore) CountTotalByZone(zoneID uint, zoneCenterLat, zoneCenterLng, zoneRadiusKm float64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, c := range s.couriers {
		dist := haversineKm(zoneCenterLat, zoneCenterLng, c.Lat, c.Lng)
		if dist <= zoneRadiusKm {
			count++
		}
	}
	return count
}

// EstimateDensity calcula a densidade de entregadores por km² em uma zona.
// densidade = total_couriers / (pi * raio²)
func (s *CourierStore) EstimateDensity(zoneID uint, zoneCenterLat, zoneCenterLng, zoneRadiusKm float64) float64 {
	total := s.CountTotalByZone(zoneID, zoneCenterLat, zoneCenterLng, zoneRadiusKm)
	area := math.Pi * zoneRadiusKm * zoneRadiusKm
	if area <= 0 {
		return 0
	}
	return float64(total) / area
}

// SetZoneDensity define a densidade estimada para uma zona.
func (s *CourierStore) SetZoneDensity(zoneID uint, density float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.densityByZone[zoneID] = density
	s.densityUpdatedAt[zoneID] = time.Now().UnixMilli()
}

// GetZoneDensity retorna a densidade estimada para uma zona.
func (s *CourierStore) GetZoneDensity(zoneID uint) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.densityByZone[zoneID]
}

// RecalculateAllDensities recalcula a densidade de todas as zonas.
// Chamado periodicamente pelo job de calibracao.
func (s *CourierStore) RecalculateAllDensities(zones []ZoneInfo) {
	for _, z := range zones {
		density := s.EstimateDensity(z.ID, z.CenterLat, z.CenterLng, z.RadiusKm)
		s.SetZoneDensity(z.ID, density)
	}
}

// ZoneInfo e uma representacao simplificada de zona para recalculo de densidade.
type ZoneInfo struct {
	ID        uint
	CenterLat float64
	CenterLng float64
	RadiusKm  float64
}

// CleanupStale remove entregadores que nao atualizaram posicao nos ultimos N segundos.
func (s *CourierStore) CleanupStale(maxAgeSeconds int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().UnixMilli() - maxAgeSeconds*1000
	for id, c := range s.couriers {
		if c.LastUpdate <= cutoff {
			delete(s.couriers, id)
		}
	}
}
