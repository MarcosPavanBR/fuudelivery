-- Migration 19: Outbox Pattern para Garantia de Entrega de Eventos
-- Data: 2026-08-30
-- Descrição: Implementa padrão Transactional Outbox para consistência entre DB e filas

-- Tabela outbox_events armazena eventos pendentes de publicação
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT NOT NULL,        -- Ex: 'order', 'payment', 'delivery'
    aggregate_id TEXT NOT NULL,          -- Ex: order_id, payment_id
    event_type TEXT NOT NULL,            -- Ex: 'order.created', 'payment.confirmed'
    payload JSONB NOT NULL,              -- Dados do evento em JSON
    metadata JSONB,                      -- Metadata adicional (trace_id, user_id, etc.)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,            -- Quando foi processado/publicado
    processing_at TIMESTAMPTZ,           -- Quando começou a ser processado (lock)
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    error TEXT,                          -- Último erro ocorrido
    
    -- Índices para performance
    CONSTRAINT chk_retry_count CHECK (retry_count >= 0 AND max_retries > 0)
);

-- Índices para consultas eficientes
CREATE INDEX idx_outbox_events_unprocessed 
ON outbox_events (created_at ASC) 
WHERE processed_at IS NULL AND processing_at IS NULL;

CREATE INDEX idx_outbox_events_aggregate 
ON outbox_events (aggregate_type, aggregate_id);

CREATE INDEX idx_outbox_events_event_type 
ON outbox_events (event_type);

CREATE INDEX idx_outbox_events_processed 
ON outbox_events (processed_at DESC);

-- Comentário na tabela
COMMENT ON TABLE outbox_events IS 
'Padrão Transactional Outbox: Garante que eventos sejam publicados apenas após transação principal commitar com sucesso';

COMMENT ON COLUMN outbox_events.aggregate_type IS 
'Tipo da entidade agregada (ex: order, payment, delivery)';

COMMENT ON COLUMN outbox_events.aggregate_id IS 
'ID da entidade agregada';

COMMENT ON COLUMN outbox_events.event_type IS 
'Tipo do evento de domínio (ex: order.created, payment.confirmed)';

COMMENT ON COLUMN outbox_events.payload IS 
'Dados completos do evento em formato JSON';

COMMENT ON COLUMN outbox_events.metadata IS 
'Metadata adicional: trace_id, user_id, source, version';

COMMENT ON COLUMN outbox_events.processing_at IS 
'Timestamp de quando o evento começou a ser processado (usado como lock)';

-- Função para limpar eventos antigos (manter últimos 30 dias)
CREATE OR REPLACE FUNCTION cleanup_old_outbox_events(retention_days INTEGER DEFAULT 30)
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM outbox_events
    WHERE processed_at IS NOT NULL
      AND created_at < NOW() - (retention_days || ' days')::INTERVAL;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- View para monitoramento de eventos pendentes
CREATE OR REPLACE VIEW v_outbox_pending AS
SELECT 
    id,
    aggregate_type,
    aggregate_id,
    event_type,
    created_at,
    retry_count,
    max_retries,
    EXTRACT(EPOCH FROM (NOW() - created_at)) AS age_seconds,
    CASE 
        WHEN retry_count >= max_retries THEN 'FAILED'
        WHEN processing_at IS NOT NULL THEN 'PROCESSING'
        ELSE 'PENDING'
    END AS status
FROM outbox_events
WHERE processed_at IS NULL
ORDER BY created_at ASC;

-- View para estatísticas de outbox
CREATE OR REPLACE VIEW v_outbox_stats AS
SELECT 
    event_type,
    COUNT(*) FILTER (WHERE processed_at IS NULL AND processing_at IS NULL) AS pending_count,
    COUNT(*) FILTER (WHERE processing_at IS NOT NULL) AS processing_count,
    COUNT(*) FILTER (WHERE processed_at IS NOT NULL AND created_at >= NOW() - INTERVAL '1 hour') AS processed_last_hour,
    COUNT(*) FILTER (WHERE retry_count > 0) AS retry_count,
    COUNT(*) FILTER (WHERE retry_count >= max_retries) AS failed_count,
    AVG(EXTRACT(EPOCH FROM (processed_at - created_at))) FILTER (WHERE processed_at IS NOT NULL) AS avg_processing_time_seconds
FROM outbox_events
GROUP BY event_type;

-- Grants de segurança (ajustar conforme necessidade)
-- GRANT SELECT, INSERT, UPDATE ON outbox_events TO app_user;
-- GRANT SELECT ON v_outbox_pending TO monitoring_user;
-- GRANT SELECT ON v_outbox_stats TO monitoring_user;

-- Exemplo de uso em transação:
/*
BEGIN;

-- 1. Cria o pedido
INSERT INTO orders (id, customer_id, restaurant_id, total_amount, status, created_at)
VALUES ('order_123', 'cust_456', 'rest_789', 91.80, 'pending', NOW());

-- 2. Insere evento no outbox (MESMA transação)
INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload, metadata)
VALUES (
    'order',
    'order_123',
    'order.created',
    '{"order_id": "order_123", "customer_id": "cust_456", "total_amount": 91.80}',
    '{"trace_id": "abc123", "user_id": "cust_456", "source": "orders_api"}'
);

COMMIT;

-- Worker externo lê eventos pendentes e publica no Redis Stream
*/
