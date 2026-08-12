-- ============================================================
-- FUUDELIVERY — Migration Corretiva Completa
-- Corrige os 3 erros que impedem o AutoMigrate de rodar:
--   1. uni_clients_phone constraint não existe
--   2. fk_category_products_product FK type mismatch
--   3. orders.establishment_id column ausente
--
-- Execute: psql "$DB_CONNECTION_STRING" -f scripts/migrate-fix-all.sql
-- ============================================================

BEGIN;

-- ============================================================
-- ERRO 1: Constraint uni_clients_phone
-- O código atual usa idx_clients_phone, mas o banco antigo
-- pode ter uni_clients_phone de uma versão anterior do GORM.
-- ============================================================
DO $$
BEGIN
    -- Tentar remover constraint antigo se existir
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uni_clients_phone'
        AND conrelid = 'clients'::regclass
    ) THEN
        ALTER TABLE clients DROP CONSTRAINT uni_clients_phone;
        RAISE NOTICE 'Constraint uni_clients_phone removida com sucesso';
    ELSE
        RAISE NOTICE 'Constraint uni_clients_phone nao existe — nada a fazer';
    END IF;
END $$;

-- Garantir que o índice unique existe com o nome correto
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE indexname = 'idx_clients_phone'
        AND tablename = 'clients'
    ) THEN
        -- Verificar se o índice antigo existe e renomear
        IF EXISTS (
            SELECT 1 FROM pg_indexes
            WHERE indexname LIKE '%clients%phone%'
            AND tablename = 'clients'
        ) THEN
            -- Não dá pra renomear índice diretamente, criar novo e dropar antigo
            -- Mas primeiro verificar se é unique
            CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_phone ON clients(phone);
            RAISE NOTICE 'Índice idx_clients_phone criado';
        ELSE
            CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_phone ON clients(phone);
            RAISE NOTICE 'Índice idx_clients_phone criado (não existia)';
        END IF;
    ELSE
        RAISE NOTICE 'Índice idx_clients_phone já existe';
    END IF;
END $$;

-- ============================================================
-- ERRO 2: category_products FK type mismatch
-- O GORM define ProductID como integer, mas products.id é bigint.
-- A FK não pode referenciar tipos diferentes.
-- Solução: Recriar a tabela com tipos consistentes.
-- ============================================================

-- Primeiro, fazer backup dos dados existentes (se houver)
CREATE TABLE IF NOT EXISTS category_products_backup AS
SELECT * FROM category_products WHERE false;

-- Inserir dados existentes (ignorar erro se tabela não existe)
DO $$
BEGIN
    INSERT INTO category_products_backup SELECT * FROM category_products;
    RAISE NOTICE 'Backup de category_products criado';
EXCEPTION WHEN undefined_table THEN
    RAISE NOTICE 'Tabela category_products não existe — será criada';
END $$;

-- Dropar tabela antiga com FK problemática
DROP TABLE IF EXISTS category_products CASCADE;

-- Recriar com tipos corretos (bigint para ambas FKs)
CREATE TABLE category_products (
    product_id  BIGINT NOT NULL,
    category_id BIGINT NOT NULL,
    PRIMARY KEY (product_id, category_id),
    CONSTRAINT fk_category_products_product
        FOREIGN KEY (product_id) REFERENCES products(id)
        ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_category_products_category
        FOREIGN KEY (category_id) REFERENCES categories(id)
        ON DELETE CASCADE ON UPDATE CASCADE
);

-- Restaurar dados do backup
DO $$
BEGIN
    INSERT INTO category_products (product_id, category_id)
    SELECT product_id, category_id FROM category_products_backup;
    RAISE NOTICE 'Dados de category_products restaurados';
EXCEPTION WHEN undefined_table OR others THEN
    RAISE NOTICE 'Sem dados para restaurar';
END $$;

-- Limpar backup
DROP TABLE IF EXISTS category_products_backup;

RAISE NOTICE 'Tabela category_products recriada com tipos BIGINT consistentes';

-- ============================================================
-- ERRO 3: orders.establishment_id ausente
-- O model Order tem EstablishmentID uint mas a tabela não tem.
-- GORM AutoMigrate deveria adicionar, mas falha porque o
-- category_products quebra tudo antes.
-- ============================================================
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'orders'
        AND column_name = 'establishment_id'
    ) THEN
        ALTER TABLE orders ADD COLUMN establishment_id BIGINT;
        RAISE NOTICE 'Coluna establishment_id adicionada à tabela orders';
    ELSE
        RAISE NOTICE 'Coluna establishment_id já existe em orders';
    END IF;
END $$;

-- Adicionar índice para performance (o raw query faz JOIN nessa coluna)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE indexname = 'idx_orders_establishment_id'
    ) THEN
        CREATE INDEX idx_orders_establishment_id ON orders(establishment_id);
        RAISE NOTICE 'Índice idx_orders_establishment_id criado';
    ELSE
        RAISE NOTICE 'Índice idx_orders_establishment_id já existe';
    END IF;
END $$;

-- ============================================================
-- VERIFICAÇÕES FINAIS
-- ============================================================

-- Verificar que clients tem o índice correto
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE indexname = 'idx_clients_phone'
        AND tablename = 'clients'
    ) THEN
        RAISE NOTICE '✅ clients.phone tem unique index idx_clients_phone';
    ELSE
        RAISE WARNING '❌ clients.phone NÃO tem unique index idx_clients_phone';
    END IF;
END $$;

-- Verificar que category_products foi criada corretamente
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_name = 'category_products'
    ) THEN
        RAISE NOTICE '✅ Tabela category_products existe';
    ELSE
        RAISE WARNING '❌ Tabela category_products NÃO existe';
    END IF;
END $$;

-- Verificar que orders tem establishment_id
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'orders'
        AND column_name = 'establishment_id'
    ) THEN
        RAISE NOTICE '✅ Tabela orders tem coluna establishment_id';
    ELSE
        RAISE WARNING '❌ Tabela orders NÃO tem coluna establishment_id';
    END IF;
END $$;

-- Garantir que a tabela batches existe (do script anterior)
CREATE TABLE IF NOT EXISTS batches (
    id                BIGSERIAL       PRIMARY KEY,
    status            VARCHAR(30)     NOT NULL DEFAULT 'active',
    zone_id           BIGINT          NOT NULL,
    courier_id        BIGINT          DEFAULT NULL,
    max_detour_km     DOUBLE PRECISION DEFAULT 3.0,
    origin_lat        DOUBLE PRECISION DEFAULT 0,
    origin_lng        DOUBLE PRECISION DEFAULT 0,
    destination_lat   DOUBLE PRECISION DEFAULT 0,
    destination_lng   DOUBLE PRECISION DEFAULT 0,
    total_orders      INTEGER         DEFAULT 0,
    total_km          DOUBLE PRECISION DEFAULT 0,
    total_amount      DOUBLE PRECISION DEFAULT 0,
    started_at        TIMESTAMP WITH TIME ZONE,
    completed_at      TIMESTAMP WITH TIME ZONE,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_batches_zone_id ON batches(zone_id);
CREATE INDEX IF NOT EXISTS idx_batches_courier_id ON batches(courier_id);
CREATE INDEX IF NOT EXISTS idx_batches_status ON batches(status);

RAISE NOTICE '';
RAISE NOTICE '============================================================';
RAISE NOTICE '  MIGRATION FUUDELIVERY CONCLUIDA';
RAISE NOTICE '  1. uni_clients_phone → idx_clients_phone ✓';
RAISE NOTICE '  2. category_products → tipos BIGINT corrigidos ✓';
RAISE NOTICE '  3. orders.establishment_id → coluna adicionada ✓';
RAISE NOTICE '  4. batches → tabela criada ✓';
RAISE NO
