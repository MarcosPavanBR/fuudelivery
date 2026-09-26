-- ============================================================================
-- sql/34_establishment_disabled.sql
-- FUUDELIVERY — Desativar loja pelo painel admin
-- ============================================================================
-- establishments.disabled_at: preenchido quando o admin desativa a loja. Ela
-- sai da vitrine, não recebe pedidos e não consegue se abrir; pedidos,
-- pagamentos e repasses antigos ficam intactos. Excluir de vez só é
-- permitido para loja sem histórico (auth_api DeleteEstablishment).
-- O AutoMigrate do auth_api cria a mesma coluna; este arquivo é para quem
-- aplica o schema pelos .sql.
-- IDEMPOTENTE: ADD COLUMN IF NOT EXISTS. Roda N vezes.
-- ============================================================================

ALTER TABLE establishments ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ;
