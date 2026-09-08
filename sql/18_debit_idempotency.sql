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

-- SUPERSEDIDO POR sql/20_debit_idempotency_por_wallet.sql, que reescopa este
-- índice para (wallet_id, reference_id).
--
-- Por isso o CREATE é condicional: num banco que já aplicou o 20, duas
-- carteiras podem legitimamente compartilhar a mesma reference_id de débito —
-- e recriar o índice GLOBAL aqui falharia (não dá para criar índice único
-- sobre dados que o violam), abortando o run_all.sh inteiro com
-- ON_ERROR_STOP=1. Mantido para bancos que ainda não chegaram ao 20.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE tablename = 'wallet_transactions'
          AND indexname = 'uq_wallet_txns_debit_ref_wallet'
    ) THEN
        CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_txns_debit_ref
            ON wallet_transactions (reference_id)
            WHERE type = 'debit' AND reference_id <> '';
    END IF;
END $$;

INSERT INTO schema_migrations (version, description)
VALUES ('18', 'debit_idempotency')
ON CONFLICT DO NOTHING;
