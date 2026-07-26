-- ============================================================
-- FUUDELIVERY — Migration: Create batches table
-- Execute this manually if AutoMigrate did not create it.
-- Usage: psql "$DB_CONNECTION_STRING" -f scripts/migrate-batches.sql
-- ============================================================

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

COMMENT ON TABLE batches IS 'Agrupa multiplos pedidos sob um unico entregador na mesma rota (batching)';
COMMENT ON COLUMN batches.status IS 'active, delivering, completed, cancelled';
