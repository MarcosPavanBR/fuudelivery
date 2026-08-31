package outbox

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// OutboxEvent representa eventos pendentes de publicação
// Padrão Transactional Outbox para garantir consistência entre DB e Filas
type OutboxEvent struct {
	ID            string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	AggregateType string     `gorm:"index;not null"` // Ex: "order", "payment"
	AggregateID   string     `gorm:"index;not null"` // Ex: OrderID, PaymentID
	EventType     string     `gorm:"index;not null"` // Ex: "order.created", "payment.confirmed"
	Payload       string     `gorm:"type:jsonb;not null"`
	Metadata      string     `gorm:"type:jsonb"`
	CreatedAt     time.Time  `gorm:"autoCreateTime;index"`
	ProcessedAt   *time.Time `gorm:"index"`
	ProcessingAt  *time.Time
	RetryCount    int    `gorm:"default:0"`
	MaxRetries    int    `gorm:"default:3"`
	Error         string `gorm:"type:text"`
}

// SaveInTransaction salva entidade e evento na MESMA transação ACID
// Garante que o evento só existe se a entidade principal foi salva
func SaveInTransaction[T any](tx *gorm.DB, aggregateType string, aggregate *T, eventType string) error {
	// Serializa payload
	payloadBytes, err := json.Marshal(aggregate)
	if err != nil {
		return err
	}

	var aggregateID string
	// Tenta extrair ID da entidade (convenção comum)
	type IDGetter interface {
		GetID() string
	}
	if idGetter, ok := interface{}(aggregate).(IDGetter); ok {
		aggregateID = idGetter.GetID()
	} else {
		// Fallback: usa timestamp + random como ID
		aggregateID = time.Now().Format("20060102150405")
	}

	event := OutboxEvent{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       string(payloadBytes),
		CreatedAt:     time.Now(),
	}

	return tx.Create(&event).Error
}

// SaveWithMetadata salva evento com metadados adicionais (trace_id, user_id, etc.)
func SaveWithMetadata(tx *gorm.DB, aggregateType, aggregateID, eventType string, payload interface{}, metadata map[string]interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	event := OutboxEvent{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       string(payloadBytes),
		Metadata:      string(metadataBytes),
		CreatedAt:     time.Now(),
	}

	return tx.Create(&event).Error
}

// MarkAsProcessed marca evento como processado com sucesso
func (e *OutboxEvent) MarkAsProcessed(tx *gorm.DB) error {
	now := time.Now()
	e.ProcessedAt = &now
	return tx.Save(e).Error
}

// MarkAsFailed marca evento como falho e incrementa retry count
func (e *OutboxEvent) MarkAsFailed(tx *gorm.DB, errMsg string) error {
	e.RetryCount++
	e.Error = errMsg
	if e.RetryCount >= e.MaxRetries {
		now := time.Now()
		e.ProcessedAt = &now // Move para DLQ após max retries
	}
	return tx.Save(e).Error
}

// GetUnprocessedEvents retorna eventos pendentes de processamento
func GetUnprocessedEvents(db *gorm.DB, limit int) ([]OutboxEvent, error) {
	var events []OutboxEvent
	err := db.Where("processed_at IS NULL AND processing_at IS NULL").
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// ClaimForProcessing reserva evento para processamento (evita duplicidade em múltiplos workers)
func (e *OutboxEvent) ClaimForProcessing(tx *gorm.DB) error {
	now := time.Now()
	e.ProcessingAt = &now
	result := tx.Model(&OutboxEvent{}).
		Where("id = ? AND processed_at IS NULL AND processing_at IS NULL", e.ID).
		Updates(map[string]interface{}{"processing_at": now})

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound // Outro worker já pegou
	}
	return nil
}
