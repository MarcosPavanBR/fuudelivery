// Package gateway define a abstração de múltiplos gateways de pagamento
// para o FuuDelivery. Este arquivo implementa o normalizador de webhooks
// e o publicador de eventos de pagamento no Redis.
//
// # Fluxo dos eventos
//
//  1. Cada gateway recebe seu webhook específico (HMAC/Token validation)
//  2. O gateway.ParseWebhook() retorna um WebhookEvent normalizado
//  3. O WebhookNormalizer.process() aplica regras de negócio:
//     - Idempotência (evitar processar o mesmo evento 2x)
//     - Conversão de status externo → status interno
//     - Enriquecimento com dados do banco
//  4. O resultado é publicado no Redis Pub/Sub para consumo assíncrono
//
// # Canais Redis
//
//	pagamento:confirmado   — pagamento aprovado (capturado)
//	pagamento:rejeitado    — pagamento falhou
//	pagamento:estornado    — reembolso processado
//	pagamento:cancelado    — pagamento cancelado/voided
//	pagamento:pendente     — pagamento aguardando
//	pagamento:split_ok     — split processado com sucesso
//	pagamento:split_erro   — split falhou
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// ═══════════════════════════════════════════════════════════════
// TIPOS DO SISTEMA DE EVENTOS
// ═══════════════════════════════════════════════════════════════

// EventChannel representa um canal Redis para pub/sub de eventos de pagamento.
type EventChannel string

const (
	// ChannelPagamentoConfirmado é publicado quando um pagamento é aprovado/capturado.
	ChannelPagamentoConfirmado EventChannel = "pagamento:confirmado"

	// ChannelPagamentoRejeitado é publicado quando um pagamento falha.
	ChannelPagamentoRejeitado EventChannel = "pagamento:rejeitado"

	// ChannelPagamentoEstornado é publicado quando um reembolso é processado.
	ChannelPagamentoEstornado EventChannel = "pagamento:estornado"

	// ChannelPagamentoCancelado é publicado quando um pagamento é cancelado/voided.
	ChannelPagamentoCancelado EventChannel = "pagamento:cancelado"

	// ChannelPagamentoPendente é publicado quando um pagamento está aguardando.
	ChannelPagamentoPendente EventChannel = "pagamento:pendente"

	// ChannelSplitOk é publicado quando o split é processado com sucesso.
	ChannelSplitOk EventChannel = "pagamento:split_ok"

	// ChannelSplitErro é publicado quando o split falha.
	ChannelSplitErro EventChannel = "pagamento:split_erro"
)

// PaymentEvent é o evento normalizado publicado no Redis.
// Contém todas as informações necessárias para o handler assíncrono processar.
type PaymentEvent struct {
	// ID único do evento (para idempotência)
	EventID string `json:"event_id"`

	// Canal Redis de destino
	Channel EventChannel `json:"channel"`

	// Dados do pagamento
	PaymentID        int64              `json:"payment_id"`         // ID interno no FuuDelivery
	PaymentExternalID string            `json:"payment_external_id"` // ID no gateway
	GatewayName      string             `json:"gateway_name"`       // pagarme, asaas, etc.
	OrderID          int64              `json:"order_id"`           // ID do pedido关联

	// Status
	Status           TransactionStatus  `json:"status"`            // Status interno normalizado
	PreviousStatus   TransactionStatus  `json:"previous_status,omitempty"`

	// Valores (em centavos)
	Amount           int64              `json:"amount"`             // Valor total
	CapturedAmount   int64              `json:"captured_amount,omitempty"`

	// Split
	SplitStatus      string             `json:"split_status,omitempty"` // ok, erro, pendente, nao_aplicavel
	SplitDetails     []SplitResult      `json:"split_details,omitempty"`

	// Timestamps
	ReceivedAt       time.Time          `json:"received_at"`
	ProcessedAt      time.Time          `json:"processed_at,omitempty"`

	// Metadados extras do gateway
	Metadata         map[string]string  `json:"metadata,omitempty"`
}

// SplitResult representa o resultado do processamento de um split para um recebedor.
type SplitResult struct {
	RecipientID    int64  `json:"recipient_id"`    // ID interno do recebedor
	GatewayID      string `json:"gateway_id"`      // ID da sub-conta no gateway
	RecipientType  string `json:"recipient_type"`  // establishment, deliveryman, platform
	Amount         int64  `json:"amount"`          // Valor em centavos
	Status         string `json:"status"`          // ok, erro, pendente
	ErrorMessage   string `json:"error_message,omitempty"`
}

// ═══════════════════════════════════════════════════════════════
// NORMALIZADOR DE WEBHOOKS
// ═══════════════════════════════════════════════════════════════

// WebhookNormalizer processa webhooks de múltiplos gateways e publica
// eventos normalizados no Redis.
//
// Uso:
//
//	normalizer := NewWebhookNormalizer(redisClient, registry)
//	err := normalizer.ProcessWebhook(ctx, "pagarme", body, headers)
type WebhookNormalizer struct {
	rdb      *redis.Client
	registry *Registry
}

// NewWebhookNormalizer cria uma nova instância do normalizador.
//
// Parâmetros:
//   - rdb: cliente Redis para pub/sub
//   - registry: registry de gateways para selecionar o adapter correto
func NewWebhookNormalizer(rdb *redis.Client, registry *Registry) *WebhookNormalizer {
	return &WebhookNormalizer{
		rdb:      rdb,
		registry: registry,
	}
}

// ProcessWebhook processa um webhook recebido de um gateway específico.
//
// Fluxo:
//  1. Obtém o gateway do registry
//  2. Valida a assinatura (HMAC/Token)
//  3. Faz o parse do payload para WebhookEvent
//  4. Converte para PaymentEvent normalizado
//  5. Publica no Redis Pub/Sub
//
// Retorna o PaymentEvent publicado ou erro.
func (n *WebhookNormalizer) ProcessWebhook(
	ctx context.Context,
	gatewayName string,
	body []byte,
	headers map[string]string,
) (*PaymentEvent, error) {
	// 1. Obtém o gateway
	gw, err := n.registry.Get(gatewayName)
	if err != nil {
		return nil, fmt.Errorf("gateway não encontrado: %w", err)
	}

	// 2. Valida assinatura
	if !gw.ValidateWebhook(body, headers) {
		return nil, fmt.Errorf("assinatura do webhook inválida para %s", gatewayName)
	}

	// 3. Faz o parse
	event, err := gw.ParseWebhook(body)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear webhook: %w", err)
	}

	// 4. Converte para PaymentEvent normalizado
	paymentEvent := n.normalizeEvent(event)

	// 5. Publica no Redis
	if err := n.publish(ctx, paymentEvent); err != nil {
		return nil, fmt.Errorf("erro ao publicar evento: %w", err)
	}

	log.Printf("[WEBHOOK_NORMALIZER] ✅ Evento publicado: channel=%s, gateway=%s, payment=%s",
		paymentEvent.Channel, paymentEvent.GatewayName, paymentEvent.PaymentExternalID)

	return paymentEvent, nil
}

// normalizeEvent converte um WebhookEvent (vindo do gateway) em um
// PaymentEvent normalizado com o canal Redis correto.
func (n *WebhookNormalizer) normalizeEvent(event *WebhookEvent) *PaymentEvent {
	pe := &PaymentEvent{
		EventID:           event.ID,
		PaymentExternalID: event.GatewayID,
		GatewayName:       event.GatewayName,
		Status:            event.Status,
		ReceivedAt:        event.ReceivedAt,
		ProcessedAt:       time.Now(),
		Metadata:          event.Metadata,
	}

	// Determina o canal Redis com base no status
	switch event.Type {
	case WebhookPaymentApproved:
		pe.Channel = ChannelPagamentoConfirmado
	case WebhookPaymentFailed:
		pe.Channel = ChannelPagamentoRejeitado
	case WebhookRefundCompleted:
		pe.Channel = ChannelPagamentoEstornado
	case WebhookPaymentCancelled:
		pe.Channel = ChannelPagamentoCancelado
	case WebhookPaymentPending:
		pe.Channel = ChannelPagamentoPendente
	default:
		pe.Channel = ChannelPagamentoPendente
	}

	return pe
}

// publish serializa e publica um PaymentEvent no Redis Pub/Sub.
func (n *WebhookNormalizer) publish(ctx context.Context, event *PaymentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("erro ao serializar evento: %w", err)
	}

	cmd := n.rdb.Publish(ctx, string(event.Channel), data)
	if cmd.Err() != nil {
		return fmt.Errorf("erro ao publicar no Redis: %w", cmd.Err())
	}

	log.Printf("[REDIS] 📤 Publicado em %s: %d bytes", event.Channel, len(data))
	return nil
}

// ═══════════════════════════════════════════════════════════════
// CONSUMIDOR DE EVENTOS (para handlers assíncronos)
// ═══════════════════════════════════════════════════════════════

// EventSubscriber escuta canais Redis e processa eventos de pagamento.
//
// Uso:
//
//	sub := NewEventSubscriber(redisClient)
//	sub.Subscribe(ctx, ChannelPagamentoConfirmado, func(event PaymentEvent) {
//	    log.Printf("Pagamento confirmado: %d", event.PaymentID)
//	})
type EventSubscriber struct {
	rdb *redis.Client
}

// NewEventSubscriber cria um novo subscriber.
func NewEventSubscriber(rdb *redis.Client) *EventSubscriber {
	return &EventSubscriber{rdb: rdb}
}

// EventHandler é uma função que processa um evento de pagamento.
type EventHandler func(event PaymentEvent)

// Subscribe escuta um canal Redis e chama o handler para cada evento.
//
// Bloqueia até o context ser cancelado. Execute em uma goroutine:
//
//	go subscriber.Subscribe(ctx, ChannelPagamentoConfirmado, handler)
func (s *EventSubscriber) Subscribe(
	ctx context.Context,
	channel EventChannel,
	handler EventHandler,
) {
	pubsub := s.rdb.Subscribe(ctx, string(channel))
	defer pubsub.Close()

	log.Printf("[EVENT_SUBSCRIBER] 🔔 Escutando canal: %s", channel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[EVENT_SUBSCRIBER] ⏹️ Parando subscription: %s", channel)
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			var event PaymentEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("[EVENT_SUBSCRIBER] ❌ Erro ao deserializar: %v", err)
				continue
			}

			log.Printf("[EVENT_SUBSCRIBER] 📥 Evento recebido: %s (payment=%d)",
				event.Channel, event.PaymentID)

			handler(event)
		}
	}
}

// SubscribeMultiple escuta múltiplos canais simultaneamente.
func (s *EventSubscriber) SubscribeMultiple(
	ctx context.Context,
	channels []EventChannel,
	handler EventHandler,
) {
	for _, ch := range channels {
		go s.Subscribe(ctx, ch, handler)
	}
}
