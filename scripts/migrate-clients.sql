-- ============================================================
-- FUUDELIVERY — Migration: Create clients table
-- Execute this manually if AutoMigrate did not create it.
-- Usage: psql "$DB_CONNECTION_STRING" -f scripts/migrate-clients.sql
-- ============================================================

CREATE TABLE IF NOT EXISTS clients (
    id         BIGSERIAL    PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    phone      VARCHAR(20)  NOT NULL UNIQUE,
    password   VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clients_phone ON clients(phone);

COMMENT ON TABLE clients IS 'Clientes do AppComida (consumidores finais)';
COMMENT ON COLUMN clients.password IS 'Hash bcrypt da senha do cliente';
