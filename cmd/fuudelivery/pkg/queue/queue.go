package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/go-redis/redis/v8"
)

var (
	ctx          = context.Background()
	rdb          *redis.Client
	useRedis     bool
	internalQueues = make(map[string]chan []byte)
	internalMu   sync.Mutex
	clients      = make(map[int64]interface{})
	clientsMu    sync.Mutex
)

type Message struct {
	Queue   string          `json:"queue"`
	Payload json.RawMessage `json:"payload"`
}

func Init() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("Redis URL parse error: %v, falling back to internal queue", err)
			useRedis = false
			return
		}
		rdb = redis.NewClient(opt)
		_, err = rdb.Ping(ctx).Result()
		if err != nil {
			log.Printf("Redis connection error: %v, falling back to internal queue", err)
			useRedis = false
			return
		}
		useRedis = true
		log.Println("Message queue: Redis connected")
	} else {
		log.Println("Message queue: Internal Go channels (no Redis configured)")
	}
}

func Publish(queueName string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if useRedis {
		return rdb.LPush(ctx, "queue:"+queueName, data).Err()
	}

	internalMu.Lock()
	ch, ok := internalQueues[queueName]
	if !ok {
		ch = make(chan []byte, 100)
		internalQueues[queueName] = ch
	}
	internalMu.Unlock()

	select {
	case ch <- data:
	default:
		log.Printf("Queue %s full, dropping message", queueName)
	}
	return nil
}

func Subscribe(queueName string, handler func([]byte)) {
	if useRedis {
		go func() {
			sub := rdb.Subscribe(ctx, "pubsub:"+queueName)
			ch := sub.Channel()
			for msg := range ch {
				handler([]byte(msg.Payload))
			}
		}()
		return
	}

	internalMu.Lock()
	ch, ok := internalQueues[queueName]
	if !ok {
		ch = make(chan []byte, 100)
		internalQueues[queueName] = ch
	}
	internalMu.Unlock()

	go func() {
		for msg := range ch {
			handler(msg)
		}
	}()
}

func SendMessageToClient(clientID int64, message []byte) error {
	log.Printf("[WS] Message for client %d: %s", clientID, string(message))
	return Publish("ws:client:"+formatInt(clientID), message)
}

func formatInt(n int64) string {
	return fmt.Sprintf("%d", n)
}

func RegisterClient(clientID int64) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	clients[clientID] = struct{}{}
}

func UnregisterClient(clientID int64) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients, clientID)
}
