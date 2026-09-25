-- ============================================================================
-- sql/30_product_available.sql
-- FUUDELIVERY — Item do cardápio pausado/esgotado
-- ============================================================================
-- products.available=false: o item continua no cardápio da loja, aparece como
-- "Esgotado" para o cliente e o pedido com ele é recusado pelo servidor
-- (computeOrderTotal). O AutoMigrate do orders_api cria a mesma coluna; este
-- arquivo é para quem aplica o schema pelos .sql.
--
-- Default true preserva o mundo atual: todo produto existente segue à venda.
-- IDEMPOTENTE: ADD COLUMN IF NOT EXISTS. Roda N vezes.
-- ============================================================================

DO $$
BEGIN
    IF to_regclass('public.products') IS NULL THEN
        RAISE NOTICE 'products ainda nao existe (criada pelo AutoMigrate do orders_api) — pulando sql/30.';
    ELSE
        ALTER TABLE products ADD COLUMN IF NOT EXISTS available BOOLEAN NOT NULL DEFAULT true;
    END IF;
END $$;
