-- ============================================================================
-- FUUDELIVERY — Unique constraints de defense-in-depth
-- 17 — Unique constraints para evitar race conditions em coupon e loyalty
-- ============================================================================
-- POR QUE ESTE ARQUIVO EXISTE:
--   As transações de aplicação de cupom e crédito de pontos de fidelidade
--   foram reforçadas com transações atômicas no código Go, mas o banco
--   ainda não tem constraints únicas. Em cenários de alta concorrência
--   (webhook + API simultâneos), o banco é a última linha de defesa.
--
--   Este script adiciona:
--   1. UNIQUE(coupon_id, user_phone, order_id) em coupon_usages
--      → impede que o mesmo usuário aplique o mesmo cupom no mesmo pedido
--   2. UNIQUE(order_id, type, user_phone) em loyalty_transactions onde type='earn'
--      → impede crédito duplicado de pontos para o mesmo pedido
-- ============================================================================

-- coupon_usages e loyalty_transactions são criadas pelo AutoMigrate do GORM,
-- não por script SQL. Num banco NOVO, onde a aplicação ainda não subiu, elas
-- não existem e o CREATE INDEX abortava a suíte inteira — os scripts 18 a 24
-- nunca chegavam a rodar. Os guardas abaixo deixam este script rodar antes OU
-- depois do primeiro boot: sem as tabelas ele avisa e segue; com elas, cria os
-- índices. Rodar run_all.sh de novo depois do boot fecha a lacuna.

-- 1) Coupon usage: unique por (coupon, usuario, pedido)
-- Permite que o MESMO pedido use cupons DIFERENTES, mas não o mesmo cupom.
DO $$
BEGIN
    IF to_regclass('public.coupon_usages') IS NULL THEN
        RAISE NOTICE 'coupon_usages ainda nao existe (AutoMigrate nao rodou) — pulando uq_coupon_usage_per_order. Rode run_all.sh de novo apos o primeiro boot da aplicacao.';
    ELSE
        EXECUTE 'CREATE UNIQUE INDEX IF NOT EXISTS uq_coupon_usage_per_order
                 ON coupon_usages (coupon_id, user_phone, order_id)';
    END IF;
END $$;

-- 2) Loyalty earn: unique por (pedido, tipo=earn, usuario)
-- Permite outros tipos (redeem, bonus) para o mesmo pedido, mas não dois "earn".
DO $$
BEGIN
    IF to_regclass('public.loyalty_transactions') IS NULL THEN
        RAISE NOTICE 'loyalty_transactions ainda nao existe (AutoMigrate nao rodou) — pulando uq_loyalty_earn_per_order. Rode run_all.sh de novo apos o primeiro boot da aplicacao.';
    ELSE
        EXECUTE 'CREATE UNIQUE INDEX IF NOT EXISTS uq_loyalty_earn_per_order
                 ON loyalty_transactions (order_id, user_phone)
                 WHERE type = ''earn''';
    END IF;
END $$;

-- Registra a migration
INSERT INTO schema_migrations (version, description)
VALUES ('17_unique_constraints', 'Unique constraints de defense-in-depth para coupon_usages e loyalty_transactions')
ON CONFLICT (version) DO NOTHING;
