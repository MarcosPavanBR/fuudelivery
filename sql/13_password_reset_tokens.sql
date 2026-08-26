-- ============================================================================
-- FUUDELIVERY — Reset de senha assistido
-- 13 — Tabela password_reset_tokens (código de uso único gerado pelo admin)
-- ============================================================================
-- POR QUE ESTE SCRIPT EXISTE:
--   O projeto não tem fluxo "esqueci minha senha" e não tem serviço de email;
--   clientes (clients) NÃO possuem email — só telefone. O reset passa a ser
--   assistido: o suporte gera um código de uso único no WebAdmin e informa ao
--   usuário por telefone/WhatsApp; ele define a nova senha na página pública
--   /resetar-senha (WebRestaurant) que chama POST /auth/reset-password.
--
-- SEGURANÇA:
--   - Só o hash SHA-256 do código é persistido (code_hash).
--   - code_hash entra em audit_redacted_columns (regra da skill banco-unico).
--   - Trigger de auditoria (fn_audit_trigger do script 05) registra toda
--     INSERT/UPDATE/DELETE na audit_log.
--   - RLS habilitado com policy backend_full_access (padrão do script 06) —
--     NUNCA usar o padrão auth.uid() = user_id: este projeto não usa Supabase
--     Auth e auth.uid() aqui sempre retorna nulo.
--
-- IDEMPOTENTE: pode rodar quantas vezes quiser.
-- ============================================================================

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_type  VARCHAR(20) NOT NULL,      -- client | user | delivery_man
    user_id    INTEGER NOT NULL,          -- ID na tabela de origem (clients/users/delivery_men)
    code_hash  CHAR(64) NOT NULL,         -- SHA-256 hex do código; NUNCA o código em claro
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,               -- NULL enquanto o código está válido
    attempts   INT NOT NULL DEFAULT 0,    -- tentativas erradas; teto no backend (5)
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pwd_reset_lookup
    ON password_reset_tokens (user_type, user_id);
CREATE INDEX IF NOT EXISTS idx_pwd_reset_expires_at
    ON password_reset_tokens (expires_at);

COMMENT ON TABLE password_reset_tokens IS
    'Códigos de uso único para reset assistido de senha ("esqueci minha senha"). '
    'O suporte gera o código no WebAdmin e informa ao usuário fora de banda '
    '(telefone/WhatsApp). Só o hash SHA-256 é persistido. TTL de 15 minutos e '
    'máximo de 5 tentativas por código (aplicados no backend Go).';

COMMENT ON COLUMN password_reset_tokens.code_hash IS
    'SHA-256 hex do código informado ao usuário. Coluna redigida no audit_log.';

-- ---------------------------------------------------------------------------
-- Auditoria: mesma mecânica do script 05, anexada aqui porque esta tabela
-- nasce DEPOIS do loop original.
-- ---------------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_audit_password_reset_tokens ON password_reset_tokens;
CREATE TRIGGER trg_audit_password_reset_tokens
    AFTER INSERT OR UPDATE OR DELETE ON password_reset_tokens
    FOR EACH ROW EXECUTE FUNCTION fn_audit_trigger();

INSERT INTO audit_redacted_columns (table_name, column_name)
VALUES ('password_reset_tokens', 'code_hash')
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- RLS: revoga as roles públicas do Supabase (condicional — em Postgres puro
-- elas não existem) e libera apenas app_backend, padrão do script 06.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        EXECUTE 'REVOKE ALL ON password_reset_tokens FROM anon;';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        EXECUTE 'REVOKE ALL ON password_reset_tokens FROM authenticated;';
    END IF;

    EXECUTE 'ALTER TABLE password_reset_tokens ENABLE ROW LEVEL SECURITY;';
    EXECUTE 'ALTER TABLE password_reset_tokens FORCE ROW LEVEL SECURITY;';

    EXECUTE 'DROP POLICY IF EXISTS backend_full_access ON password_reset_tokens;';
    EXECUTE
        'CREATE POLICY backend_full_access ON password_reset_tokens '
        'FOR ALL TO app_backend USING (true) WITH CHECK (true);';
END
$$;

GRANT SELECT, INSERT, UPDATE, DELETE ON password_reset_tokens TO app_backend;
GRANT USAGE, SELECT ON SEQUENCES IN SCHEMA public TO app_backend;

-- service_role (chave SUPABASE_SERVICE_ROLE_KEY usada em REST/Storage)
GRANT SELECT, INSERT, UPDATE, DELETE ON password_reset_tokens TO service_role;

INSERT INTO schema_migrations (version, description)
VALUES ('13_password_reset_tokens', 'Cria password_reset_tokens (reset assistido via código de uso único gerado no WebAdmin), com auditoria, redação do hash e RLS app_backend')
ON CONFLICT (version) DO NOTHING;
