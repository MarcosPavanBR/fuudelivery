package health

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Check representa o resultado de uma verificacao de saude.
type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "up", "degraded", "down"
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// OverallStatus retorna o status geral baseado nos checks individuais.
// Se qualquer check estiver "down", retorna "down".
// Se qualquer check estiver "degraded", retorna "degraded".
func OverallStatus(checks ...Check) string {
	hasDegraded := false
	for _, c := range checks {
		switch c.Status {
		case "down":
			return "down"
		case "degraded":
			hasDegraded = true
		}
	}
	if hasDegraded {
		return "degraded"
	}
	return "up"
}

// DatabaseCheck verifica a saude do PostgreSQL via GORM.
func DatabaseCheck(db *gorm.DB) Check {
	start := time.Now()
	if db == nil {
		return Check{Name: "postgres", Status: "down", Error: "database not configured"}
	}
	sqlDB, err := db.DB()
	if err != nil {
		return Check{Name: "postgres", Status: "down", Error: err.Error()}
	}
	if err := sqlDB.Ping(); err != nil {
		return Check{Name: "postgres", Status: "down", Error: err.Error()}
	}
	return Check{Name: "postgres", Status: "up", Latency: time.Since(start).String()}
}

// RedisCheck verifica a saude do Redis via ping.
func RedisCheck(client *redis.Client) Check {
	start := time.Now()
	if client == nil {
		return Check{Name: "redis", Status: "down", Error: "redis not configured"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return Check{Name: "redis", Status: "down", Error: err.Error()}
	}
	return Check{Name: "redis", Status: "up", Latency: time.Since(start).String()}
}

// RedisGeoCheck verifica se o Redis GEO esta operacional.
func RedisGeoCheck(client *redis.Client) Check {
	start := time.Now()
	if client == nil {
		return Check{Name: "redis_geo", Status: "down", Error: "redis not configured"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := client.GeoRadius(ctx, "courier:location", 0, 0, &redis.GeoRadiusQuery{
		Radius: 1,
		Unit:   "km",
		Count:  1,
	}).Result()
	if err != nil && err != redis.Nil {
		return Check{Name: "redis_geo", Status: "degraded", Error: err.Error()}
	}
	return Check{Name: "redis_geo", Status: "up", Latency: time.Since(start).String()}
}

// BatchCheck verifica se o sistema de batches esta operacional.
func BatchCheck(db *gorm.DB) Check {
	start := time.Now()
	if db == nil {
		return Check{Name: "batches", Status: "down", Error: "database not configured"}
	}
	var totalBatches int64
	if err := db.Table("batches").Count(&totalBatches).Error; err != nil {
		return Check{Name: "batches", Status: "degraded", Error: err.Error()}
	}
	return Check{Name: "batches", Status: "up", Latency: time.Since(start).String()}
}

// GatewayCheck verifica se pelo menos um gateway de pagamento esta configurado.
// Retorna "up" se pelo menos uma API key estiver presente, "down" caso contrario.
func GatewayCheck() Check {
	start := time.Now()
	gateways := map[string]string{
		"abacatepay":  os.Getenv("ABACATE_PAY_API_KEY"),
		"pagarme":     os.Getenv("PAGARME_API_KEY"),
		"asaas":       os.Getenv("ASAAS_API_KEY"),
		"mercadopago": os.Getenv("MERCADOPAGO_ACCESS_TOKEN"),
	}

	available := make([]string, 0)
	for name, key := range gateways {
		if key != "" {
			available = append(available, name)
		}
	}

	if len(available) == 0 {
		return Check{Name: "payment_gateways", Status: "down", Error: "no payment gateway configured"}
	}

	return Check{
		Name:    "payment_gateways",
		Status:  "up",
		Latency: time.Since(start).String(),
		Error:   fmt.Sprintf("available: %v", available),
	}
}

// FiberHandler monta o handler HTTP (Fiber) padrao para o endpoint /health.
//
// Retorna HTTP 200 com status "up" quando todos os checks passam, ou HTTP 503
// com o status real ("degraded"/"down") caso contrario — o que permite que o
// Render (ou qualquer load balancer) marque o servico como unhealthy.
//
// Uso:
//
//	app.Get("/health", health.FiberHandler("delivery_api",
//	))
func FiberHandler(service string, checks ...Check) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status := OverallStatus(checks...)
		payload := fiber.Map{
			"status":  status,
			"service": service,
			"checks":  toFiberMap(checks),
		}
		if status != "up" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(payload)
		}
		return c.JSON(payload)
	}
}

// toFiberMap converte a lista de checks em um mapa {nome: check} para o JSON.
func toFiberMap(checks []Check) fiber.Map {
	result := make(fiber.Map, len(checks))
	for _, check := range checks {
		result[check.Name] = check
	}
	return result
}
