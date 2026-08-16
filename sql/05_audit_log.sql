-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 05 — Auditoria: registrar CADA mudança de dados (INSERT/UPDATE/DELETE)
-- ============================================================================
-- Isto é o changelog de DADOS (complementa schema_migrations, que é o
-- changelog de ESTRUTURA). Toda vez que uma linha for criada, alterada ou
-- apagada em uma tabela auditada, uma cópia do "antes" e do "depois" fica
-- registrada aqui, com quem fez (role do Postgres) e quando.
--
-- Colunas sensíveis (senha, token, dados de cartão) são automaticamente
-- REDIGIDAS (substituídas por '[REDACTED]') antes de gravar no log — a
-- lista fica na tabela audit_redacted_columns, editável sem mudar código.
-- ============================================================================

CREATE TABLE IF NOT EXISTS audit_log (
    id            BIGSERIAL PRIMARY KEY,
    table_name    TEXT NOT NULL,
    operation     TEXT NOT NULL,          -- INSERT | UPDATE | DELETE
    row_pk        TEXT,                   -- id da linha afetada, como texto
    before_data   JSONB,
    after_data    JSONB,
    changed_by    TEXT NOT NULL DEFAULT current_user,
    changed_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_table ON audit_log (table_name);
CREATE INDEX IF NOT EXISTS idx_audit_changed_at ON audit_log (changed_at);
CREATE INDEX IF NOT EXISTS idx_audit_row_pk ON audit_log (table_name, row_pk);

COMMENT ON TABLE audit_log IS
    'Log imutável de todas as alterações de dados nas tabelas auditadas. '
    'Nunca faça UPDATE/DELETE aqui manualmente. Para consultar o histórico '
    'de uma linha: SELECT * FROM audit_log WHERE table_name = ''payments'' '
    'AND row_pk = ''123'' ORDER BY changed_at;';

CREATE TABLE IF NOT EXISTS audit_redacted_columns (
    table_name  TEXT NOT NULL,
    column_name TEXT NOT NULL,
    PRIMARY KEY (table_name, column_name)
);

INSERT INTO audit_redacted_columns (table_name, column_name) VALUES
    ('users', 'password'),
    ('clients', 'password'),
    ('delivery_men', 'password'),
    ('payment_admin_users', 'password_hash'),
    ('payments', 'card_token'),
    ('payments', 'pix_copy_paste'),
    ('payments', 'qr_code_base64')
ON CONFLICT DO NOTHING;

COMMENT ON TABLE audit_redacted_columns IS
    'Lista de colunas que nunca devem aparecer em texto puro no audit_log. '
    'Adicione uma linha aqui sempre que criar uma coluna nova com senha, '
    'token, segredo ou dado de cartão — não precisa mudar a função abaixo.';

CREATE OR REPLACE FUNCTION fn_audit_trigger()
RETURNS TRIGGER AS $$
DECLARE
    v_before JSONB;
    v_after  JSONB;
    v_pk     TEXT;
    v_col    TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        v_before := to_jsonb(OLD);
        v_after  := NULL;
        v_pk     := (to_jsonb(OLD)->>'id');
    ELSIF TG_OP = 'INSERT' THEN
        v_before := NULL;
        v_after  := to_jsonb(NEW);
        v_pk     := (to_jsonb(NEW)->>'id');
    ELSE
        v_before := to_jsonb(OLD);
        v_after  := to_jsonb(NEW);
        v_pk     := (to_jsonb(NEW)->>'id');
    END IF;

    -- redige colunas sensíveis desta tabela, se houver alguma cadastrada
    FOR v_col IN
        SELECT column_name FROM audit_redacted_columns WHERE table_name = TG_TABLE_NAME
    LOOP
        IF v_before ? v_col THEN
            v_before := jsonb_set(v_before, ARRAY[v_col], '"[REDACTED]"');
        END IF;
        IF v_after ? v_col THEN
            v_after := jsonb_set(v_after, ARRAY[v_col], '"[REDACTED]"');
        END IF;
    END LOOP;

    INSERT INTO audit_log (table_name, operation, row_pk, before_data, after_data)
    VALUES (TG_TABLE_NAME, TG_OP, v_pk, v_before, v_after);

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Anexa o trigger de auditoria a todas as tabelas de negócio relevantes.
-- (schema_migrations, audit_log e audit_redacted_columns ficam de fora —
-- não faz sentido auditar o próprio log.)
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
            EXECUTE format('DROP TRIGGER IF EXISTS trg_audit_%I ON %I;', t, t);
            EXECUTE format(
                'CREATE TRIGGER trg_audit_%I AFTER INSERT OR UPDATE OR DELETE ON %I '
                'FOR EACH ROW EXECUTE FUNCTION fn_audit_trigger();', t, t
            );
        END IF;
        -- tabelas que ainda não existem (porque um script anterior não
        -- rodou) são simplesmente ignoradas aqui — rode 07 para ver o que
        -- falta.
    END LOOP;
END
$$;

INSERT INTO schema_migrations (version, description)
VALUES ('05_audit_log', 'Cria audit_log genérico por trigger com redação de colunas sensíveis, anexado a todas as tabelas de negócio')
ON CONFLICT (version) DO NOTHING;
