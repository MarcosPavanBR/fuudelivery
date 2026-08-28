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

-- 1) Coupon usage: unique por (coupon, usuario, pedido)
-- Permite que o MESMO pedido use cupons DIFERENTES, mas não o mesmo cupom.
CREATE UNIQUE INDEX IF NOT EXISTS uq_coupon_usage_per_order
    ON coupon_usages (coupon_id, user_phone, order_id);

-- 2) Loyalty earn: unique por (pedido, tipo=earn, usuario)
-- Permite outros tipos (redeem, bonus) para o mesmo pedido, mas não dois "earn".
CREATE UNIQUE INDEX IF NOT EXISTS uq_loyalty_earn_per_order
    ON loyalty_transactions (order_id, user_phone)
    WHERE type = 'earn';

-- Registra a migration
INSERT INTO schema_migrations (version, description)
VALUES ('17_unique_constraints', 'Unique constraints de defense-in-depth para coupon_usages e loyalty_transactions')
ON CONFLICT (version) DO NOTHING;
