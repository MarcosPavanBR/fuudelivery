package health

import (
	"context"
	"time"

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
