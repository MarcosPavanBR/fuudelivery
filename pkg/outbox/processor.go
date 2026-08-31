package outbox

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// OutboxProcessor processa eventos pendentes e publica em Redis Streams
type OutboxProcessor struct {
	db           *gorm.DB
	redis        *redis.Client
	streamName   string
	batchSize    int
	pollInterval time.Duration
}

// NewOutboxProcessor cria novo processador de outbox
func NewOutboxProcessor(db *gorm.DB, redis *redis.Client, streamName string) *OutboxProcessor {
	return &OutboxProcessor{
		db:           db,
		redis:        redis,
		streamName:   streamName,
		batchSize:    100,
		pollInterval: 5 * time.Second,
	}
}

// Start inicia o processamento contínuo de eventos
func (p *OutboxProcessor) Start(ctx context.Context) {
	log.Printf("Outbox processor started for stream: %s", p.streamName)
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Outbox processor stopped")
			return
		case <-ticker.C:
			p.processBatch(ctx)
		}
	}
}

// processBatch processa um lote de eventos pendentes
func (p *OutboxProcessor) processBatch(ctx context.Context) {
	events, err := GetUnprocessedEvents(p.db, p.batchSize)
	if err != nil {
		log.Printf("Error fetching unprocessed events: %v", err)
		return
	}

	if len(events) == 0 {
		return
	}

	log.Printf("Processing %d outbox events", len(events))

	for _, event := range events {
		// Tenta reservar o evento
		tx := p.db.Begin()
		if err := event.ClaimForProcessing(tx); err != nil {
			tx.Rollback()
			continue // Outro worker já pegou
		}

		// Publica no Redis Stream
		err := p.publishEvent(ctx, event)
		if err != nil {
			log.Printf("Error publishing event %s: %v", event.ID, err)
			event.MarkAsFailed(tx, err.Error())
			tx.Commit()
			continue
		}

		// Marca como processado
		event.MarkAsProcessed(tx)
		tx.Commit()
	}
}

// publishEvent publica evento no Redis Stream
func (p *OutboxProcessor) publishEvent(ctx context.Context, event OutboxEvent) error {
	// Adiciona metadata de tracing
	values := map[string]interface{}{
		"id":             event.ID,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"event_type":     event.EventType,
		"payload":        event.Payload,
		"metadata":       event.Metadata,
		"created_at":     event.CreatedAt.Format(time.RFC3339),
	}

	// Se tiver metadata JSON, extrai trace_id se existir
	if event.Metadata != "" {
		// Pode extrair trace_id, user_id, etc. para melhor observabilidade
	}

	return p.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: p.streamName,
		ID:     "*",
		Values: values,
	}).Err()
}

// ProcessDeadLetterQueue reprocessa eventos que falharam múltiplas vezes
func (p *OutboxProcessor) ProcessDeadLetterQueue(ctx context.Context, maxRetry int) {
	var failedEvents []OutboxEvent
	p.db.Where("processed_at IS NOT NULL AND retry_count >= max_retries").
		Find(&failedEvents)

	for _, event := range failedEvents {
		if event.RetryCount < maxRetry {
			// Reset para reprocessamento manual
			p.db.Model(&event).Updates(map[string]interface{}{
				"processed_at":  nil,
				"processing_at": nil,
				"retry_count":   event.RetryCount + 1,
			})
		}
	}
}
