-- ============================================================================
-- 11_idempotencia_financeira.sql
-- Idempotência estrutural de créditos e cobranças.
--
-- Problema: a idempotência era check-then-act no código (HasLedgerEntry antes
-- de creditar), fail-open em erro de banco e sem constraint nenhuma — duas
-- requisições concorrentes (webhook reentregue, duplo top-up) creditavam 2x.
--
-- Correção em duas camadas:
--   1. UNIQUE parcial no ledger: um único lançamento de crédito por
--      referência. Débitos ficam de fora (podem existir vários legítimos por
--      pedido: saque, estorno, dedução) e são guardados pelo saldo + FOR UPDATE.
--   2. UNIQUE no abacatepay_id do pagamento: uma linha por cobrança do gateway.
--
-- O código (wallet.go / webhook.go) passa a tratar a violação única como
-- "crédito já aplicado" (409 idempotente) em vez de checar antes.
--
-- Idempotente: CREATE UNIQUE INDEX IF NOT EXISTS.
-- ============================================================================

CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_txns_credit_ref
    ON wallet_transactions (reference_id)
    WHERE type = 'credit' AND reference_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_abacatepay_id
    ON payments (abacatepay_id)
    WHERE abacatepay_id IS NOT NULL AND abacatepay_id <> '';

-- Atenção operacional: se houver dados históricos duplicados, o CREATE acima
-- falha com "could not create unique index". Nesse caso rodar primeiro uma
-- auditoria: SELECT reference_id, COUNT(*) FROM wallet_transactions WHERE
-- type='credit' GROUP BY reference_id HAVING COUNT(*) > 1;

INSERT INTO schema_migrations (version, description)
VALUES ('11', 'idempotencia_financeira')
ON CONFLICT DO NOTHING;
