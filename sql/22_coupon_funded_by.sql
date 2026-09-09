-- ============================================================================
-- 22_coupon_funded_by.sql
-- De quem sai o desconto do cupom.
--
-- Sem esta coluna o split não tem como saber de qual lado subtrair o desconto:
-- ele sairia de ninguém, e o pedido fecharia com a soma das partes (plataforma
-- + estabelecimento + entrega + cashback) maior do que o cliente pagou.
--
-- Valores: 'platform' (a taxa da plataforma absorve) ou 'establishment' (o
-- restaurante absorve).
--
-- DEFAULT 'platform' de propósito: cupom já existente foi criado sem escolha,
-- e assumir que o restaurante paga tiraria dinheiro de terceiro por omissão.
-- Quem oferece a promoção sem dizer nada é a plataforma.
--
-- Idempotente: ADD COLUMN IF NOT EXISTS.
-- ============================================================================

ALTER TABLE coupons
    ADD COLUMN IF NOT EXISTS funded_by VARCHAR(20) NOT NULL DEFAULT 'platform';

-- Trava os valores aceitos no banco, não só no Go: quem escrever direto no
-- Postgres (script, correção manual) não consegue inventar um terceiro valor
-- que o split não saiba tratar.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'coupons_funded_by_check'
    ) THEN
        ALTER TABLE coupons
            ADD CONSTRAINT coupons_funded_by_check
            CHECK (funded_by IN ('platform', 'establishment'));
    END IF;
END $$;

INSERT INTO schema_migrations (version, description)
VALUES ('22', 'coupon_funded_by')
ON CONFLICT DO NOTHING;
