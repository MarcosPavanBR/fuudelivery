-- ============================================================================
-- FUUDELIVERY — Dead-letter queue persistente (dispatch engine)
-- 18 — Tabela unmatched_orders (pedidos que não encontraram entregador)
-- ============================================================================
-- POR QUE ESTE SCRIPT EXISTE:
--   A DLQ do motor de matching vivia só na memória. Se o dyno do Render dorme
--   ou o processo reinicia, todos os pedidos não casados eram perdidos — o
--   matching ficava cego até cada entregador reenviar GPS.
--
--   Com esta tabela, a DLQ sobrevive restarts. O PostgresDLQStore (Go) faz
--   INSERT/SELECT/DELETE com a mesma interface da DLQStore in-memory, então
--   o MatchingEngine não percebe a troca.
--
-- SEGURANÇA:
--   - Trigger de auditoria (fn_audit_trigger do script 05) registra toda
--     INSERT/UPDATE/DELETE na audit_log.
--   - RLS habilitado com policy backend_full_access (padrão do script 06).
--   - coluna metadata é JSONB para dados extras (coords, zone, etc.)
--
-- IDEMPOTENTE: pode rodar quantas vezes quiser.
-- ============================================================================

CREATE TABLE IF NOT EXISTS unmatched_orders (
    id               BIGSERIAL PRIMARY KEY,
    order_id         VARCHAR(64) NOT NULL,
    establishment_lat DOUBLE PRECISION NOT NULL,
    establishment_lng DOUBLE PRECISION NOT NULL,
    zone_id          INT NOT NULL DEFAULT 0,
    created_at       BIGINT NOT NULL,        -- UnixMilli
    retry_count      INT NOT NULL DEFAULT 0, -- max 3 (aplicado no backend)
    last_attempt_at  BIGINT NOT NULL,        -- UnixMilli
    metadata         JSONB,                  -- dados extras (opcional)
    created_tz       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Índices para as queries do PostgresDLQStore:
--   - PopNext: WHERE retry_count < 3 AND last_attempt_at < $cutoff ORDER BY created_at
--   - Len: COUNT(*)
--   - Cleanup: WHERE created_tz < $age
CREATE INDEX IF NOT EXISTS idx_unmatched_orders_retry
    ON unmatched_orders (retry_count, last_attempt_at);
CREATE INDEX IF NOT EXISTS idx_unmatched_orders_created
    ON unmatched_orders (created_tz);
CREATE INDEX IF NOT EXISTS idx_unmatched_orders_order_id
    ON unmatched_orders (order_id);

COMMENT ON TABLE unmatched_orders IS
    'Dead-letter queue persistente do dispatch engine. Pedidos que não '
    'encontraram entregador são armazenados aqui e reprocessados a cada 30s. '
    'Máximo de 3 retries por pedido. Sobrevive restarts do processo.';

-- ---------------------------------------------------------------------------
-- Auditoria
-- ---------------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_audit_unmatched_orders ON unmatched_orders;
CREATE TRIGGER trg_audit_unmatched_orders
    AFTER INSERT OR UPDATE OR DELETE ON unmatched_orders
    FOR EACH ROW EXECUTE FUNCTION fn_audit_trigger();

-- ---------------------------------------------------------------------------
-- RLS: padrão backend_full_access
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL ON unmatched_orders FROM anon;';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL ON unmatched_orders FROM authenticated;';
    END IF;

    EXECUTE 'ALTER TABLE unmatched_orders ENABLE ROW LEVEL SECURITY;';
    EXECUTE 'ALTER TABLE unmatched_orders FORCE ROW LEVEL SECURITY;';

    EXECUTE 'DROP POLICY IF EXISTS backend_full_access ON unmatched_orders;';
    EXECUTE
        'CREATE POLICY backend_full_access ON unmatched_orders '
        'FOR ALL TO app_backend USING (true) WITH CHECK (true);';
END
$$;

GRANT SELECT, INSERT, UPDATE, DELETE ON unmatched_orders TO app_backend;
GRANT USAGE, SELECT ON SEQUENCES IN SCHEMA public TO app_backend;

GRANT SELECT, INSERT, UPDATE, DELETE ON unmatched_orders TO service_role;

INSERT INTO schema_migrations (version, description)
VALUES ('18_unmatched_orders', 'Cria unmatched_orders (DLQ persistente do dispatch engine), com auditoria e RLS app_backend')
ON CONFLICT (version) DO NOTHING;
