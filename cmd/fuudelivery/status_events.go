package main

// Listeners das filas e o repasse de eventos de status para o WebSocket.

import (
	"encoding/json"
	"fmt"
	"log"

	// Models (database initialization)

	// Handlers

	// Middleware

	// Dispatch engine

	// Batch expiry

	// Queue + Health + Upload + Metrics + Search

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/pkg/queue"
)

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
	_, id := resolveStatusRecipientAccount(queueName, evt)
	return id
}

// resolveStatusRecipientAccount é resolveStatusRecipient com o tipo da conta:
// client_id e user_id são clientes (o estorno publica em user_id o
// CustomerID do pagamento); courier_id é entregador. O WebSocket separa as
// conexões por tipo — o cliente 7 e o entregador 7 são pessoas diferentes.
func resolveStatusRecipientAccount(queueName string, evt *statusEvent) (string, int64) {
	switch {
	case evt.ClientID != 0:
		return middlewares.AccountClient, evt.ClientID
	case evt.UserID != 0:
		return middlewares.AccountClient, evt.UserID
	case queueName == "delivery_updates" && evt.CourierID != 0:
		return middlewares.AccountDeliveryMan, evt.CourierID
	}
	return "", 0
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

	// Mensagens sem destinatário (ex.: community_fallback) são informativas —
	// apenas log, sem erro, para não ir para a DLQ indevidamente.
	kind, recipient := resolveStatusRecipientAccount(queueName, &evt)
	if recipient == 0 {
		return nil
	}

	if err := sendToWS(wsKey{kind, recipient}, msg); err != nil {
		return fmt.Errorf("[QUEUE] %s: falha ao notificar cliente %d: %w", queueName, recipient, err)
	}
	return nil
}
