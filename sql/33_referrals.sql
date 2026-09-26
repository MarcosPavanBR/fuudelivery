-- ============================================================================
-- sql/33_referrals.sql
-- FUUDELIVERY — Indicação de amigos
-- ============================================================================
-- referral_codes: código pessoal de cada cliente ("AMIGO7K3QX").
-- referrals: quem indicou → amigo novo (um por amigo), com o cupom de
-- boas-vindas criado para ele; vira 'rewarded' quando o 1º pedido do amigo é
-- entregue e quem indicou ganha os pontos. Regras em
-- orders_api/app/handlers/referral.go. O AutoMigrate do orders_api cria as
-- mesmas tabelas; este arquivo é para quem aplica o schema pelos .sql.
--
-- IDEMPOTENTE: CREATE TABLE / INDEX IF NOT EXISTS + RLS padrão de sql/06.
-- ============================================================================

CREATE TABLE IF NOT EXISTS referral_codes (
    id         BIGSERIAL PRIMARY KEY,
    phone      VARCHAR(32) NOT NULL,
    code       VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_referral_codes_phone ON referral_codes (phone);
CREATE UNIQUE INDEX IF NOT EXISTS idx_referral_codes_code ON referral_codes (code);

CREATE TABLE IF NOT EXISTS referrals (
    id             BIGSERIAL PRIMARY KEY,
    referrer_phone VARCHAR(32) NOT NULL,
    referee_phone  VARCHAR(32) NOT NULL,
    coupon_code    VARCHAR(40) NOT NULL,
    status         VARCHAR(20) NOT NULL,
    order_id       VARCHAR(64),
    rewarded_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_referrals_referrer_phone ON referrals (referrer_phone);
CREATE UNIQUE INDEX IF NOT EXISTS idx_referrals_referee_phone ON referrals (referee_phone);
CREATE UNIQUE INDEX IF NOT EXISTS idx_referrals_coupon_code ON referrals (coupon_code);
CREATE INDEX IF NOT EXISTS idx_referrals_status ON referrals (status);
CREATE INDEX IF NOT EXISTS idx_referrals_order_id ON referrals (order_id);

DO $$
DECLARE
    t TEXT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_referrals_status') THEN
        ALTER TABLE referrals ADD CONSTRAINT chk_referrals_status CHECK (status IN ('pending', 'rewarded'));
    END IF;
    FOREACH t IN ARRAY ARRAY['referral_codes', 'referrals'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
            EXECUTE format('REVOKE ALL ON %I FROM anon;', t);
        END IF;
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
            EXECUTE format('REVOKE ALL ON %I FROM authenticated;', t);
        END IF;
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY;', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY;', t);
        EXECUTE format('DROP POLICY IF EXISTS backend_full_access ON %I;', t);
        EXECUTE format('CREATE POLICY backend_full_access ON %I FOR ALL TO app_backend USING (true) WITH CHECK (true);', t);
    END LOOP;
END
$$;
