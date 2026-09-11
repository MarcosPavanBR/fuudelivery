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

-- coupons é criada pelo AutoMigrate do GORM, não por SQL. Num banco NOVO,
-- antes do primeiro boot, ela não existe — e sem este guarda o script abortava
-- a suíte, levando junto os scripts 23 e 24.
DO $$
BEGIN
    IF to_regclass('public.coupons') IS NULL THEN
        RAISE NOTICE 'coupons ainda nao existe (AutoMigrate nao rodou) — pulando funded_by. Rode run_all.sh de novo apos o primeiro boot.';
        RETURN;
    END IF;

    ALTER TABLE coupons
        ADD COLUMN IF NOT EXISTS funded_by VARCHAR(20) NOT NULL DEFAULT 'platform';

    -- Trava os valores aceitos no banco, não só no Go: quem escrever direto no
    -- Postgres (script, correção manual) não consegue inventar um terceiro
    -- valor que o split não saiba tratar.
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
