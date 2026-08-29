package reaper

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// StreamReaper é responsável por recuperar mensagens pendentes em Redis Streams
type StreamReaper struct {
	client        *redis.Client
	streamName    string
	groupName     string
	consumerName  string
	maxIdleTime   time.Duration
	maxRetries    int
	checkInterval time.Duration
	done          chan struct{}
}

// PendingMessage representa uma mensagem pendente
type PendingMessage struct {
	ID         string
	Consumer   string
	IdleTime   time.Duration
	DeliveryCount int
}

// NewStreamReaper cria um novo reaper para um stream específico
func NewStreamReaper(
	client *redis.Client,
	streamName string,
	groupName string,
	consumerName string,
	maxIdleTime time.Duration,
	maxRetries int,
	checkInterval time.Duration,
) *StreamReaper {
	return &StreamReaper{
		client:        client,
		streamName:    streamName,
		groupName:     groupName,
		consumerName:  consumerName,
		maxIdleTime:   maxIdleTime,
		maxRetries:    maxRetries,
		checkInterval: checkInterval,
		done:          make(chan struct{}),
	}
}

// Start inicia o processo de reaper em background
func (r *StreamReaper) Start(ctx context.Context) {
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	log.Printf("[StreamReaper] Started for stream %s, group %s", r.streamName, r.groupName)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[StreamReaper] Stopped due to context cancellation")
			return
		case <-r.done:
			log.Printf("[StreamReaper] Stopped by signal")
			return
		case <-ticker.C:
			r.reapPendingMessages(ctx)
		}
	}
}

// Stop para o reaper
func (r *StreamReaper) Stop() {
	close(r.done)
}

// reapPendingMessages verifica e recupera mensagens pendentes
func (r *StreamReaper) reapPendingMessages(ctx context.Context) {
	// Verifica mensagens pendentes
	pending, err := r.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: r.streamName,
		Group:  r.groupName,
		Idle:   r.maxIdleTime,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()

	if err != nil {
		if err != redis.Nil {
			log.Printf("[StreamReaper] Error checking pending messages: %v", err)
		}
		return
	}

	if len(pending) == 0 {
		return
	}

	log.Printf("[StreamReaper] Found %d pending messages older than %v", len(pending), r.maxIdleTime)

	// Recupera detalhes das mensagens pendentes
	for _, msg := range pending {
		if msg.Idle < r.maxIdleTime {
			continue
		}

		// Verifica se excedeu número máximo de tentativas
		if msg.Count > int64(r.maxRetries) {
			log.Printf("[StreamReaper] Message %s exceeded max retries (%d), moving to DLQ", msg.ID, r.maxRetries)
			r.moveToDLQ(ctx, msg.ID)
			continue
		}

		// Tenta claimar a mensagem para este consumidor
		claimed, err := r.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   r.streamName,
			Group:    r.groupName,
			Consumer: r.consumerName,
			Messages: []string{msg.ID},
			MinIdle:  r.maxIdleTime,
		}).Result()

		if err != nil {
			log.Printf("[StreamReaper] Error claiming message %s: %v", msg.ID, err)
			continue
		}

		if len(claimed) > 0 {
			log.Printf("[StreamReaper] Claimed message %s from consumer %s", msg.ID, msg.Consumer)
			// A mensagem agora está disponível para este consumidor processar
			// O consumidor principal deve detectar e reprocessar
		}
	}
}

// moveToDLQ move uma mensagem para a Dead Letter Queue
func (r *StreamReaper) moveToDLQ(ctx context.Context, messageID string) {
	// Lê a mensagem original
	messages, err := r.client.XRange(ctx, r.streamName, messageID, messageID).Result()
	if err != nil {
		log.Printf("[StreamReaper] Error reading message %s: %v", messageID, err)
		return
	}

	if len(messages) == 0 {
		return
	}

	msg := messages[0]
	dlqStream := r.streamName + ":dlq"

	// Adiciona metadados de DLQ
	msg.Values["dlq_reason"] = "max_retries_exceeded"
	msg.Values["dlq_original_stream"] = r.streamName
	msg.Values["dlq_timestamp"] = time.Now().UTC().Format(time.RFC3339)

	// Adiciona à DLQ
	_, err = r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqStream,
		ID:     "*",
		Values: msg.Values,
	}).Result()

	if err != nil {
		log.Printf("[StreamReaper] Error moving message %s to DLQ: %v", messageID, err)
		return
	}

	// Remove do stream original
	err = r.client.XAck(ctx, r.streamName, r.groupName, messageID).Err()
	if err != nil {
		log.Printf("[StreamReaper] Error acknowledging message %s after DLQ: %v", messageID, err)
	}

	log.Printf("[StreamReaper] Moved message %s to DLQ", messageID)
}

// GetPendingCount retorna o número de mensagens pendentes
func (r *StreamReaper) GetPendingCount(ctx context.Context) (int64, error) {
	info, err := r.client.XInfoGroups(ctx, r.streamName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get stream info: %w", err)
	}

	var totalPending int64
	for _, group := range info {
		if group.Name == r.groupName {
			totalPending = group.Pending
			break
		}
	}

	return totalPending, nil
}

// GetStats retorna estatísticas do stream
func (r *StreamReaper) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Informações do grupo
	info, err := r.client.XInfoGroups(ctx, r.streamName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	for _, group := range info {
		if group.Name == r.groupName {
			stats["pending"] = group.Pending
			stats["lag"] = group.Lag
			stats["consumers"] = group.Consumers
			break
		}
	}

	// Tamanho do stream
	length, err := r.client.XLen(ctx, r.streamName).Result()
	if err == nil {
		stats["stream_length"] = length
	}

	// Mensagens na DLQ
	dlqLength, err := r.client.XLen(ctx, r.streamName+":dlq").Result()
	if err == nil {
		stats["dlq_length"] = dlqLength
	}

	return stats, nil
}
