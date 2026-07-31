package health

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"
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

// MongoCheck verifica a saude do MongoDB via ping.
func MongoCheck(client *mongo.Client) Check {
	start := time.Now()
	if client == nil {
		return Check{Name: "mongodb", Status: "down", Error: "mongodb not configured"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		return Check{Name: "mongodb", Status: "down", Error: err.Error()}
	}
	return Check{Name: "mongodb", Status: "up", Latency: time.Since(start).String()}
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
