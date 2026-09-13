-- ============================================================================
-- sql/27_establishment_debts.sql
-- FUUDELIVERY — Contas a receber do restaurante (modelo de repasse)
-- ============================================================================
-- PARA QUE SERVE
--
-- Plano B do split na origem: quando o restaurante recebe o valor do pedido
-- DIRETO (na conta/PIX dele), ele passa a DEVER à plataforma o frete (dinheiro
-- do entregador) + a comissão. Esta tabela é o razão de contas a receber: uma
-- linha por pedido, com o que a loja deve e se já pagou.
--
-- POR QUE ISTO EXIGE CUIDADO
--
-- O repasse troca risco regulatório (segurar dinheiro de terceiro) por risco de
-- CRÉDITO (a loja te dever). Se a loja não repassa, a plataforma perde a
-- comissão E ainda deve o entregador. Por isso a coluna status e os índices
-- existem para (a) somar o que cada loja deve em aberto e (b) travar novos
-- pedidos de quem passou do limite — a trava de crédito vive no código
-- (REPASSE_CREDIT_LIMIT_CENTS), este schema só a sustenta.
--
-- UNIQUE(order_id): uma dívida por pedido. Criar a dívida é idempotente — o
-- settle reprocessado não duplica o débito.
--
-- IDEMPOTENTE: CREATE TABLE / INDEX IF NOT EXISTS. Roda N vezes.
-- ============================================================================

CREATE TABLE IF NOT EXISTS establishment_debts (
    id                BIGSERIAL PRIMARY KEY,
    establishment_id  BIGINT NOT NULL,
    order_id          VARCHAR(64) NOT NULL,
    payment_id        BIGINT,
    delivery_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,  -- frete (dinheiro do entregador)
    commission_amount NUMERIC(12,2) NOT NULL DEFAULT 0,  -- comissão da plataforma
    total_amount      NUMERIC(12,2) NOT NULL,            -- o que a loja deve = frete + comissão
    status            VARCHAR(20) NOT NULL DEFAULT 'open',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at        TIMESTAMPTZ,
    settled_by        VARCHAR(128),

    CONSTRAINT uq_establishment_debts_order UNIQUE (order_id),
    CONSTRAINT chk_establishment_debts_status CHECK (status IN ('open', 'paid', 'waived')),
    CONSTRAINT chk_establishment_debts_amounts CHECK (
        delivery_amount >= 0 AND commission_amount >= 0 AND total_amount >= 0
    )
);

-- Somar o que uma loja deve em aberto (a consulta da trava de crédito) sem
-- varrer a tabela inteira.
CREATE INDEX IF NOT EXISTS idx_establishment_debts_open
    ON establishment_debts (establishment_id)
    WHERE status = 'open';

CREATE INDEX IF NOT EXISTS idx_establishment_debts_est
    ON establishment_debts (establishment_id, status);

-- Robustez para o caso de a tabela já existir vinda do AutoMigrate do GORM
-- (que cria colunas mas NÃO cria CHECK constraints, e o CREATE TABLE IF NOT
-- EXISTS acima vira no-op). Mesmo padrão de sql/17, 21 e 22: garante as travas
-- independentemente de quem criou a tabela. O índice único vem do gorm tag
-- (uniqueIndex) ou do CREATE TABLE acima; aqui só cuidamos dos CHECKs, que o
-- AutoMigrate não cria.
DO $$
BEGIN
    IF to_regclass('public.establishment_debts') IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_establishment_debts_status') THEN
            ALTER TABLE establishment_debts ADD CONSTRAINT chk_establishment_debts_status
                CHECK (status IN ('open', 'paid', 'waived'));
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_establishment_debts_amounts') THEN
            ALTER TABLE establishment_debts ADD CONSTRAINT chk_establishment_debts_amounts
                CHECK (delivery_amount >= 0 AND commission_amount >= 0 AND total_amount >= 0);
        END IF;
        -- Unicidade de order_id: se nem constraint nem índice existir (tabela do
        -- AutoMigrate sem o gorm tag, por ex.), cria. Sem ela o ON CONFLICT da
        -- criação idempotente da dívida falha.
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_establishment_debts_order')
           AND NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'uq_establishment_debts_order') THEN
            ALTER TABLE establishment_debts ADD CONSTRAINT uq_establishment_debts_order UNIQUE (order_id);
        END IF;
    END IF;
END $$;

COMMENT ON TABLE establishment_debts IS
    'Contas a receber do restaurante no modelo de repasse: quando a loja recebe '
    'o pedido direto, ela deve frete + comissao a plataforma. Uma linha por pedido.';
COMMENT ON COLUMN establishment_debts.total_amount IS
    'O que a loja deve = delivery_amount + commission_amount. A trava de credito '
    'soma isto por loja com status=open.';

INSERT INTO schema_migrations (version, description)
VALUES ('27_establishment_debts', 'Contas a receber do restaurante (modelo de repasse) com trava de credito por loja')
ON CONFLICT DO NOTHING;
