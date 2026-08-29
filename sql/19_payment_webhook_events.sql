-- ============================================================================
-- 19_payment_webhook_events.sql
-- Tabela de idempotência e auditoria de webhooks de pagamento.
--
-- Problema: gateways podem reenviar webhooks por timeout, retry ou instabilidade.
-- Sem idempotência, o sistema pode:
--   - Confirmar pagamento duas vezes
--   - Duplicar split financeiro
--   - Atualizar pedido indevidamente
--   - Creditar carteira duplicadamente
--   - Gerar inconsistência financeira
--
-- Solução: tabela centralizada para registrar TODOS os webhooks recebidos,
-- com constraint UNIQUE para garantir idempotência no nível do banco.
--
-- Uso:
--   1. Ao receber webhook, tentar INSERT nesta tabela primeiro
--   2. Se violar UNIQUE (gateway+external_event_id já existe), retornar 200 OK
--   3. Se INSERT sucesso, processar o webhook normalmente
-- ============================================================================

CREATE TABLE IF NOT EXISTS payment_webhook_events (
    -- ID único do registro (auditoria)
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Gateway de origem (pagarme, asaas, abacatepay, mercadopago)
    gateway TEXT NOT NULL,
    
    -- ID único do evento no gateway (para idempotência)
    external_event_id TEXT NOT NULL,
    
    -- ID do pagamento associado (pode ser null em alguns casos)
    payment_id TEXT,
    
    -- ID externo do pagamento no gateway
    external_payment_id TEXT,
    
    -- Status do pagamento reportado pelo webhook
    status TEXT NOT NULL,
    
    -- Tipo do evento (payment.created, payment.updated, refund.completed, etc.)
    event_type TEXT,
    
    -- Payload bruto do webhook (para auditoria e reprocessamento)
    raw_payload JSONB NOT NULL,
    
    -- Se o webhook foi processado com sucesso
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Timestamp do processamento
    processed_at TIMESTAMPTZ,
    
    -- Erro ocorrido durante processamento (se houver)
    error_message TEXT,
    
    -- Número de tentativas de processamento
    retry_count INTEGER NOT NULL DEFAULT 0,
    
    -- Timestamp de criação (quando o webhook foi recebido)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Última atualização
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índice único para garantir idempotência (gateway + evento)
-- Este é o coração da idempotência: impede processamento duplicado
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_webhook_events_gateway_event
    ON payment_webhook_events (gateway, external_event_id);

-- Índices para consultas frequentes
CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_payment_id
    ON payment_webhook_events (payment_id);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_external_payment_id
    ON payment_webhook_events (external_payment_id);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_created_at
    ON payment_webhook_events (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_status
    ON payment_webhook_events (status);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_processed
    ON payment_webhook_events (processed) WHERE processed = FALSE;

-- Trigger para atualizar updated_at automaticamente
CREATE OR REPLACE FUNCTION update_payment_webhook_events_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_payment_webhook_events_updated_at
    BEFORE UPDATE ON payment_webhook_events
    FOR EACH ROW
    EXECUTE FUNCTION update_payment_webhook_events_updated_at();

-- Comentários para documentação
COMMENT ON TABLE payment_webhook_events IS 'Registro de todos os webhooks de pagamento recebidos para idempotência e auditoria';
COMMENT ON COLUMN payment_webhook_events.gateway IS 'Gateway de origem: pagarme, asaas, abacatepay, mercadopago';
COMMENT ON COLUMN payment_webhook_events.external_event_id IS 'ID único do evento no gateway (usado para idempotência)';
COMMENT ON COLUMN payment_webhook_events.raw_payload IS 'Payload bruto do webhook para auditoria e possível reprocessamento';
COMMENT ON COLUMN payment_webhook_events.processed IS 'Indica se o webhook foi processado com sucesso';
COMMENT ON COLUMN payment_webhook_events.retry_count IS 'Número de tentativas de processamento (para debugging)';

-- Registro da migração
INSERT INTO schema_migrations (version, description)
VALUES ('19', 'payment_webhook_events_idempotency')
ON CONFLICT (version) DO NOTHING;
