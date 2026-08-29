package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// WebhookIdempotencyManager gerencia idempotência de webhooks de pagamento
// usando a tabela payment_webhook_events no PostgreSQL.
type WebhookIdempotencyManager struct {
	db *sql.DB
}

// NewWebhookIdempotencyManager cria uma nova instância do gerenciador
func NewWebhookIdempotencyManager(db *sql.DB) *WebhookIdempotencyManager {
	return &WebhookIdempotencyManager{db: db}
}

// WebhookEventRecord representa um registro de webhook recebido
type WebhookEventRecord struct {
	ID                uuid.UUID       `json:"id"`
	Gateway           string          `json:"gateway"`
	ExternalEventID   string          `json:"external_event_id"`
	PaymentID         sql.NullString  `json:"payment_id,omitempty"`
	ExternalPaymentID sql.NullString  `json:"external_payment_id,omitempty"`
	Status            string          `json:"status"`
	EventType         sql.NullString  `json:"event_type,omitempty"`
	RawPayload        json.RawMessage `json:"raw_payload"`
	Processed         bool            `json:"processed"`
	ProcessedAt       sql.NullTime    `json:"processed_at,omitempty"`
	ErrorMessage      sql.NullString  `json:"error_message,omitempty"`
	RetryCount        int             `json:"retry_count"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// CheckAndRecord verifica se um webhook já foi processado (idempotência)
// e registra o evento se for a primeira vez.
//
// Retorna:
//   - true, nil: webhook já processado anteriormente (retornar 200 OK sem reprocessar)
//   - false, nil: webhook novo, pode processar
//   - false, err: erro ao verificar/registrar
func (m *WebhookIdempotencyManager) CheckAndRecord(
	ctx context.Context,
	gateway string,
	externalEventID string,
	paymentID string,
	externalPaymentID string,
	status string,
	eventType string,
	rawPayload []byte,
) (bool, error) {
	// Tenta inserir o registro com ON CONFLICT DO NOTHING
	// Se o INSERT falhar (conflito UNIQUE), significa que já existe
	query := `
		INSERT INTO payment_webhook_events 
			(gateway, external_event_id, payment_id, external_payment_id, status, event_type, raw_payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (gateway, external_event_id) DO NOTHING
	`

	var paymentIDNull, externalPaymentIDNull sql.NullString
	if paymentID != "" {
		paymentIDNull = sql.NullString{String: paymentID, Valid: true}
	}
	if externalPaymentID != "" {
		externalPaymentIDNull = sql.NullString{String: externalPaymentID, Valid: true}
	}

	var eventTypeNull sql.NullString
	if eventType != "" {
		eventTypeNull = sql.NullString{String: eventType, Valid: true}
	}

	result, err := m.db.ExecContext(ctx, query,
		gateway,
		externalEventID,
		paymentIDNull,
		externalPaymentIDNull,
		status,
		eventTypeNull,
		rawPayload,
	)

	if err != nil {
		return false, fmt.Errorf("failed to record webhook event: %w", err)
	}

	// Verifica se o INSERT foi executado (rows affected = 1) ou ignorado (rows affected = 0)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Conflito UNIQUE: webhook já processado anteriormente
		log.Printf("[WEBHOOK_IDEMPOTENCY] Duplicate webhook detected: gateway=%s, event_id=%s", gateway, externalEventID)
		return true, nil
	}

	// INSERT sucesso: webhook novo, pode processar
	log.Printf("[WEBHOOK_IDEMPOTENCY] New webhook recorded: gateway=%s, event_id=%s", gateway, externalEventID)
	return false, nil
}

// MarkProcessed marca um webhook como processado com sucesso
func (m *WebhookIdempotencyManager) MarkProcessed(
	ctx context.Context,
	gateway string,
	externalEventID string,
) error {
	query := `
		UPDATE payment_webhook_events
		SET processed = TRUE, processed_at = NOW()
		WHERE gateway = $1 AND external_event_id = $2
	`

	_, err := m.db.ExecContext(ctx, query, gateway, externalEventID)
	if err != nil {
		return fmt.Errorf("failed to mark webhook as processed: %w", err)
	}

	return nil
}

// MarkError marca um webhook como processado com erro
func (m *WebhookIdempotencyManager) MarkError(
	ctx context.Context,
	gateway string,
	externalEventID string,
	errorMsg string,
) error {
	query := `
		UPDATE payment_webhook_events
		SET processed = FALSE, error_message = $3, retry_count = retry_count + 1
		WHERE gateway = $1 AND external_event_id = $2
	`

	_, err := m.db.ExecContext(ctx, query, gateway, externalEventID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark webhook error: %w", err)
	}

	return nil
}

// GetUnprocessed retorna webhooks não processados para reprocessamento manual
func (m *WebhookIdempotencyManager) GetUnprocessed(ctx context.Context, limit int) ([]WebhookEventRecord, error) {
	query := `
		SELECT id, gateway, external_event_id, payment_id, external_payment_id, 
		       status, event_type, raw_payload, processed, processed_at, 
		       error_message, retry_count, created_at, updated_at
		FROM payment_webhook_events
		WHERE processed = FALSE
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := m.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unprocessed webhooks: %w", err)
	}
	defer rows.Close()

	var records []WebhookEventRecord
	for rows.Next() {
		var r WebhookEventRecord
		err := rows.Scan(
			&r.ID,
			&r.Gateway,
			&r.ExternalEventID,
			&r.PaymentID,
			&r.ExternalPaymentID,
			&r.Status,
			&r.EventType,
			&r.RawPayload,
			&r.Processed,
			&r.ProcessedAt,
			&r.ErrorMessage,
			&r.RetryCount,
			&r.CreatedAt,
			&r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook record: %w", err)
		}
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating webhook records: %w", err)
	}

	return records, nil
}

// Reprocess tenta reprocessar um webhook específico
func (m *WebhookIdempotencyManager) Reprocess(
	ctx context.Context,
	gateway string,
	externalEventID string,
	processFn func(record WebhookEventRecord) error,
) error {
	// Busca o registro
	record, err := m.getByGatewayAndEventID(ctx, gateway, externalEventID)
	if err != nil {
		return err
	}

	// Executa função de processamento
	if err := processFn(record); err != nil {
		// Marca erro
		m.MarkError(ctx, gateway, externalEventID, err.Error())
		return err
	}

	// Marca como processado
	return m.MarkProcessed(ctx, gateway, externalEventID)
}

// getByGatewayAndEventID busca um registro específico
func (m *WebhookIdempotencyManager) getByGatewayAndEventID(
	ctx context.Context,
	gateway string,
	externalEventID string,
) (WebhookEventRecord, error) {
	query := `
		SELECT id, gateway, external_event_id, payment_id, external_payment_id, 
		       status, event_type, raw_payload, processed, processed_at, 
		       error_message, retry_count, created_at, updated_at
		FROM payment_webhook_events
		WHERE gateway = $1 AND external_event_id = $2
	`

	var r WebhookEventRecord
	err := m.db.QueryRowContext(ctx, query, gateway, externalEventID).Scan(
		&r.ID,
		&r.Gateway,
		&r.ExternalEventID,
		&r.PaymentID,
		&r.ExternalPaymentID,
		&r.Status,
		&r.EventType,
		&r.RawPayload,
		&r.Processed,
		&r.ProcessedAt,
		&r.ErrorMessage,
		&r.RetryCount,
		&r.CreatedAt,
		&r.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return r, fmt.Errorf("webhook event not found: gateway=%s, event_id=%s", gateway, externalEventID)
	}
	if err != nil {
		return r, fmt.Errorf("failed to query webhook event: %w", err)
	}

	return r, nil
}

// CleanupOldRecords remove registros antigos (mais que N dias)
// Útil para manter a tabela limpa em produção
func (m *WebhookIdempotencyManager) CleanupOldRecords(ctx context.Context, olderThanDays int) (int64, error) {
	query := `
		DELETE FROM payment_webhook_events
		WHERE processed = TRUE 
		  AND created_at < NOW() - INTERVAL '%d days'
	`

	result, err := m.db.ExecContext(ctx, fmt.Sprintf(query, olderThanDays))
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old records: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	log.Printf("[WEBHOOK_CLEANUP] Removed %d old webhook records", rowsAffected)
	return rowsAffected, nil
}
