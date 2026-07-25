package health

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

func DatabaseCheck(db *gorm.DB) Check {
	start := time.Now()
	sqlDB, err := db.DB()
	if err != nil {
		return Check{Name: "postgres", Status: "down", Error: err.Error()}
	}
	if err := sqlDB.Ping(); err != nil {
		return Check{Name: "postgres", Status: "down", Error: err.Error()}
	}
	return Check{Name: "postgres", Status: "up", Latency: time.Since(start).String()}
}

func MongoCheck(client *mongo.Client) Check {
	start := time.Now()
	if err := client.Ping(context.Background(), nil); err != nil {
		return Check{Name: "mongodb", Status: "down", Error: err.Error()}
	}
	return Check{Name: "mongodb", Status: "up", Latency: time.Since(start).String()}
}

// RedisCheck verifica a saude do Redis (opcional).
// Se o cliente for nil ou a conexao falhar, retorna "down" sem crash.
func RedisCheck(client *redis.Client) Check {
	if client == nil {
		return Check{Name: "redis", Status: "down", Error: "not configured"}
	}
	start := time.Now()
	if err := client.Ping(context.Background()).Err(); err != nil {
		return Check{Name: "redis", Status: "down", Error: err.Error()}
	}
	return Check{Name: "redis", Status: "up", Latency: time.Since(start).String()}
}

// RedisGeoCheck verifica se o Redis GEO esta operacional
// testando um GEORADIUS basico.
func RedisGeoCheck(client *redis.Client) Check {
	start := time.Now()
	if client == nil {
		return Check{Name: "redis_geo", Status: "down", Error: "not configured"}
	}

	// Tenta GEORADIUS basico no conjunto courier:location
	_, err := client.GeoRadius(context.Background(), "courier:location", 0, 0, &redis.GeoRadiusQuery{
		Radius: 1,
		Unit:   "km",
		Count:  1,
	}).Result()

	if err != nil && err != redis.Nil {
		return Check{Name: "redis_geo", Status: "degraded", Error: err.Error()}
	}

	// Se nao tem chave ainda, ainda assim o Redis esta operacional
	return Check{Name: "redis_geo", Status: "up", Latency: time.Since(start).String()}
}

// BatchCheck verifica se o sistema de batches esta operacional
// consultando a contagem de batches ativos.
func BatchCheck(db *gorm.DB) Check {
	start := time.Now()
	if db == nil {
		return Check{Name: "batches", Status: "down", Error: "database not configured"}
	}

	var totalBatches int64
	if err := db.Table("batches").Count(&totalBatches).Error; err != nil {
		return Check{Name: "batches", Status: "degraded", Error: err.Error()}
	}

	var activeBatches int64
	db.Table("batches").Where("status IN ('active', 'delivering')").Count(&activeBatches)

	return Check{
		Name:    "batches",
		Status:  "up",
		Latency: time.Since(start).String(),
	}
}
