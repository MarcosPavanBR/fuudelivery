-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 06 — RLS (Row Level Security) e checklist de segurança do banco
-- ============================================================================
-- AVISO IMPORTANTE, leia antes de copiar isto de outro lugar:
--   O padrão que toda IA (inclusive eu, se eu não prestar atenção) copia e
--   cola para RLS no Supabase é algo como:
--       CREATE POLICY ... USING (auth.uid() = user_id);
--   Isso só funciona se o login passar pelo Supabase Auth, que popula
--   auth.uid(). ESTE PROJETO NÃO USA Supabase Auth — o login é feito por
--   JWT próprio (auth_api, bcrypt + HS256). auth.uid() aqui SEMPRE
--   retornaria NULL, e uma policy baseada nisso bloquearia geral (o que
--   pareceria seguro, mas na prática só daria falsa sensação de proteção,
--   porque ninguém validou isso de verdade).
--
--   Por isso a estratégia aqui é outra, e é a correta para este caso:
--     1. Habilitar RLS em todas as tabelas de negócio.
--     2. Revogar acesso das roles padrão do Supabase (anon, authenticated)
--        — ninguém que não seja o backend consegue ler ou escrever direto
--        via API REST do Supabase.
--     3. Dar UMA policy permissiva só para a role app_backend (criada no
--        script 00), que é quem o backend Go usa para conectar.
--     4. Toda a lógica de "usuário só vê o que é dele" continua sendo
--        responsabilidade do backend Go (middleware AuthRequired /
--        AdminRequired, que já existe conforme docs/seguranca.md) — RLS
--        aqui é a segunda camada, não substitui a primeira.
-- ============================================================================

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'users','establishments','delivery_men','clients','zones',
        'subscriptions','sponsored_listings',
        'categories','products','additionals','orders','order_items',
        'coupons','coupon_usages','loyalty_points','reviews','batches',
        'push_tokens', 'delivery_solicitations',
        'payments','wallets','wallet_transactions','chargebacks',
        'chargeback_evidence','payout_requests','payment_approval_rules',
        'payment_admin_users',
        'chat_messages'
    ]
    LOOP
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = t) THEN
            -- Revoga tudo das roles padrão do Supabase que atendem à API REST
            -- pública. Testado condicionalmente porque em Postgres puro
            -- (fora do Supabase, ex: ambiente local de teste) essas roles
            -- não existem e o REVOKE quebraria o script inteiro.
            IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
                EXECUTE format('REVOKE ALL ON %I FROM anon;', t);
            END IF;
            IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
                EXECUTE format('REVOKE ALL ON %I FROM authenticated;', t);
            END IF;

            -- Habilita RLS
            EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY;', t);
            EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY;', t);

            -- Uma única policy: só app_backend passa, e passa para tudo
            -- (SELECT/INSERT/UPDATE/DELETE). O controle fino de "quem vê o
            -- quê" é feito no backend, não aqui.
            EXECUTE format('DROP POLICY IF EXISTS backend_full_access ON %I;', t);
            EXECUTE format(
                'CREATE POLICY backend_full_access ON %I '
                'FOR ALL TO app_backend USING (true) WITH CHECK (true);', t
            );
        END IF;
    END LOOP;
END
$$;

-- audit_log e schema_migrations também não devem ser expostos via API pública
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon')
       AND EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON audit_log FROM anon, authenticated;
        REVOKE ALL ON schema_migrations FROM anon, authenticated;
        REVOKE ALL ON audit_redacted_columns FROM anon, authenticated;
    END IF;
END
$$;

INSERT INTO schema_migrations (version, description)
VALUES ('06_rls_seguranca', 'Habilita RLS em todas as tabelas de negócio e restringe acesso apenas à role app_backend; revoga anon/authenticated')
ON CONFLICT (version) DO NOTHING;

-- ============================================================================
-- CHECKLIST DE SEGURANÇA DO BANCO — itens que uma IA tende a esquecer
-- (marque cada um conforme aplicar; nada disso é feito automaticamente
-- por rodar este script)
-- ============================================================================
-- [ ] app_backend tem senha forte, única, guardada em secret manager
--     (Render env var), NUNCA commitada. Rotacionar se algum dia vazar.
-- [ ] DB_CONNECTION_STRING usa sslmode=require (Supabase já força isso,
--     mas confirme na connection string usada pelo Go).
-- [ ] Nenhuma rota de API devolve card_token, password, password_hash,
--     pix_copy_paste ou qr_code_base64 no JSON de resposta.
-- [ ] Backups automáticos do Supabase estão ativos (Settings > Database >
--     Backups) — auditoria e RLS não substituem backup.
-- [ ] audit_log tem uma rotina de retenção (ex: mover para tabela "fria"
--     ou exportar após 12 meses) — ele cresce para sempre do jeito que
--     está, e isso é intencional por enquanto, mas monitore o tamanho.
-- [ ] A role de administração usada por você (fora da aplicação) para
--     rodar estes scripts é DIFERENTE de app_backend e tem MFA ativado
--     no painel do Supabase.
-- [ ] Reveja de tempos em tempos: SELECT * FROM pg_policies WHERE
--     schemaname = 'public'; — para garantir que nenhuma policy nova
--     "de exemplo" (tipo auth.uid()) foi colada por engano numa
--     migração futura.
-- ============================================================================
