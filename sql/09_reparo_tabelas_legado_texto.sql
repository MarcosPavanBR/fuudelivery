-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 09 — Reparo: tabelas legadas com id TEXT bloqueando o schema correto
-- ============================================================================
-- PROBLEMA ENCONTRADO EM PRODUÇÃO (2026-08-23):
--
--   Um conjunto de tabelas (payments, withdrawals, notifications,
--   order_status_history, addresses, courier_profiles, ratings,
--   store_categories, stores) existia no banco com coluna id do tipo TEXT e
--   VAZIAS — resquício de um schema antigo da era MongoDB.
--
--   Consequência: o `CREATE TABLE IF NOT EXISTS` dos scripts 01–03 nunca criava
--   o schema correto (a tabela já "existia"), e o AutoMigrate do GORM não altera
--   o tipo de uma coluna já existente. Resultado: INSERT com id serial falhava
--   com SQLSTATE 23502/22P04 — a criação de pagamento em produção estava
--   QUEBRADA sem ninguém perceber. O bug só apareceu quando o ETL tentou
--   importar o histórico do Atlas.
--
--   Este script é IDEMPOTENTE: só renomeia a tabela se ela ainda tiver id TEXT
--   e estiver VAZIA. Se tiver dados, NÃO TOCA — exige revisão humana (seria
--   preciso decidir o que fazer com os ids em texto antes de migrar).
--
-- COMO RODAR:
--   psql "$DB_CONNECTION_STRING" -f sql/09_reparo_tabelas_legado_texto.sql
--
-- Depois de rodar, execute novamente os scripts 01–03 (todos são IF NOT EXISTS)
-- para recriar as tabelas com BIGSERIAL, ou simplesmente reinicie o monolito
-- (o AutoMigrate recria as que faltarem).
-- ============================================================================

DO $$
DECLARE
    t         RECORD;
    val_count BIGINT;
BEGIN
    FOR t IN
        SELECT c.table_name
        FROM information_schema.columns c
        JOIN information_schema.tables tb
          ON tb.table_name = c.table_name AND tb.table_schema = 'public'
        WHERE c.table_schema = 'public'
          AND c.column_name = 'id'
          AND c.data_type IN ('text', 'character varying')
          AND c.table_name IN (
              'payments', 'withdrawals', 'notifications', 'order_status_history',
              'addresses', 'courier_profiles', 'ratings', 'store_categories', 'stores'
          )
    LOOP
        -- Só mexe se estiver VAZIA (segurança: nunca descartamos dado).
        EXECUTE format('SELECT count(*) FROM %I', t.table_name) INTO val_count;
        IF val_count = 0 THEN
            EXECUTE format(
                'ALTER TABLE %I RENAME TO %I',
                t.table_name,
                t.table_name || '_legacy_textid_backup'
            );
            RAISE NOTICE 'Tabela % renomeada para %_legacy_textid_backup (estava vazia com id TEXT)',
                t.table_name, t.table_name;
        ELSE
            RAISE WARNING 'Tabela % tem id TEXT mas NÃO está vazia (%) linhas — revisão manual necessária',
                t.table_name, val_count;
        END IF;
    END LOOP;
END $$;
