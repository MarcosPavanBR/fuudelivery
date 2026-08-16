-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 07 — Auditoria de tabelas órfãs ("olhar todo o banco e manter só o que
--       condiz com o projeto")
-- ============================================================================
-- COMO FUNCIONA:
--   A lista abaixo foi levantada lendo o código-fonte de verdade: cada
--   struct com `gorm:"primaryKey"` em auth_api/orders_api, mais as
--   tabelas novas criadas pelos scripts 01-06 deste pacote para substituir
--   as coleções MongoDB. Este script NÃO apaga nada — só compara essa
--   lista com o que existe DE VERDADE no schema "public" do Supabase e
--   aponta as diferenças, para revisão humana.
--
--   Roda quantas vezes você quiser, é só leitura:
--     psql "$DB_CONNECTION_STRING" -f sql/07_auditoria_tabelas_orfas.sql
-- ============================================================================

WITH tabelas_do_codigo(nome) AS (
    VALUES
        ('users'), ('establishments'), ('delivery_men'), ('business_hours'), ('zones'),
        ('subscriptions'), ('sponsored_listings'), ('clients'),
        ('categories'), ('category_products'), ('products'), ('additionals'), ('additional_products'),
        ('orders'), ('order_items'), ('coupons'), ('coupon_usages'),
        ('loyalty_points'), ('loyalty_transactions'), ('reviews'), ('batches'), ('delivery'),
        ('push_tokens'),
        ('delivery_solicitations'),
        ('payments'), ('wallets'), ('wallet_transactions'), ('chargebacks'),
        ('chargeback_evidence'), ('payout_requests'), ('payment_approval_rules'),
        ('payment_admin_users'),
        ('chat_messages'),
        -- infraestrutura deste próprio pacote de migração, não é "lixo"
        ('schema_migrations'), ('audit_log'), ('audit_redacted_columns')
),
tabelas_no_banco AS (
    SELECT tablename AS nome
      FROM pg_tables
     WHERE schemaname = 'public'
)
SELECT
    COALESCE(b.nome, c.nome)                              AS tabela,
    CASE
        WHEN b.nome IS NOT NULL AND c.nome IS NOT NULL THEN 'OK — existe no banco e no código'
        WHEN b.nome IS NOT NULL AND c.nome IS NULL       THEN '⚠ ÓRFÃ — existe no banco, mas nenhum código conhecido usa. Revisar antes de dropar (pode ser tabela nova que ainda não entrou nesta lista — atualize-a se for o caso).'
        WHEN b.nome IS NULL AND c.nome IS NOT NULL       THEN '❌ FALTANDO — o código espera esta tabela e ela NÃO existe no banco. Rode os scripts 00-06 deste pacote.'
    END AS situacao,
    CASE WHEN b.nome IS NOT NULL
         THEN pg_size_pretty(pg_total_relation_size(quote_ident(b.nome)::regclass))
         ELSE '-' -- tabela não existe no banco, não dá pra medir tamanho
    END AS tamanho
FROM tabelas_no_banco b
FULL OUTER JOIN tabelas_do_codigo c ON b.nome = c.nome
ORDER BY
    (b.nome IS NOT NULL AND c.nome IS NULL) DESC,  -- órfãs primeiro (mais urgente revisar)
    (b.nome IS NULL) DESC,                          -- depois faltando
    tabela;

-- ----------------------------------------------------------------------------
-- Se aparecer alguma tabela "⚠ ÓRFÃ" que vocês confirmarem ser lixo mesmo
-- (ex: tabela de teste, feature descontinuada), o passo seguro é:
--
--   1. Renomear em vez de apagar direto (reversível):
--        ALTER TABLE nome_da_tabela RENAME TO zz_deprecated_nome_da_tabela;
--   2. Esperar 1-2 semanas monitorando erros de aplicação.
--   3. Só então, se nada quebrou:
--        DROP TABLE zz_deprecated_nome_da_tabela;
--
-- Nunca rode DROP TABLE direto em produção sem esse período de quarentena.
-- ----------------------------------------------------------------------------

INSERT INTO schema_migrations (version, description)
VALUES ('07_auditoria_tabelas_orfas', 'Script de auditoria (somente leitura) comparando tabelas do banco vs código')
ON CONFLICT (version) DO NOTHING;
