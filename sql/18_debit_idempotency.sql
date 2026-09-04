-- ============================================================================
-- 18_debit_idempotency.sql
-- Prevent double-debit on wallet transactions from webhook replay.
--
-- Problem: reverseWalletCredit uses abacatepayID as reference_id for debits,
-- but there's no unique constraint on debits. Concurrent REFUNDED webhooks
-- can double-debit the wallet.
--
-- Fix: Add unique partial index on debit reference_id, mirroring the credit
-- constraint from sql/11.
-- ============================================================================

CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_txns_debit_ref
    ON wallet_transactions (reference_id)
    WHERE type = 'debit' AND reference_id <> '';

INSERT INTO schema_migrations (version, description)
VALUES ('18', 'debit_idempotency')
ON CONFLICT DO NOTHING;
