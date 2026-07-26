// Package queue fornece uma fila baseada em Redis para substituir o RabbitMQ.
// Usa LPush para publicar e BRPop para consumir. Isso elimina a dependencia
// externa de um broker RabbitMQ e funciona com o Redis nativo do Render.
package queue

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	paymentQueueKey = "queue:payments"
	redisURLEnv     = "REDIS_URL"
)

var (
	rdb    *redis.Client
	once   sync.Once
	initMu sync.RWMutex
)

// getClient retorna o cliente Redis (inicializado sob demanda).
func getClient() *redis.Client {
	once.Do(func() {
		redisURL := os.Getenv(redisURLEnv)
		if redisURL == "" {
			log.Printf("[REDIS_QUEUE] %s nao configurado, fila de pagamentos desativada", redisURLEnv)
			return
		}

		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("[REDIS_QUEUE] Erro ao parsear REDIS_URL: %v", err)
			return
		}

		client := redis.NewClient(opts)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.Ping(ctx).Err(); err != nil {
			log.Printf("[REDIS_QUEUE] Erro ao conectar no Redis: %v", err)
			return
		}

		rdb = client
		log.Println("[REDIS_QUEUE] Conectado ao Redis para fila de pagamentos")
	})
	return rdb
}

// PaymentMessage representa uma mensagem na fila de pagamentos.
type PaymentMessage struct {
	OrderID         string  `json:"order_id"`
	EstablishmentID int64   `json:"establishment_id"`
	Amount          float64 `json:"amount"`
	DeliveryAmount  float64 `json:"delivery_amount"`
	Status          string  `json:"status"`
}

// Publish publica uma mensagem de pagamento na fila Redis.
func Publish(msg PaymentMessage) error {
	client := getClient()
	if client == nil {
		log.Printf("[REDIS_QUEUE] Redis nao disponivel, mensagem descartada: order=%s amount=%.2f", msg.OrderID, msg.Amount)
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := client.LPush(ctx, paymentQueueKey, data).Err(); err != nil {
		log.Printf("[REDIS_QUEUE] Erro ao publicar na fila: %v", err)
		return err
	}

	log.Printf("[REDIS_QUEUE] Pagamento publicado na fila: order=%s amount=%.2f", msg.OrderID, msg.Amount)
	return nil
}

// Subscribe inicia o loop de consumo da fila de pagamentos.
// Bloqueia em BRPop ate receber mensagens. Executa handler para cada uma.
// Roda em uma goroutine. Retorna imediatamente se Redis nao estiver configurado.
func Subscribe(handler func(PaymentMessage)) {
	client := getClient()
	if client == nil {
		log.Printf("[REDIS_QUEUE] Redis nao disponivel, consumer desativado")
		return
	}

	go func() {
		log.Printf("[REDIS_QUEUE] Consumer iniciado, aguardando mensagens em %s...", paymentQueueKey)

		for {
			result, err := client.BRPop(context.Background(), 0, paymentQueueKey).Result()
			if err != nil {
				log.Printf("[REDIS_QUEUE] Erro no BRPop: %v (reconectando em 5s)", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if len(result) < 2 {
				continue
			}

			var msg PaymentMessage
			if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
				log.Printf("[REDIS_QUEUE] Erro ao decodificar mensagem: %v", err)
				continue
			}

			log.Printf("[REDIS_QUEUE] Mensagem recebida: order=%s amount=%.2f status=%s",
				msg.OrderID, msg.Amount, msg.Status)

			handler(msg)
		}
	}()
}

// Close encerra a conexao com o Redis.
func Close() {
	initMu.Lock()
	defer initMu.Unlock()

	if rdb != nil {
		rdb.Close()
		rdb = nil
	}
	log.Println("[REDIS_QUEUE] Conexao Redis encerrada")
}
