package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/queue"
)

func startRateLimitCleanup() {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			cutoff := time.Now().Add(-10 * time.Minute)
			ipLimitersMu.Lock()
			for ip, li := range ipLimiters {
				if li.lastSeen.Before(cutoff) {
					delete(ipLimiters, ip)
				}
			}
			ipLimitersMu.Unlock()
		}
	}()
}

// startQueueListeners consome as filas de status do monolito.
// Usa SubscribeFunc (em vez de Subscribe) para que handlers que retornam erro
// ativem o retry (maxRetries) e a dead-letter queue do pkg/queue — mensagens
// malformadas ou com falha de envio não são perdidas em silêncio.
func startQueueListeners() {
	queue.SubscribeFunc("order_updates", func(msg []byte) error {
		return processStatusUpdate("order_updates", msg)
	})

	queue.SubscribeFunc("delivery_updates", func(msg []byte) error {
		return processStatusUpdate("delivery_updates", msg)
	})

	queue.SubscribeFunc("payment_updates", func(msg []byte) error {
		return processStatusUpdate("payment_updates", msg)
	})
}

// statusEvent representa os campos relevantes das mensagens publicadas nas
// filas order_updates/delivery_updates/payment_updates (ex.: o dispatch engine
// publica order_matched com order_id + courier_id).
type statusEvent struct {
	Type      string `json:"type"`
	OrderID   string `json:"order_id"`
	CourierID int64  `json:"courier_id"`
	ClientID  int64  `json:"client_id"`
	UserID    int64  `json:"user_id"`
}

// resolveStatusRecipient define o destinatário WebSocket da mensagem:
// client_id explícito → user_id → courier_id (somente na fila de delivery).
// Retorna 0 quando a mensagem é informativa (sem destinatário).
func resolveStatusRecipient(queueName string, evt *statusEvent) int64 {
	recipient := evt.ClientID
	if recipient == 0 {
		recipient = evt.UserID
	}
	if recipient == 0 && queueName == "delivery_updates" {
		recipient = evt.CourierID
	}
	return recipient
}

// processStatusUpdate decodifica uma mensagem de status da fila, registra no
// log e notifica o cliente WebSocket do destinatário quando identificável.
// Retorna erro apenas quando a mensagem está malformada ou a notificação
// falha — o pkg/queue então re-tenta (maxRetries) e move a mensagem para a DLQ.
func processStatusUpdate(queueName string, msg []byte) error {
	var evt statusEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		return fmt.Errorf("[QUEUE] %s: mensagem inválida: %w", queueName, err)
	}

	log.Printf("[QUEUE] %s: %s", queueName, string(msg))

	recipient := resolveStatusRecipient(queueName, &evt)
	if recipient == 0 {
		return nil
	}

	if err := sendMessageToClient(recipient, msg); err != nil {
		return fmt.Errorf("[QUEUE] %s: falha ao notificar cliente %d: %w", queueName, recipient, err)
	}
	return nil
}
