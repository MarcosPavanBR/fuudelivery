-- ============================================================================
-- sql/14_recipients.sql
-- FUUDELIVERY — Recebedores multi-gateway
-- ============================================================================
-- Cada participante (restaurante/entregador) pode ter sub-contas em vários
-- gateways para receber splits de pagamento automaticamente.
--
-- Gateways suportados: pagarme, asaas, abacatepay, mercadopago
--
-- SEGURANÇA:
--   - RLS habilitado com policy backend_full_access (padrão do script 06)
--   - Constraints CHECK validam valores de enum
--   - UNIQUE em (user_type, user_id, gateway) impede duplicatas
--
-- IDEMPOTENTE: pode rodar quantas vezes quiser.
-- ============================================================================

CREATE TABLE IF NOT EXISTS recipients (
    id                   BIGSERIAL PRIMARY KEY,
    user_type            VARCHAR(20) NOT NULL,
    user_id              INTEGER NOT NULL,
    gateway              VARCHAR(20) NOT NULL,
    gateway_recipient_id VARCHAR(128) NOT NULL,
    status               VARCHAR(20) NOT NULL DEFAULT 'pending',
    bank_account_last4   VARCHAR(4),
    transfer_interval    VARCHAR(20) DEFAULT 'daily',
    transfer_day         INTEGER,
    metadata             JSONB DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_recipients_user_gateway UNIQUE (user_type, user_id, gateway),
    CONSTRAINT chk_recipients_user_type CHECK (user_type IN ('restaurant', 'delivery_man')),
    CONSTRAINT chk_recipients_gateway CHECK (gateway IN ('pagarme', 'asaas', 'abacatepay', 'mercadopago')),
    CONSTRAINT chk_recipients_status CHECK (status IN ('pending', 'active', 'blocked', 'kyc_pending', 'kyc_rejected')),
    CONSTRAINT chk_recipients_transfer CHECK (transfer_interval IN ('daily', 'weekly', 'monthly'))
);

CREATE INDEX idx_recipients_user ON recipients (user_type, user_id);
CREATE INDEX idx_recipients_gateway ON recipients (gateway, gateway_recipient_id);
CREATE INDEX idx_recipients_active ON recipients (status) WHERE status = 'active';

COMMENT ON TABLE recipients IS
    'Recebedores multi-gateway. Cada participante (restaurante/entregador) '
    'pode ter sub-contas em vários gateways. Gateway padrão: Pagar.me. '
    'Criado em 2026-08-27 para suportar split automático de pagamentos.';

COMMENT ON COLUMN recipients.user_type IS
    'Tipo do participante: restaurant (restaurante) ou delivery_man (entregador)';

COMMENT ON COLUMN recipients.gateway IS
    'Gateway ao qual o recebedor está vinculado: pagarme, asaas, abacatepay, mercadopago';

COMMENT ON COLUMN recipients.gateway_recipient_id IS
    'ID do recebedor no gateway externo (walletId, recipient_id, etc.)';

COMMENT ON COLUMN recipients.bank_account_last4 IS
    'Últimos 4 dígitos da conta bancária (auditoria, não expor em API pública)';

-- RLS
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL ON recipients FROM anon;';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL ON recipients FROM authenticated;';
    END IF;

    EXECUTE 'ALTER TABLE recipients ENABLE ROW LEVEL SECURITY;';
    EXECUTE 'ALTER TABLE recipients FORCE ROW LEVEL SECURITY;';

    EXECUTE 'DROP POLICY IF EXISTS backend_full_access ON recipients;';
    EXECUTE
        'CREATE POLICY backend_full_access ON recipients '
        'FOR ALL TO app_backend USING (true) WITH CHECK (true);';
END
$$;

GRANT SELECT, INSERT, UPDATE, DELETE ON recipients TO app_backend;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_backend;

INSERT INTO schema_migrations (version, description)
VALUES ('14_recipients', 'Cria tabela recipients para recebedores multi-gateway (split automático)')
ON CONFLICT (version) DO NOTHING;
