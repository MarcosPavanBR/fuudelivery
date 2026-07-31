// Package queue fornece uma fila de pagamentos baseada no pacote compartilhado
// pkg/queue. Mantem a API tipada (PaymentMessage) para compatibilidade com
// consumers existentes, delegando a logica Redis para o pacote compartilhado.
package queue

import (
	"encoding/json"
	"log"

	sharedqueue "github.com/carloshomar/fuudelivery/pkg/queue"
)

const paymentQueueKey = "payments"

// PaymentMessage representa uma mensagem na fila de pagamentos.
type PaymentMessage struct {
	OrderID         string  `json:"order_id"`
	EstablishmentID int64   `json:"establishment_id"`
	Amount          float64 `json:"amount"`
	DeliveryAmount  float64 `json:"delivery_amount"`
	Status          string  `json:"status"`
}

// Publish publica uma mensagem de pagamento na fila Redis via pkg/queue compartilhado.
func Publish(msg PaymentMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	q := sharedqueue.New()
	if err := q.Publish(paymentQueueKey, data); err != nil {
		log.Printf("[REDIS_QUEUE] Erro ao publicar na fila: %v", err)
		return err
	}

	log.Printf("[REDIS_QUEUE] Pagamento publicado na fila: order=%s amount=%.2f", msg.OrderID, msg.Amount)
	return nil
}

// Subscribe inicia o loop de consumo da fila de pagamentos via pkg/queue compartilhado.
// Bloqueia em BRPop ate receber mensagens. Executa handler para cada uma.
func Subscribe(handler func(PaymentMessage)) {
	q := sharedqueue.New()
	q.Subscribe(paymentQueueKey, func(data []byte) {
		var msg PaymentMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[REDIS_QUEUE] Erro ao decodificar mensagem: %v", err)
			return
		}
		log.Printf("[REDIS_QUEUE] Mensagem recebida: order=%s amount=%.2f status=%s",
			msg.OrderID, msg.Amount, msg.Status)
		handler(msg)
	})
}

// Close e mantida para compatibilidade. O shared queue gerencia sua propria conexao.
func Close() {
	// A conexao e gerenciada pelo sharedqueue.New() singleton.
	// Esta funcao e mantida para nao quebrar chamadas existentes.
	log.Println("[REDIS_QUEUE] Close chamado (conexao gerenciada pelo pkg/queue compartilhado)")
}
