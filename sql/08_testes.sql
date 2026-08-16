-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 08 — Testes automatizados
-- ============================================================================
-- Roda uma bateria de verificações e IMPRIME PASS/FAIL para cada uma.
-- Não usa extensão nenhuma (nem pgTAP) de propósito — só SQL puro — para
-- rodar em qualquer Postgres do Supabase sem instalar nada.
--
-- Como rodar:
--   psql "$DB_CONNECTION_STRING" -f sql/08_testes.sql
--
-- Se algum teste falhar, o script AVISA mas não trava os demais — você vê
-- a lista completa de problemas de uma vez só.
-- ============================================================================

DO $$
DECLARE
    r RECORD;
    v_count INT;
    v_ok BOOLEAN;
    v_test_order_id TEXT := 'TESTE_AUDIT_' || floor(random()*1000000)::text;
    v_row_id BIGINT;
    v_audit_count INT;
BEGIN
    RAISE NOTICE '=== FUUDELIVERY — bateria de testes pós-migração ===';

    -- TESTE 1: todas as tabelas esperadas existem
    FOR r IN
        SELECT nome FROM (VALUES
            ('push_tokens'), ('delivery_solicitations'),
            ('payments'), ('wallets'), ('wallet_transactions'),
            ('chargebacks'), ('chargeback_evidence'), ('payout_requests'),
            ('payment_approval_rules'), ('payment_admin_users'),
            ('chat_messages'), ('audit_log'), ('schema_migrations')
        ) AS t(nome)
    LOOP
        SELECT EXISTS (
            SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename = r.nome
        ) INTO v_ok;
        IF v_ok THEN
            RAISE NOTICE '[PASS] tabela % existe', r.nome;
        ELSE
            RAISE WARNING '[FAIL] tabela % NÃO existe — rode os scripts 00-06', r.nome;
        END IF;
    END LOOP;

    -- TESTE 2: RLS habilitado em todas as tabelas de negócio
    FOR r IN
        SELECT tablename FROM pg_tables
         WHERE schemaname='public'
           AND tablename NOT IN ('schema_migrations','audit_log','audit_redacted_columns')
    LOOP
        SELECT relrowsecurity FROM pg_class
         WHERE relname = r.tablename AND relnamespace = 'public'::regnamespace
         INTO v_ok;
        IF v_ok THEN
            RAISE NOTICE '[PASS] RLS habilitado em %', r.tablename;
        ELSE
            RAISE WARNING '[FAIL] RLS DESABILITADO em % — rode o script 06', r.tablename;
        END IF;
    END LOOP;

    -- TESTE 3: anon e authenticated não têm privilégio nenhum nas tabelas de negócio
    SELECT count(*) INTO v_count
      FROM information_schema.role_table_grants
     WHERE grantee IN ('anon','authenticated')
       AND table_schema = 'public'
       AND table_name NOT IN ('schema_migrations'); -- essa é sempre restrita também, mas conferimos as de negócio
    IF v_count = 0 THEN
        RAISE NOTICE '[PASS] anon/authenticated não têm nenhum GRANT em tabelas public';
    ELSE
        RAISE WARNING '[FAIL] anon/authenticated ainda têm % grant(s) — rode REVOKE do script 06 novamente', v_count;
    END IF;

    -- TESTE 4: audit_log realmente grava em INSERT/UPDATE/DELETE
    -- (usa chat_messages por ser a tabela mais simples e sem FK)
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='chat_messages') THEN
        INSERT INTO chat_messages (order_id, sender_id, sender_type, message)
        VALUES (v_test_order_id, 999999, 'client', 'mensagem de teste do script 08')
        RETURNING id INTO v_row_id;

        UPDATE chat_messages SET message = 'mensagem de teste EDITADA' WHERE id = v_row_id;
        DELETE FROM chat_messages WHERE id = v_row_id;

        SELECT count(*) INTO v_audit_count
          FROM audit_log
         WHERE table_name = 'chat_messages' AND row_pk = v_row_id::text;

        IF v_audit_count = 3 THEN
            RAISE NOTICE '[PASS] audit_log registrou INSERT+UPDATE+DELETE (3 linhas) para chat_messages.id=%', v_row_id;
        ELSE
            RAISE WARNING '[FAIL] audit_log registrou % linha(s) (esperado 3) para chat_messages.id=% — confira o trigger do script 05', v_audit_count, v_row_id;
        END IF;
    ELSE
        RAISE WARNING '[SKIP] chat_messages não existe, não foi possível testar audit_log';
    END IF;

    -- TESTE 5: redação de coluna sensível funciona (payment_admin_users.password_hash)
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='payment_admin_users') THEN
        INSERT INTO payment_admin_users (email, name, password_hash, role)
        VALUES ('teste_script08_' || v_test_order_id || '@exemplo.com', 'Teste Script 08', 'segredo_nao_deveria_aparecer', 'operator')
        RETURNING id INTO v_row_id;

        DELETE FROM payment_admin_users WHERE id = v_row_id;

        SELECT count(*) INTO v_count
          FROM audit_log
         WHERE table_name = 'payment_admin_users'
           AND row_pk = v_row_id::text
           AND (after_data->>'password_hash' = 'segredo_nao_deveria_aparecer'
                OR before_data->>'password_hash' = 'segredo_nao_deveria_aparecer');

        IF v_count = 0 THEN
            RAISE NOTICE '[PASS] password_hash foi redigido no audit_log (não aparece em texto puro)';
        ELSE
            RAISE WARNING '[FAIL] password_hash apareceu em TEXTO PURO no audit_log — corrija audit_redacted_columns/script 05 antes de ir para produção';
        END IF;
    ELSE
        RAISE WARNING '[SKIP] payment_admin_users não existe, não foi possível testar redação';
    END IF;

    -- TESTE 6: payment_approval_rules tem exatamente 1 linha (config global)
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='payment_approval_rules') THEN
        SELECT count(*) INTO v_count FROM payment_approval_rules;
        IF v_count = 1 THEN
            RAISE NOTICE '[PASS] payment_approval_rules tem exatamente 1 linha';
        ELSE
            RAISE WARNING '[FAIL] payment_approval_rules tem % linha(s), esperado 1', v_count;
        END IF;
    END IF;

    -- TESTE 7: schema_migrations tem registro de todos os scripts deste pacote
    SELECT count(*) INTO v_count FROM schema_migrations
     WHERE version IN (
        '00_role_e_controle_migracoes','01_dominio_pedidos','02_dominio_entrega',
        '03_dominio_pagamentos','04_dominio_chat','05_audit_log','06_rls_seguranca'
     );
    IF v_count = 7 THEN
        RAISE NOTICE '[PASS] schema_migrations confirma que os 7 scripts (00-06) já rodaram';
    ELSE
        RAISE WARNING '[FAIL] só % de 7 scripts (00-06) estão registrados em schema_migrations — algum não rodou ainda', v_count;
    END IF;

    RAISE NOTICE '=== fim da bateria de testes ===';
END
$$;

INSERT INTO schema_migrations (version, description)
VALUES ('08_testes', 'Bateria de testes automatizados (SQL puro) rodada para validar a migração')
ON CONFLICT (version) DO NOTHING;
