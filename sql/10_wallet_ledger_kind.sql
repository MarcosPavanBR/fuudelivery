-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 10 — Classificação de débitos no ledger da carteira (kind/destination)
-- ============================================================================
-- CONTEXTO:
--   O modelo GORM WalletTxn (Backend/payment_api/app/models/wallet.go) tem as
--   colunas `kind` e `destination`, usadas para classificar débitos:
--     - kind        '' = débito normal | 'withdrawal' = saque via PIX
--     - destination = chave PIX de destino do saque (dado de auditoria)
--
--   Essas colunas só eram criadas pelo AutoMigrate do GORM — não existiam em
--   SQL versionado, quebrando a regra de governança do projeto (todo schema
--   deve estar aqui em sql/ para ambientes novos e auditoria).
--
-- IDEMPOTENTE: ADD COLUMN IF NOT EXISTS pode rodar quantas vezes quiser.
-- ============================================================================

ALTER TABLE wallet_transactions
    ADD COLUMN IF NOT EXISTS kind VARCHAR(20) NOT NULL DEFAULT '';

ALTER TABLE wallet_transactions
    ADD COLUMN IF NOT EXISTS destination TEXT;

COMMENT ON COLUMN wallet_transactions.kind IS
    'Classificação do lançamento: '''' = débito/crédito normal, ''withdrawal'' = saque via PIX.';
COMMENT ON COLUMN wallet_transactions.destination IS
    'Chave PIX de destino quando kind = ''withdrawal''. Dado de auditoria financeira.';

-- Consultas típicas: extrato de saques por carteira e conciliação de saques.
CREATE INDEX IF NOT EXISTS idx_wallet_txns_kind
    ON wallet_transactions (wallet_id, kind);

INSERT INTO schema_migrations (version, description)
VALUES ('10_wallet_ledger_kind', 'Versiona colunas kind/destination do ledger da carteira (antes só existiam via AutoMigrate)')
ON CONFLICT (version) DO NOTHING;
