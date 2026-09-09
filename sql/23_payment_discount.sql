-- ============================================================================
-- 23_payment_discount.sql
-- O desconto do cupom na cobrança.
--
-- Sem estas colunas o split não tem como saber de qual lado subtrair a
-- promoção: as porcentagens incidiriam sobre o valor JÁ descontado e o cupom
-- sairia rateado entre plataforma e restaurante, em proporção, qualquer que
-- fosse a escolha feita na criação do cupom (ver sql/22_coupon_funded_by.sql).
--
-- Os dois valores são escritos pelo servidor a partir do PEDIDO
-- (order_documents.payload, campos discount_amount e discount_funded_by), nunca
-- do corpo da requisição de cobrança — em dto.PaymentRequest os campos são
-- `json:"-"` justamente por isso.
--
-- Cobrança antiga fica com 0 / NULL, que é exatamente "sem cupom": o split
-- volta a dividir como sempre dividiu.
--
-- Idempotente: ADD COLUMN IF NOT EXISTS.
-- ============================================================================

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0;

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS discount_funded_by VARCHAR(20);

-- Mesma trava da tabela de cupons: quem escrever direto no Postgres não
-- consegue inventar um terceiro lado que o split não saiba tratar. NULL é
-- permitido e significa "sem cupom".
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'payments_discount_funded_by_check'
    ) THEN
        ALTER TABLE payments
            ADD CONSTRAINT payments_discount_funded_by_check
            CHECK (discount_funded_by IS NULL
                   OR discount_funded_by IN ('platform', 'establishment'));
    END IF;
END $$;

-- Desconto negativo inflaria o bruto e, com ele, a fatia percentual do
-- estabelecimento acima do que o pedido pagou.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'payments_discount_amount_check'
    ) THEN
        ALTER TABLE payments
            ADD CONSTRAINT payments_discount_amount_check
            CHECK (discount_amount >= 0);
    END IF;
END $$;

INSERT INTO schema_migrations (version, description)
VALUES ('23', 'payment_discount')
ON CONFLICT DO NOTHING;
