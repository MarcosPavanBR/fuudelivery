// Package consumers gerencia o consumo da fila de pagamentos.
// O RedisPaymentConsumer escuta a fila Redis (BRPop) e processa
// aprovacoes automaticamente, creditando valores nas carteiras.
//
// Substitui a implementacao antiga com RabbitMQ (streadway/amqp)
// para eliminar a dependencia de um broker externo.
package consumers

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/services"
)

const paymentQueueKey = "queue:payments"

// RedisPaymentConsumer escuta a fila Redis e processa pagamentos.
type RedisPaymentConsumer struct {
	rdb    *redis.Client
	Wallet *services.WalletService
	done   chan struct{}
	wg     sync.WaitGroup
}

// PaymentMessage representa a mensagem recebida da fila de pagamentos.
type PaymentMessage struct {
	OrderID         string  `json:"order_id"`
	EstablishmentID int64   `json:"establishment_id"`
	Amount          float64 `json:"amount"`
	DeliveryAmount  float64 `json:"delivery_amount"`
	Status          string  `json:"status"`
}

// NewRedisPaymentConsumer cria uma nova instancia do consumer.
// Conecta ao Redis usando REDIS_URL (configurada no Render via env var).
func NewRedisPaymentConsumer() (*RedisPaymentConsumer, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Printf("ERROR: [REDIS_CONSUMER] REDIS_URL nao configurado. Consumer desativado.")
		return nil, nil // nao e erro fatal, apenas desativa o consumer
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}

	log.Println("[REDIS_CONSUMER] Conectado ao Redis para fila de pagamentos")
	return &RedisPaymentConsumer{
		rdb:    client,
		Wallet: services.NewWalletService(),
		done:   make(chan struct{}),
	}, nil
}

// Start inicia o loop de consumo da fila de pagamentos.
// Bloqueia em BRPop ate receber mensagens. Roda em goroutine.
func (r *RedisPaymentConsumer) Start() error {
	if r.rdb == nil {
		log.Println("[REDIS_CONSUMER] Redis nao configurado, consumer nao iniciado")
		return nil
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		log.Printf("[REDIS_CONSUMER] Aguardando pagamentos em %s...", paymentQueueKey)

		for {
			select {
			case <-r.done:
				log.Println("[REDIS_CONSUMER] Consumer encerrado")
				return
			default:
			}

			result, err := r.rdb.BRPop(context.Background(), 0, paymentQueueKey).Result()
			if err != nil {
				log.Printf("ERROR: [REDIS_CONSUMER] Erro no BRPop: %v (reconectando em 5s)", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if len(result) < 2 {
				continue
			}

			var msg PaymentMessage
			if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
				log.Printf("ERROR: [REDIS_CONSUMER] Erro ao decodificar mensagem: %v", err)
				continue
			}

			log.Printf("[REDIS_CONSUMER] Pagamento recebido: order=%s amount=%.2f status=%s",
				msg.OrderID, msg.Amount, msg.Status)

			r.processMessage(msg)
		}
	}()

	log.Println("[REDIS_CONSUMER] Consumer iniciado com sucesso")
	return nil
}

// processMessage processa uma mensagem recebida da fila.
// Se o pagamento estiver aprovado, credita o valor na carteira do restaurante.
func (r *RedisPaymentConsumer) processMessage(msg PaymentMessage) {
	if msg.Status != "approved" {
		log.Printf("[REDIS_CONSUMER] Status inesperado: %s, ignorando", msg.Status)
		return
	}

	// Converte a mensagem para o modelo de pagamento esperado pelo WalletService
	payment := &models.Payment{
		OrderID:         msg.OrderID,
		EstablishmentID: msg.EstablishmentID,
		Amount:          msg.Amount,
		DeliveryAmount:  msg.DeliveryAmount,
		Status:          models.PaymentApproved,
	}

	log.Printf("[REDIS_CONSUMER] Creditando carteira: order=%s amount=%.2f", msg.OrderID, msg.Amount)

	if err := r.Wallet.ProcessPaymentApproval(payment); err != nil {
		log.Printf("ERROR: [REDIS_CONSUMER] Falha ao creditar carteira order=%s: %v", msg.OrderID, err)
		return
	}

	log.Printf("[REDIS_CONSUMER] Carteira creditada com sucesso: order=%s amount=%.2f", msg.OrderID, msg.Amount)
}

// Stop encerra o consumer de forma graciosa.
func (r *RedisPaymentConsumer) Stop() {
	if r.rdb == nil {
		return
	}
	close(r.done)
	r.wg.Wait()
	r.rdb.Close()
	log.Println("[REDIS_CONSUMER] Conexao Redis encerrada")
}
