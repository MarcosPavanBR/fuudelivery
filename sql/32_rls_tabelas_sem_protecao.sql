-- ============================================================================
-- sql/32_rls_tabelas_sem_protecao.sql
-- FUUDELIVERY — RLS nas tabelas que nasceram depois do sql/06 sem ele
-- ============================================================================
-- O sql/06 fecha a API REST pública do Supabase (roles anon/authenticated) e
-- liga RLS com uma policy só para app_backend — mas numa LISTA fixa de
-- tabelas. As criadas depois que não repetiram o bloco ficaram de fora:
--
--   refresh_tokens        (sql/12) — sessões de login
--   delivery_region_fees  (sql/24) — preço do frete por região
--   establishment_debts   (sql/27) — o que cada loja deve à plataforma
--   outbox_events         (sql/19) — fila de eventos internos
--   business_hours        (AutoMigrate) — grade de horários
--   order_documents       (AutoMigrate) — pedidos
--
-- Mesmo padrão de sql/06 e sql/17. Tabela que não existe é pulada.
-- IDEMPOTENTE: roda N vezes.
-- ============================================================================

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'refresh_tokens', 'delivery_region_fees', 'establishment_debts',
        'outbox_events', 'business_hours', 'order_documents'
    ]
    LOOP
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = t) THEN
            IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
                EXECUTE format('REVOKE ALL ON %I FROM anon;', t);
            END IF;
            IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
                EXECUTE format('REVOKE ALL ON %I FROM authenticated;', t);
            END IF;
            EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY;', t);
            EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY;', t);
            EXECUTE format('DROP POLICY IF EXISTS backend_full_access ON %I;', t);
            EXECUTE format(
                'CREATE POLICY backend_full_access ON %I '
                'FOR ALL TO app_backend USING (true) WITH CHECK (true);', t
            );
        END IF;
    END LOOP;
END
$$;
