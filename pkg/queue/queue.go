// Package queue fornece uma fila de mensagens compartilhada entre os microservicos.
// Suporta Redis (LPush/BRPop) como transport primario e Go channels como fallback
// quando Redis nao esta configurado. Elimina a duplicacao de logica Redis que existia
// em payment_api/webhook.go, Payment/queue/redis_queue.go e cmd/fuudelivery/pkg/queue.
//
// Uso basico:
//
//	q := queue.New()
//	defer q.Close()
//	q.Publish("orders", []byte(`{"id":"123"}`))
//	q.Subscribe("orders", func(msg []byte) { fmt.Println(string(msg)) })
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

// Queue e uma fila de mensagens com Redis como transport e Go channels como fallback.
type Queue struct {
	client         *redis.Client
	useRedis       bool
	internalQueues map[string]chan []byte
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
}

var (
	defaultQueue *Queue
	defaultOnce  sync.Once
)

// New retorna a instancia singleton da Queue.
// Se REDIS_URL nao estiver configurado ou Redis nao estiver acessivel,
// usa Go channels internos como fallback.
// A conexao Redis e inicializada sob demanda na primeira chamada.
func New() *Queue {
	defaultOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		q := &Queue{
			internalQueues: make(map[string]chan []byte),
			ctx:            ctx,
			cancel:         cancel,
		}

		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			log.Println("[QUEUE] REDIS_URL nao configurado, usando Go channels (fallback)")
			defaultQueue = q
			return
		}

		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("[QUEUE] Erro ao parsear REDIS_URL: %v, usando Go channels", err)
			defaultQueue = q
			return
		}

		client := redis.NewClient(opts)
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()

		if err := client.Ping(pingCtx).Err(); err != nil {
			log.Printf("[QUEUE] Erro ao conectar no Redis: %v, usando Go channels", err)
			defaultQueue = q
			return
		}

		q.client = client
		q.useRedis = true
		log.Println("[QUEUE] Conectado ao Redis")
		defaultQueue = q
	})
	return defaultQueue
}

// Publish envia uma mensagem para a fila identificada por queueName.
// Se Redis estiver disponivel, usa LPush. Caso contrario, usa Go channels.
func (q *Queue) Publish(queueName string, data []byte) error {
	if q.useRedis && q.client != nil {
		return q.client.LPush(q.ctx, "queue:"+queueName, data).Err()
	}

	// Fallback: Go channel
	q.mu.Lock()
	ch, ok := q.internalQueues[queueName]
	if !ok {
		ch = make(chan []byte, 100)
		q.internalQueues[queueName] = ch
	}
	q.mu.Unlock()

	select {
	case ch <- data:
	default:
		log.Printf("[QUEUE] Fila %s cheia, mensagem descartada", queueName)
	}
	return nil
}

// PublishJSON serializa o payload para JSON e publica na fila.
func (q *Queue) PublishJSON(queueName string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return q.Publish(queueName, data)
}

// Subscribe inicia o consumo de mensagens da fila queueName.
// Se Redis estiver disponivel, usa BRPop (bloqueante). Caso contrario, usa Go channels.
// Roda em goroutine. Retorna imediatamente se Redis nao estiver configurado.
func (q *Queue) Subscribe(queueName string, handler func([]byte)) {
	if q.useRedis && q.client != nil {
		go func() {
			log.Printf("[QUEUE] Consumer Redis iniciado em %s...", queueName)
			for {
				result, err := q.client.BRPop(q.ctx, 0, "queue:"+queueName).Result()
				if err != nil {
					if q.ctx.Err() != nil {
						log.Printf("[QUEUE] Consumer Redis encerrado em %s (shutdown)", queueName)
						return
					}
					log.Printf("[QUEUE] Erro no BRPop (%s): %v, reconectando em 5s", queueName, err)
					time.Sleep(5 * time.Second)
					continue
				}
				if len(result) < 2 {
					continue
				}
				handler([]byte(result[1]))
			}
		}()
		return
	}

	// Fallback: Go channel
	q.mu.Lock()
	ch, ok := q.internalQueues[queueName]
	if !ok {
		ch = make(chan []byte, 100)
		q.internalQueues[queueName] = ch
	}
	q.mu.Unlock()

	go func() {
		log.Printf("[QUEUE] Consumer Go channel iniciado em %s...", queueName)
		for msg := range ch {
			handler(msg)
		}
	}()
}

// Close encerra a conexao com Redis (se ativa) e cancela todas as goroutines de consumer.
func (q *Queue) Close() {
	if q.cancel != nil {
		q.cancel()
	}
	if q.client != nil {
		q.client.Close()
		log.Println("[QUEUE] Conexao Redis encerrada")
	}
}

// IsRedis retorna true se a fila esta usando Redis como transport.
func (q *Queue) IsRedis() bool {
	return q.useRedis
}

// GetClient retorna o cliente Redis da fila (nil se nao estiver usando Redis).
// Utilizado pelo health endpoint para reutilizar a conexao existente.
func (q *Queue) GetClient() *redis.Client {
	return q.client
}

// GetClient retorna o cliente Redis da fila singleton.
func GetClient() *redis.Client {
	return New().GetClient()
}

// --- Package-level convenience functions ---
// Estas funcoes operam no singleton defaultQueue para compatibilidade
// com chamadas como queue.Init(), queue.Publish(), queue.Subscribe().

// Init inicializa a fila singleton. Chamar New() e suficiente;
// esta funcao existe para compatibilidade com codigo legado.
func Init() {
	New()
}

// Publish envia uma mensagem usando a fila singleton.
func Publish(queueName string, data []byte) error {
	return New().Publish(queueName, data)
}

// PublishJSON serializa e envia uma mensagem usando a fila singleton.
func PublishJSON(queueName string, payload interface{}) error {
	return New().PublishJSON(queueName, payload)
}

// Subscribe consome mensagens da fila usando a fila singleton.
func Subscribe(queueName string, handler func([]byte)) {
	New().Subscribe(queueName, handler)
}

// CloseQueue encerra a conexao da fila singleton.
func CloseQueue() {
	New().Close()
}
