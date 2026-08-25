-- ============================================================================
-- FUUDELIVERY — Criação da tabela refresh_tokens
-- 11 — Tabela de refresh tokens (acesso +30d, renovação automática)
-- ============================================================================
-- PROBLEMA: esta tabela faltava em produção (AutoMigrate falhou silenciosamente
-- no startup do monolito). Sem ela, createTokenPair() retorna 500 em todo
-- registro e login — "Failed to generate tokens".
-- IDEMPOTENTE: pode rodar quantas vezes quiser.

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL,
    token      VARCHAR(64) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked    BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Índice para buscas por user_id (cleanup, refresh, revoke)
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id
    ON refresh_tokens(user_id);

-- Índice para cleanup de tokens expirados/revogados
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at
    ON refresh_tokens(expires_at);

-- Conceder acesso ao role da aplicação
GRANT SELECT, INSERT, UPDATE, DELETE ON refresh_tokens TO app_backend;
GRANT USAGE, SELECT ON SEQUENCES IN SCHEMA public TO app_backend;

-- Conceder acesso ao service_role (REST/Supabase)
GRANT SELECT, INSERT, UPDATE, DELETE ON refresh_tokens TO service_role;

COMMENT ON TABLE refresh_tokens IS
    'Refresh tokens JWT (30 dias). Criado em 2026-08-24 — tabela estava faltando '
    'em produção porque o AutoMigrate do monolito falhou silenciosamente no '
    'startup. Cada token é vinculado a um user_id e pode ser revogado no logout.';
