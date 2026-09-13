-- ============================================================================
-- sql/25_recipient_oauth.sql
-- FUUDELIVERY — Split na origem: credenciais OAuth por recebedor
-- ============================================================================
-- PARA QUE SERVE
--
-- O modelo de marketplace faz cada estabelecimento conectar a PRÓPRIA conta do
-- gateway (Mercado Pago, via OAuth). A partir daí, a cobrança é criada com o
-- token do vendedor e a fatia dele cai DIRETO na conta dele — o dinheiro da
-- venda nunca passa pela conta da plataforma. Isto exige guardar, por
-- recebedor, o access_token e o refresh_token da conta conectada.
--
-- SEGURANÇA — os tokens ficam CIFRADOS (AES-256-GCM, pkg/secretbox) nas colunas
-- *_enc. Quem tem esses tokens cobra em nome do lojista, então NUNCA em texto
-- puro: nem aqui, nem no `metadata` JSONB (que é texto puro). A regra do
-- projeto (CLAUDE.md) já registra vazamento recorrente de credencial — este é o
-- pior caso possível.
--
-- IDEMPOTENTE: guarda de existência + ADD COLUMN IF NOT EXISTS. Roda N vezes.
-- ============================================================================

DO $$
BEGIN
    IF to_regclass('public.recipients') IS NULL THEN
        RAISE NOTICE 'recipients ainda nao existe (rode sql/14 antes) — pulando sql/25.';
    ELSE
        -- Colunas cifradas do OAuth. BYTEA porque o secretbox devolve
        -- nonce||ciphertext binário, não texto.
        ALTER TABLE recipients ADD COLUMN IF NOT EXISTS access_token_enc  BYTEA;
        ALTER TABLE recipients ADD COLUMN IF NOT EXISTS refresh_token_enc BYTEA;
        ALTER TABLE recipients ADD COLUMN IF NOT EXISTS token_expires_at  TIMESTAMPTZ;
        ALTER TABLE recipients ADD COLUMN IF NOT EXISTS mp_user_id        VARCHAR(64);

        -- Alinhar o CHECK de user_type. O script 14 original só previa
        -- 'restaurant' e 'delivery_man', mas TODO o resto do código de dinheiro
        -- (wallets, split_calculator, EstablishmentID) usa 'establishment'. Sem
        -- este alinhamento, gravar o recebedor do estabelecimento violaria o
        -- CHECK. Drop+add em vez de ALTER porque Postgres não tem
        -- "ALTER CONSTRAINT ... CHECK".
        ALTER TABLE recipients DROP CONSTRAINT IF EXISTS chk_recipients_user_type;
        ALTER TABLE recipients ADD  CONSTRAINT chk_recipients_user_type
            CHECK (user_type IN ('restaurant', 'delivery_man', 'establishment'));
    END IF;
END $$;

-- Índice para o job de refresh achar tokens perto de expirar sem varrer a
-- tabela inteira. Parcial: só recebedores ativos com token têm o que renovar.
DO $$
BEGIN
    IF to_regclass('public.recipients') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_recipients_token_expiry
            ON recipients (token_expires_at)
            WHERE status = 'active' AND token_expires_at IS NOT NULL;
    END IF;
END $$;

COMMENT ON COLUMN recipients.access_token_enc IS
    'access_token do OAuth do gateway, cifrado com AES-256-GCM (pkg/secretbox). Nunca em texto puro.';
COMMENT ON COLUMN recipients.refresh_token_enc IS
    'refresh_token do OAuth, cifrado. Usado pelo job de refresh antes de token_expires_at.';
COMMENT ON COLUMN recipients.mp_user_id IS
    'collector_id / user_id da conta conectada no Mercado Pago (identifica o vendedor no split).';

INSERT INTO schema_migrations (version, description)
VALUES ('25_recipient_oauth', 'Colunas cifradas de OAuth em recipients (access/refresh token, expiry, mp_user_id) + user_type establishment, para split na origem')
ON CONFLICT DO NOTHING;
