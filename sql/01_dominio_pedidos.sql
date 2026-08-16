-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 01 — Domínio de pedidos (orders_api)
-- ============================================================================
-- DIAGNÓSTICO:
--   As tabelas deste domínio (categories, products, additionals, orders,
--   order_items, coupons, loyalty_points, reviews, batches, delivery) JÁ
--   estão em Postgres — o próprio orders_api roda GORM AutoMigrate nelas
--   toda vez que sobe (veja ConnectPostgresDatabase em
--   Backend/orders_api/app/models/database.go). Não recriamos essas tabelas
--   aqui para não competir com o AutoMigrate; este script só cobre o que
--   AINDA está fora do Postgres nesse domínio: os push tokens de
--   notificação, hoje gravados direto numa collection Mongo
--   ("push_tokens") por Backend/orders_api/app/handlers/notifications.go —
--   sem schema, sem FK para o usuário, sem índice.
-- ============================================================================

CREATE TABLE IF NOT EXISTS push_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,           -- id do client/user (auth_api)
    user_type   VARCHAR(20) NOT NULL DEFAULT 'client',  -- client | establishment | delivery_man
    push_token  TEXT NOT NULL,
    platform    VARCHAR(20),               -- ios | android | web (opcional, hoje não é gravado)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, user_type)
);

COMMENT ON TABLE push_tokens IS
    'Tokens de push notification. Antes vivia numa collection MongoDB solta, '
    'sem relação formal com o usuário. Consolidado aqui com FK lógica para '
    'users/clients/delivery_men (não criamos FK física porque user_id pode '
    'apontar para três tabelas diferentes dependendo de user_type).';

CREATE INDEX IF NOT EXISTS idx_push_tokens_user ON push_tokens (user_id, user_type);

-- Trigger simples para manter updated_at em dia (reaproveitado em outros
-- scripts deste pacote — função criada apenas uma vez).
CREATE OR REPLACE FUNCTION fn_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_push_tokens_updated_at ON push_tokens;
CREATE TRIGGER trg_push_tokens_updated_at
    BEFORE UPDATE ON push_tokens
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

INSERT INTO schema_migrations (version, description)
VALUES ('01_dominio_pedidos', 'Cria push_tokens em Postgres (antes MongoDB); demais tabelas do domínio de pedidos já são geridas via GORM AutoMigrate')
ON CONFLICT (version) DO NOTHING;

-- ----------------------------------------------------------------------------
-- IMPORTANTE — isto é só o banco. O código de
-- Backend/orders_api/app/handlers/notifications.go ainda escreve em
-- models.MongoDabase.Collection("push_tokens"). Sem trocar essas linhas
-- para gravar em Postgres (via GORM, como o resto do orders_api já faz),
-- esta tabela fica vazia. Ver skills/fuudelivery-banco-unico/SKILL.md,
-- seção "Ordem de corte" para o passo a passo de troca sem downtime.
-- ----------------------------------------------------------------------------
