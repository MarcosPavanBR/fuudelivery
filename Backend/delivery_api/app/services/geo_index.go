package services

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// GeoIndex define as operacoes GEO para o motor de matching.
// CourierStore implementa in-memory via Haversine.
// RedisGeoIndex implementa via Redis GEO (GEORADIUS, GEOADD, etc.).
//
// Trocar entre as implementacoes e feito no initDispatchEngine no main.go:
//   - Para dev/local: courierStore = NewCourierStore()
//   - Para producao:  courierStore = NewRedisGeoIndex(redisClient)
type GeoIndex interface {
	// UpdateLocation atualiza (ou insere) a localizacao de um entregador.
	UpdateLocation(deliverymanID int64, name string, lat, lng float64, status string)

	// RemoveCourier remove um entregador do indice.
	RemoveCourier(deliverymanID int64)

	// SetCourierStatus atualiza o status de um entregador.
	SetCourierStatus(deliverymanID int64, status string)

	// GetCourier retorna a localizacao de um entregador especifico.
	GetCourier(deliverymanID int64) *CourierLocation

	// FindNearby retorna entregadores disponiveis dentro de um raio (km),
	// ordenados por score ponderado.
	FindNearby(lat, lng, radiusKm float64, limit int) []*CourierLocation

	// CountAvailable retorna quantos entregadores disponiveis estao dentro do raio.
	CountAvailable(lat, lng, radiusKm float64) int

	// CountTotalByZone retorna o numero total de entregadores em uma zona.
	CountTotalByZone(zoneID uint, zoneCenterLat, zoneCenterLng, zoneRadiusKm float64) int

	// GetZoneDensity retorna a densidade estimada para uma zona.
	GetZoneDensity(zoneID uint) float64

	// SetZoneDensity define a densidade estimada para uma zona.
	SetZoneDensity(zoneID uint, density float64)

	// RecalculateAllDensities recalcula a densidade de todas as zonas.
	RecalculateAllDensities(zones []ZoneInfo)

	// CleanupStale remove entregadores que nao atualizaram posicao nos ultimos N segundos.
	CleanupStale(maxAgeSeconds int64)
}

// RedisGeoIndex implementa GeoIndex usando comandos GEO do Redis.
// Ideal para producao: suporta milhares de consultas GEORADIUS por segundo.
//
// Estrutura de chaves no Redis:
//   - courier:location  -> sorted set (member=deliverymanID, score=geohash)
//   - courier:data:{id} -> hash (name, lat, lng, status, last_update, current_orders, max_orders)
//   - courier:density:{zoneID} -> string (densidade estimada)
type RedisGeoIndex struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisGeoIndex cria um novo indice GEO baseado em Redis.
// Requer um cliente Redis configurado (v8).
func NewRedisGeoIndex(redisURL string) (*RedisGeoIndex, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	// Testa conexao
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("[REDIS_GEO] Conexao estabelecida com Redis GEO")
	return &RedisGeoIndex{
		client: client,
		ctx:    ctx,
	}, nil
}

// NewRedisGeoIndexWithClient cria a partir de um cliente existente.
func NewRedisGeoIndexWithClient(client *redis.Client) *RedisGeoIndex {
	return &RedisGeoIndex{
		client: client,
		ctx:    context.Background(),
	}
}

// --- Metodos GeoIndex ---

func (r *RedisGeoIndex) UpdateLocation(deliverymanID int64, name string, lat, lng float64, status string) {
	pipe := r.client.Pipeline()

	// GEOADD: chave unificada para GEORADIUS
	pipe.GeoAdd(r.ctx, "courier:location", &redis.GeoLocation{
		Name:      formatCourierKey(deliverymanID),
		Latitude:  lat,
		Longitude: lng,
	})

	// Hash com dados do entregador
	pipe.HSet(r.ctx, "courier:data:"+formatCourierKey(deliverymanID), map[string]interface{}{
		"deliveryman_id": deliverymanID,
		"name":           name,
		"lat":            lat,
		"lng":            lng,
		"status":         status,
		"last_update":    time.Now().UnixMilli(),
		"current_orders": 0,
		"max_orders":     3,
	})

	if _, err := pipe.Exec(r.ctx); err != nil {
		log.Printf("[REDIS_GEO] UpdateLocation error: %v", err)
	}
}

func (r *RedisGeoIndex) RemoveCourier(deliverymanID int64) {
	pipe := r.client.Pipeline()
	pipe.ZRem(r.ctx, "courier:location", formatCourierKey(deliverymanID))
	pipe.Del(r.ctx, "courier:data:"+formatCourierKey(deliverymanID))
	pipe.Exec(r.ctx)
}

func (r *RedisGeoIndex) SetCourierStatus(deliverymanID int64, status string) {
	r.client.HSet(r.ctx, "courier:data:"+formatCourierKey(deliverymanID), "status", status)
}

func (r *RedisGeoIndex) GetCourier(deliverymanID int64) *CourierLocation {
	data, err := r.client.HGetAll(r.ctx, "courier:data:"+formatCourierKey(deliverymanID)).Result()
	if err != nil || len(data) == 0 {
		return nil
	}
	return hashToCourier(data)
}

func (r *RedisGeoIndex) FindNearby(lat, lng, radiusKm float64, limit int) []*CourierLocation {
	// Converte km para metros (Redis GEORADIUS usa metros)
	radiusM := radiusKm * 1000

	// Busca membros dentro do raio
	results, err := r.client.GeoRadius(r.ctx, "courier:location", lng, lat, &redis.GeoRadiusQuery{
		Radius:      radiusM,
		Unit:        "m",
		WithCoord:   true,
		WithDist:    true,
		Count:       limit,
		Sort:        "ASC",
	}).Result()

	if err != nil {
		log.Printf("[REDIS_GEO] FindNearby error: %v", err)
		return nil
	}

	if len(results) == 0 {
		return nil
	}

	couriers := make([]*CourierLocation, 0, len(results))
	for _, loc := range results {
		courierID := parseCourierKey(loc.Name)
		if courierID == 0 {
			continue
		}

		// Busca dados completos
		data, err := r.client.HGetAll(r.ctx, "courier:data:"+loc.Name).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		c := hashToCourier(data)
		if c == nil {
			continue
		}
		if c.Status != "available" {
			continue
		}
		if c.CurrentOrders >= c.MaxOrders {
			continue
		}

		couriers = append(couriers, c)
	}

	return couriers
}

func (r *RedisGeoIndex) CountAvailable(lat, lng, radiusKm float64) int {
	return len(r.FindNearby(lat, lng, radiusKm, 0))
}

func (r *RedisGeoIndex) CountTotalByZone(zoneID uint, zoneCenterLat, zoneCenterLng, zoneRadiusKm float64) int {
	// No Redis GEO, conta todos os membros (qualquer status) dentro do raio
	radiusM := zoneRadiusKm * 1000
	results, err := r.client.GeoRadius(r.ctx, "courier:location", zoneCenterLng, zoneCenterLat, &redis.GeoRadiusQuery{
		Radius: radiusM,
		Unit:   "m",
	}).Result()

	if err != nil {
		return 0
	}
	return len(results)
}

func (r *RedisGeoIndex) GetZoneDensity(zoneID uint) float64 {
	val, err := r.client.Get(r.ctx, "courier:density:"+formatUintKey(zoneID)).Float64()
	if err != nil {
		return 0
	}
	return val
}

func (r *RedisGeoIndex) SetZoneDensity(zoneID uint, density float64) {
	r.client.Set(r.ctx, "courier:density:"+formatUintKey(zoneID), density, 24*time.Hour)
}

func (r *RedisGeoIndex) RecalculateAllDensities(zones []ZoneInfo) {
	for _, z := range zones {
		// Estima densidade: total_couriers / (pi * raio²)
		total := r.CountTotalByZone(z.ID, z.CenterLat, z.CenterLng, z.RadiusKm)
		area := 3.14159 * z.RadiusKm * z.RadiusKm
		if area <= 0 {
			continue
		}
		density := float64(total) / area
		r.SetZoneDensity(z.ID, density)
	}
}

func (r *RedisGeoIndex) CleanupStale(maxAgeSeconds int64) {
	// Redis GEO nao precisa de cleanup manual
	// A expiracao e feita pelo TTL dos dados individuais ou
	// pelo job de heartbeat que atualiza a cada 5-10s
}

// --- Helpers ---

const courierKeyPrefix = "courier:"

func formatCourierKey(deliverymanID int64) string {
	buf := make([]byte, 0, 20)
	buf = append(buf, courierKeyPrefix...)
	buf = append(buf, []byte(formatInt64(deliverymanID))...)
	return string(buf)
}

func parseCourierKey(key string) int64 {
	if len(key) <= len(courierKeyPrefix) {
		return 0
	}
	return parseInt64(key[len(courierKeyPrefix):])
}

func formatUintKey(id uint) string {
	// Conversao simples de uint para string
	if id == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	n := id
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if negative {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func parseInt64(s string) int64 {
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		} else {
			return 0
		}
	}
	return n
}

// hashToCourier converte um map[string]string do Redis para CourierLocation.
func hashToCourier(data map[string]string) *CourierLocation {
	if data["deliveryman_id"] == "" {
		return nil
	}

	id := parseInt64(data["deliveryman_id"])
	if id == 0 {
		return nil
	}

	return &CourierLocation{
		DeliverymanID: id,
		Name:          data["name"],
		Lat:           parseFloat64(data["lat"]),
		Lng:           parseFloat64(data["lng"]),
		LastUpdate:    parseInt64(data["last_update"]),
		Status:        data["status"],
		CurrentOrders: int(parseInt64(data["current_orders"])),
		MaxOrders:     int(parseInt64(data["max_orders"])),
	}
}

func parseFloat64(s string) float64 {
	if s == "" {
		return 0
	}
	// Parse manual simples
	var result float64
	var decimal float64 = 1
	dec := false
	neg := false
	for _, c := range s {
		if c == '-' {
			neg = true
			continue
		}
		if c == '.' {
			dec = true
			continue
		}
		if c >= '0' && c <= '9' {
			if dec {
				decimal /= 10
				result += float64(c-'0') * decimal
			} else {
				result = result*10 + float64(c-'0')
			}
		}
	}
	if neg {
		return -result
	}
	return result
}
