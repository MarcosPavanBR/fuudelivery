-- ============================================================================
-- sql/28_repasse_mode.sql
-- FUUDELIVERY — Modo de recebimento por loja (split vs repasse)
-- ============================================================================
-- PARA QUE SERVE
--
-- O recebimento da loja pode ser de dois jeitos, escolhido POR LOJA:
--   - split   (padrão): a cobrança já retém a comissão da plataforma
--     (application_fee) no ato — a loja recebe a fatia dela e nada deve.
--   - repasse (plano B): a cobrança vai com application_fee=0 — a loja recebe
--     100%% e passa a DEVER frete + comissão (uma linha em establishment_debts,
--     sql/27), que a plataforma cobra depois.
--
-- recipients.payment_mode guarda essa escolha por loja. payments.repasse marca
-- o pagamento que nasceu em repasse, para o settle saber que tem que CRIAR a
-- dívida (além de não creditar a carteira, que o split_at_origin já garante).
--
-- Defaults preservam o mundo atual: payment_mode='split', repasse=false.
--
-- IDEMPOTENTE: ADD COLUMN IF NOT EXISTS + CHECK guardado. Roda N vezes.
-- ============================================================================

DO $$
BEGIN
    IF to_regclass('public.recipients') IS NULL THEN
        RAISE NOTICE 'recipients ainda nao existe (rode sql/14) — pulando parte do sql/28.';
    ELSE
        ALTER TABLE recipients ADD COLUMN IF NOT EXISTS payment_mode VARCHAR(20) NOT NULL DEFAULT 'split';
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_recipients_payment_mode') THEN
            ALTER TABLE recipients ADD CONSTRAINT chk_recipients_payment_mode
                CHECK (payment_mode IN ('split', 'repasse'));
        END IF;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public.payments') IS NULL THEN
        RAISE NOTICE 'payments ainda nao existe (AutoMigrate nao rodou) — pulando parte do sql/28.';
    ELSE
        ALTER TABLE payments ADD COLUMN IF NOT EXISTS repasse BOOLEAN NOT NULL DEFAULT false;
    END IF;
END $$;

COMMENT ON COLUMN recipients.payment_mode IS
    'split (padrao): comissao retida no ato via application_fee. repasse: loja '
    'recebe 100%% e deve frete+comissao (establishment_debts).';
COMMENT ON COLUMN payments.repasse IS
    'true = pagamento em modo repasse; o settle cria a divida da loja (frete + comissao).';

INSERT INTO schema_migrations (version, description)
VALUES ('28_repasse_mode', 'recipients.payment_mode (split|repasse) e payments.repasse para o modo de recebimento por loja')
ON CONFLICT DO NOTHING;
