-- ============================================================================
-- sql/15_split_rules.sql
-- FUUDELIVERY — Regras de split por pagamento
-- ============================================================================
-- Cada linha representa uma porção do valor que deve ser transferida
-- para um recebedor. Atualizada via webhook quando o gateway confirma
-- o split.
--
-- Exemplo: Pedido R$ 50,00 com split 75/15/10
--   - Restaurante (75%): R$ 37,50 → split_rule amount=3750
--   - Entregador (15%):  R$  7,50 → split_rule amount=750
--   - Plataforma (10%):  R$  5,00 → split_rule amount=500 (fica na conta principal)
--
-- SEGURANÇA:
--   - UNIQUE em (payment_id, recipient_id) impede splits duplicados
--   - Constraints CHECK validam status e valores
--   - ON DELETE CASCADE: se o pagamento for deletado, os splits são removidos
--
-- IDEMPOTENTE: pode rodar quantas vezes quiser.
-- ============================================================================

CREATE TABLE IF NOT EXISTS payment_split_rules (
    id                BIGSERIAL PRIMARY KEY,
    payment_id        BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    recipient_id      BIGINT NOT NULL REFERENCES recipients(id),
    gateway           VARCHAR(20) NOT NULL,
    gateway_split_id  VARCHAR(128),
    percentage        DECIMAL(5,2),
    fixed_value       INTEGER,
    amount            INTEGER NOT NULL,
    liable            BOOLEAN NOT NULL DEFAULT false,
    chargeback_responsible BOOLEAN NOT NULL DEFAULT false,
    status            VARCHAR(20) NOT NULL DEFAULT 'pending',
    failure_reason    TEXT,
    paid_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_split_payment_recipient UNIQUE (payment_id, recipient_id),
    CONSTRAINT chk_split_gateway CHECK (gateway IN ('pagarme', 'asaas', 'abacatepay', 'mercadopago')),
    CONSTRAINT chk_split_status CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'blocked')),
    CONSTRAINT chk_split_amount CHECK (amount > 0),
    CONSTRAINT chk_split_percentage CHECK (percentage IS NULL OR (percentage > 0 AND percentage <= 100)),
    CONSTRAINT chk_split_fixed_value CHECK (fixed_value IS NULL OR fixed_value > 0)
);

CREATE INDEX idx_split_payment ON payment_split_rules (payment_id);
CREATE INDEX idx_split_recipient ON payment_split_rules (recipient_id);
CREATE INDEX idx_split_pending ON payment_split_rules (status) WHERE status = 'pending';

COMMENT ON TABLE payment_split_rules IS
    'Regras de split por pagamento. Cada linha = uma porção do valor para um recebedor. '
    'Atualizada via webhook quando o gateway confirma o split. '
    'Criado em 2026-08-27 para suportar split automático multi-gateway.';

COMMENT ON COLUMN payment_split_rules.amount IS
    'Valor efetivo em centavos que será transferido para o recebedor';

COMMENT ON COLUMN payment_split_rules.liable IS
    'Se true, este recipient é responsável pelo MDR (taxa de interchange do cartão)';

COMMENT ON COLUMN payment_split_rules.chargeback_responsible IS
    'Se true, este recipient é responsável por chargebacks (contestação do cliente)';

-- RLS
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL ON payment_split_rules FROM anon;';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL ON payment_split_rules FROM authenticated;';
    END IF;

    EXECUTE 'ALTER TABLE payment_split_rules ENABLE ROW LEVEL SECURITY;';
    EXECUTE 'ALTER TABLE payment_split_rules FORCE ROW LEVEL SECURITY;';

    EXECUTE 'DROP POLICY IF EXISTS backend_full_access ON payment_split_rules;';
    EXECUTE
        'CREATE POLICY backend_full_access ON payment_split_rules '
        'FOR ALL TO app_backend USING (true) WITH CHECK (true);';
END
$$;

GRANT SELECT, INSERT, UPDATE, DELETE ON payment_split_rules TO app_backend;
GRANT USAGE, SELECT ON SEQUENCES IN SCHEMA public TO app_backend;

INSERT INTO schema_migrations (version, description)
VALUES ('15_split_rules', 'Cria tabela payment_split_rules para split automático de pagamentos')
ON CONFLICT (version) DO NOTHING;
