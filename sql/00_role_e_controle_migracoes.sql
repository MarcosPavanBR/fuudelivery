-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único (Supabase/Postgres)
-- 00 — Role dedicada do backend + tabela de controle de migrações
-- ============================================================================
-- POR QUE ESTE ARQUIVO EXISTE:
--   Hoje o backend provavelmente conecta no Postgres usando a role padrão
--   "postgres" (superusuário) via DB_CONNECTION_STRING. Isso é um risco: um
--   bug no backend, ou uma SQL injection residual, tem poder de superusuário
--   sobre o banco inteiro. Criamos uma role própria, com só os privilégios
--   que a aplicação realmente precisa.
--
--   Também criamos "schema_migrations": toda vez que um destes scripts (ou
--   qualquer alteração de schema futura) rodar, ele se registra aqui. Isso
--   é o "registro de cada mudança no banco de dados" que você pediu — é o
--   changelog de DDL (estrutura). O changelog de DML (dados alterados por
--   usuários/admin) é o "audit_log" do script 05.
-- ============================================================================

-- 1) Role de aplicação (login), sem privilégios de superusuário.
--    Troque a senha abaixo antes de rodar em produção — nunca deixe o
--    valor de exemplo. Depois de rodar, salve a senha no gerenciador de
--    segredos do Render (nunca em texto puro no repositório).
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'app_backend') THEN
        CREATE ROLE app_backend WITH LOGIN PASSWORD 'TROQUE_ESTA_SENHA_ANTES_DE_RODAR';
    END IF;
END
$$;

-- Privilégios mínimos: conectar, usar o schema public, e CRUD nas tabelas.
-- NÃO concede CREATE/DROP — alterações de schema continuam exigindo a role
-- de administração (a que você usa hoje, fora da aplicação).
GRANT CONNECT ON DATABASE postgres TO app_backend;
GRANT USAGE ON SCHEMA public TO app_backend;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_backend;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO app_backend;

-- 2) Controle de migrações (changelog de estrutura do banco)
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT PRIMARY KEY,           -- nome do arquivo, ex: '01_dominio_pedidos'
    description TEXT NOT NULL,
    applied_by  TEXT NOT NULL DEFAULT current_user,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE schema_migrations IS
    'Changelog de ESTRUTURA do banco. Cada script deste pacote registra aqui '
    'quando rodou. Antes de aplicar qualquer script, confira se a versão já '
    'não está registrada (todos os scripts são idempotentes e usam '
    'ON CONFLICT DO NOTHING, mas isso evita reprocessar sem necessidade).';

INSERT INTO schema_migrations (version, description)
VALUES ('00_role_e_controle_migracoes', 'Cria role app_backend com privilégios mínimos e tabela schema_migrations')
ON CONFLICT (version) DO NOTHING;

-- ----------------------------------------------------------------------------
-- PRÓXIMO PASSO MANUAL (fora do SQL):
--   Depois de rodar este script, atualize DB_CONNECTION_STRING no Render
--   para usar "app_backend" em vez do usuário superusuário atual.
--   Teste em ambiente de homologação antes de trocar em produção.
-- ----------------------------------------------------------------------------
