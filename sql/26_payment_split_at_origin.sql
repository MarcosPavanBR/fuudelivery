-- ============================================================================
-- sql/26_payment_split_at_origin.sql
-- FUUDELIVERY — Marca de pagamento liquidado por split na origem
-- ============================================================================
-- PARA QUE SERVE
--
-- No modelo marketplace, a fatia da loja cai DIRETO na conta MP dela — não
-- passa pela carteira interna da plataforma. O settle precisa saber disso:
-- para um pagamento com split na origem, ele NÃO credita a carteira do
-- estabelecimento (a loja já recebeu direto), só cuida do que a plataforma
-- paga da própria comissão (entregador, cashback). Sem esta marca, o settle
-- creditaria a loja de novo — pagando duas vezes.
--
-- Default false: todo pagamento existente e todo pagamento no fluxo antigo
-- (custódia) continua exatamente como está. Só o caminho novo marca true.
--
-- IDEMPOTENTE: ADD COLUMN IF NOT EXISTS. A tabela payments é criada pelo
-- AutoMigrate do GORM; a guarda evita quebrar num banco onde ela ainda não
-- exista.
-- ============================================================================

DO $$
BEGIN
    IF to_regclass('public.payments') IS NULL THEN
        RAISE NOTICE 'payments ainda nao existe (AutoMigrate nao rodou) — pulando sql/26.';
    ELSE
        ALTER TABLE payments ADD COLUMN IF NOT EXISTS split_at_origin BOOLEAN NOT NULL DEFAULT false;
    END IF;
END $$;

COMMENT ON COLUMN payments.split_at_origin IS
    'true = a fatia da loja caiu direto na conta do gateway dela (marketplace); '
    'o settle NAO credita a carteira interna do estabelecimento para estes.';

INSERT INTO schema_migrations (version, description)
VALUES ('26_payment_split_at_origin', 'Coluna payments.split_at_origin para o settle nao creditar a carteira da loja no modelo marketplace')
ON CONFLICT DO NOTHING;
