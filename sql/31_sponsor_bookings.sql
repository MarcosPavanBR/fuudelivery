-- ============================================================================
-- sql/31_sponsor_bookings.sql
-- FUUDELIVERY — Destaque patrocinado por dia
-- ============================================================================
-- A loja reserva dias em que aparece no topo do app do cliente ("Patrocinado").
-- Vagas limitadas por dia e praça (SPONSOR_SLOTS_PER_DAY), preço por dia
-- (SPONSOR_DAILY_PRICE), rodízio entre as lojas do dia. Regras no código:
-- auth_api/app/models/sponsor_booking.go; cobrança e trava de concorrência
-- (pg_advisory_xact_lock por praça): payment_api/app/handlers/sponsor.go.
--
-- Dias em texto 'AAAA-MM-DD' (fuso da loja), end_day inclusivo.
-- O AutoMigrate do auth_api cria a mesma tabela; este arquivo é para quem
-- aplica o schema pelos .sql.
--
-- IDEMPOTENTE: CREATE TABLE / INDEX IF NOT EXISTS + CHECK guardado.
-- ============================================================================

CREATE TABLE IF NOT EXISTS sponsor_bookings (
    id               BIGSERIAL PRIMARY KEY,
    establishment_id BIGINT       NOT NULL,
    zone_id          BIGINT       NOT NULL DEFAULT 0,
    start_day        VARCHAR(10)  NOT NULL,
    end_day          VARCHAR(10)  NOT NULL,
    days             INTEGER      NOT NULL,
    price_per_day    NUMERIC(12,2) NOT NULL,
    total            NUMERIC(12,2) NOT NULL,
    status           VARCHAR(20)  NOT NULL,
    pay_with         VARCHAR(10)  NOT NULL,
    paid_at          TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ,
    cancelled_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sponsor_bookings_establishment_id ON sponsor_bookings (establishment_id);
CREATE INDEX IF NOT EXISTS idx_sponsor_bookings_zone_id ON sponsor_bookings (zone_id);
CREATE INDEX IF NOT EXISTS idx_sponsor_bookings_start_day ON sponsor_bookings (start_day);
CREATE INDEX IF NOT EXISTS idx_sponsor_bookings_end_day ON sponsor_bookings (end_day);
CREATE INDEX IF NOT EXISTS idx_sponsor_bookings_status ON sponsor_bookings (status);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_sponsor_bookings_status') THEN
        ALTER TABLE sponsor_bookings ADD CONSTRAINT chk_sponsor_bookings_status
            CHECK (status IN ('pending_payment', 'active', 'cancelled'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_sponsor_bookings_amounts') THEN
        ALTER TABLE sponsor_bookings ADD CONSTRAINT chk_sponsor_bookings_amounts
            CHECK (days >= 1 AND price_per_day >= 0 AND total >= 0 AND start_day <= end_day);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- RLS: padrão backend_full_access (mesmo de sql/06 e sql/17)
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL ON sponsor_bookings FROM anon;';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL ON sponsor_bookings FROM authenticated;';
    END IF;

    EXECUTE 'ALTER TABLE sponsor_bookings ENABLE ROW LEVEL SECURITY;';
    EXECUTE 'ALTER TABLE sponsor_bookings FORCE ROW LEVEL SECURITY;';

    EXECUTE 'DROP POLICY IF EXISTS backend_full_access ON sponsor_bookings;';
    EXECUTE
        'CREATE POLICY backend_full_access ON sponsor_bookings '
        'FOR ALL TO app_backend USING (true) WITH CHECK (true);';
END
$$;
