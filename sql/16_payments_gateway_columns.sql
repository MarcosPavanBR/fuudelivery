-- ============================================================================
-- sql/16_payments_gateway_columns.sql
-- FUUDELIVERY — Colunas multi-gateway na tabela payments
-- ============================================================================
-- Adiciona colunas necessárias para suportar múltiplos gateways de pagamento,
-- pré-autorização de cartão, PIN de verificação, e split automático.
--
-- Colunas adicionadas:
--   - gateway: qual gateway processou (pagarme, asaas, etc.)
--   - gateway_transaction_id: ID da transação no gateway externo
--   - payment_method: método (pix, credit_card, debit_card)
--   - idempotency_key: chave de idempotência (UUID v4, único)
--   - authorized_at: timestamp da pré-autorização (cartão)
--   - captured_at: timestamp da captura
--   - voided_at: timestamp do cancelamento
--   - refunded_at: timestamp do estorno
--   - refund_amount: valor estornado em centavos
--   - split_applied: se split foi processado
--   - pin_hash: SHA-256 do PIN de 4 dígitos
--   - pin_expires_at: expiração do PIN (TTL 30min)
--   - pin_attempts: tentativas de PIN (máx 3)
--   - card_brand: bandeira do cartão
--   - card_last4: últimos 4 dígitos
--   - installments: número de parcelas
--
-- IDEMPOTENTE: pode rodar quantas vezes quiser.
-- ============================================================================

-- Colunas novas (ADD COLUMN IF NOT EXISTS é idempotente)
ALTER TABLE payments ADD COLUMN IF NOT EXISTS gateway VARCHAR(20) DEFAULT 'abacatepay';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS gateway_transaction_id VARCHAR(128);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20) DEFAULT 'pix';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(64);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS authorized_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS captured_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS refunded_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS refund_amount INTEGER DEFAULT 0;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS split_applied BOOLEAN DEFAULT false;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(64);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_expires_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_attempts INTEGER DEFAULT 0;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS card_brand VARCHAR(20);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS card_last4 VARCHAR(4);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS installments INTEGER DEFAULT 1;

-- Índices para buscas frequentes
CREATE INDEX IF NOT EXISTS idx_payments_gateway
    ON payments (gateway, gateway_transaction_id)
    WHERE gateway_transaction_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_payments_idempotency
    ON payments (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_payments_method
    ON payments (payment_method);

CREATE INDEX IF NOT EXISTS idx_payments_split_pending
    ON payments (split_applied)
    WHERE split_applied = false AND status IN ('paid', 'captured');

-- Unique constraint para idempotência (impede duplicatas)
CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_idempotency
    ON payments (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Constraints (idempotent com IF NOT EXISTS via DO block)
DO $$
BEGIN
    -- Gateway constraint
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_payments_gateway' AND conrelid = 'payments'::regclass
    ) THEN
        ALTER TABLE payments ADD CONSTRAINT chk_payments_gateway
            CHECK (gateway IN ('abacatepay', 'pagarme', 'asaas', 'mercadopago'));
    END IF;

    -- Payment method constraint
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_payments_method' AND conrelid = 'payments'::regclass
    ) THEN
        ALTER TABLE payments ADD CONSTRAINT chk_payments_method
            CHECK (payment_method IN ('pix', 'credit_card', 'debit_card'));
    END IF;
END
$$;

-- Comentários nas colunas
COMMENT ON COLUMN payments.gateway IS
    'Gateway que processou o pagamento: abacatepay, pagarme, asaas, mercadopago';
COMMENT ON COLUMN payments.gateway_transaction_id IS
    'ID da transação no gateway externo (para reconcile, refund e auditoria)';
COMMENT ON COLUMN payments.payment_method IS
    'Método de pagamento: pix, credit_card, debit_card';
COMMENT ON COLUMN payments.idempotency_key IS
    'Chave de idempotência (UUID v4). Impede transações duplicadas.';
COMMENT ON COLUMN payments.authorized_at IS
    'Timestamp da pré-autorização (cartão). NULL para PIX e débito.';
COMMENT ON COLUMN payments.captured_at IS
    'Timestamp da captura efetiva. Para PIX = created_at.';
COMMENT ON COLUMN payments.voided_at IS
    'Timestamp do cancelamento (void) da pré-autorização.';
COMMENT ON COLUMN payments.refunded_at IS
    'Timestamp do estorno (refund).';
COMMENT ON COLUMN payments.refund_amount IS
    'Valor estornado em centavos. 0 = não estornado.';
COMMENT ON COLUMN payments.split_applied IS
    'Se true, o split foi processado pelo gateway.';
COMMENT ON COLUMN payments.pin_hash IS
    'SHA-256 do PIN de 4 dígitos para confirmação de entrega. NUNCA armazenar em plaintext.';
COMMENT ON COLUMN payments.pin_expires_at IS
    'Data de expiração do PIN (TTL 30 minutos).';
COMMENT ON COLUMN payments.pin_attempts IS
    'Número de tentativas de validação do PIN (máximo 3).';
COMMENT ON COLUMN payments.card_brand IS
    'Bandeira do cartão: visa, mastercard, elo, amex.';
COMMENT ON COLUMN payments.card_last4 IS
    'Últimos 4 dígitos do cartão (auditoria).';
COMMENT ON COLUMN payments.installments IS
    'Número de parcelas (1 = à vista). Ignorado para PIX e débito.';

-- RLS: habilitar se não estiver habilitado
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_tables
        WHERE tablename = 'payments'
        AND rowsecurity = false
    ) THEN
        EXECUTE 'ALTER TABLE payments ENABLE ROW LEVEL SECURITY;';
    END IF;
END
$$;

INSERT INTO schema_migrations (version, description)
VALUES ('16_payments_gateway_columns', 'Adiciona colunas multi-gateway na tabela payments (split, pre-auth, PIN, cartão)')
ON CONFLICT (version) DO NOTHING;
