-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 03 — Domínio de pagamentos (payment_api + Backend/Payment)
-- ============================================================================
-- DIAGNÓSTICO — o ponto mais crítico da consolidação:
--   Hoje existem DOIS bancos de pagamento MongoDB, escritos por dois
--   serviços Go diferentes, sincronizados por fila Redis:
--     • payment_api  -> struct Payment (foco: cobrança PIX/cartão, gateway
--       Mercado Pago/AbacatePay) e struct Wallet (saldo simples).
--     • Backend/Payment -> struct Payment (foco: aprovação manual/risco/
--       compliance), Wallet (com campos extras: currency, status),
--       WalletTransaction (histórico de crédito/débito), Chargeback,
--       Evidence, PayoutRequest, ApprovalRules, User (admin/operador).
--
--   Os dois códigos falam do MESMO pagamento e da MESMA carteira, cada um
--   com metade dos campos. O código do próprio projeto documenta: "se
--   Redis não estiver configurado, a mensagem é ignorada silenciosamente"
--   — ou seja, existe hoje um caminho real de saldo não creditado na
--   carteira do restaurante/entregador sem nenhum erro visível.
--
--   Este script cria UMA tabela "payments" (superset dos dois structs) e
--   UMA "wallets" + "wallet_transactions", eliminando a necessidade da
--   fila de sincronização entre dois bancos — é tudo a mesma tabela, a
--   mesma transação SQL.
-- ============================================================================

-- ---------- payments (unifica payment_api.Payment + Backend/Payment.Payment) --
CREATE TABLE IF NOT EXISTS payments (
    id                          BIGSERIAL PRIMARY KEY,
    order_id                    VARCHAR(100) NOT NULL,

    customer_id                 BIGINT NOT NULL,
    customer_name               VARCHAR(255),
    customer_email              VARCHAR(255),
    customer_phone              VARCHAR(30),

    establishment_id            BIGINT NOT NULL,
    establishment_name          VARCHAR(255),

    amount                      NUMERIC(12,2) NOT NULL,
    delivery_amount             NUMERIC(12,2) DEFAULT 0,

    method                      VARCHAR(20) NOT NULL,   -- pix | card
    status                      VARCHAR(20) NOT NULL DEFAULT 'pending',
        -- pending | approved | rejected | cancelled | refunded | disputed

    -- risco / aprovação manual (vindo de Backend/Payment)
    risk_level                  VARCHAR(20),             -- low | medium | high | critical
    risk_score                  NUMERIC(5,2),
    requires_approval           BOOLEAN NOT NULL DEFAULT false,
    approved_by                 VARCHAR(255),
    approved_at                 TIMESTAMPTZ,
    rejected_by                 VARCHAR(255),
    rejected_at                 TIMESTAMPTZ,
    rejection_reason            TEXT,

    -- dados de gateway / cobrança (vindo de payment_api)
    pix_qr_code                 TEXT,
    pix_copy_paste               TEXT,
    qr_code_base64               TEXT,
    ticket_url                   TEXT,
    mp_payment_id                BIGINT,
    mp_status                    VARCHAR(30),
    abacatepay_id                VARCHAR(100),
    gateway_status                VARCHAR(50),
    card_last_digits              VARCHAR(4),
    card_token                    TEXT,                  -- ⚠ ver nota de segurança no final do arquivo
    installments                  INT,
    reference                     VARCHAR(100),
    metadata                      JSONB DEFAULT '{}'::jsonb,

    -- créditos internos (quando o valor efetivamente vira saldo em wallet)
    confirmed_at                   TIMESTAMPTZ,
    wallet_credited_at             TIMESTAMPTZ,
    establishment_credited_at      TIMESTAMPTZ,
    refunded_at                    TIMESTAMPTZ,

    split_rules                    JSONB DEFAULT '[]'::jsonb,  -- [{receiver_id, receiver_type, amount, percentage}]

    created_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payments_order ON payments (order_id);
CREATE INDEX IF NOT EXISTS idx_payments_customer ON payments (customer_id);
CREATE INDEX IF NOT EXISTS idx_payments_establishment ON payments (establishment_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments (status);

COMMENT ON TABLE payments IS
    'Tabela única de pagamentos, substituindo as duas collections MongoDB '
    'duplicadas (payment_api.payments e Backend/Payment.payments). '
    'Superset de campos dos dois structs Go originais.';
COMMENT ON COLUMN payments.card_token IS
    'Token do gateway de cartão (nunca o número do cartão). Mesmo assim, '
    'nunca deve ser retornado em resposta de API para o front — trate como '
    'segredo. Ver política de mascaramento em audit_redacted_columns (05).';

DROP TRIGGER IF EXISTS trg_payments_updated_at ON payments;
CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ---------- wallets (unifica payment_api.Wallet + Backend/Payment.Wallet) -----
CREATE TABLE IF NOT EXISTS wallets (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    user_type    VARCHAR(20) NOT NULL,        -- restaurant | delivery
    balance      NUMERIC(12,2) NOT NULL DEFAULT 0,
    currency     VARCHAR(10) NOT NULL DEFAULT 'BRL',
    status       VARCHAR(20) NOT NULL DEFAULT 'active',  -- active | frozen | closed
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, user_type)
);

DROP TRIGGER IF EXISTS trg_wallets_updated_at ON wallets;
CREATE TRIGGER trg_wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

-- ---------- wallet_transactions (só existia em Backend/Payment) --------------
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id              BIGSERIAL PRIMARY KEY,
    wallet_id       BIGINT NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
    type            VARCHAR(10) NOT NULL,     -- credit | debit
    amount          NUMERIC(12,2) NOT NULL,
    balance_before  NUMERIC(12,2) NOT NULL,
    balance_after   NUMERIC(12,2) NOT NULL,
    description     TEXT,
    reference_id    VARCHAR(100),             -- ex: payment_id
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wallet_tx_wallet ON wallet_transactions (wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_tx_reference ON wallet_transactions (reference_id);

COMMENT ON TABLE wallet_transactions IS
    'Histórico imutável de crédito/débito em carteiras. Nunca fazer UPDATE '
    'ou DELETE aqui — só INSERT. É o registro contábil; se precisar '
    'estornar, insere uma transação de sinal contrário, nunca apaga.';

-- ---------- chargebacks / evidence / payout_requests / regras / admins -------
CREATE TABLE IF NOT EXISTS chargebacks (
    id                BIGSERIAL PRIMARY KEY,
    payment_id        BIGINT NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    payment_order_id  VARCHAR(100),
    customer_id       BIGINT NOT NULL,
    establishment_id  BIGINT NOT NULL,
    amount            NUMERIC(12,2) NOT NULL,
    reason            VARCHAR(30) NOT NULL,   -- unauthorized | not_received | defective | duplicate | other
    description       TEXT,
    status            VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending | approved | rejected | escalated
    evidence_count    INT NOT NULL DEFAULT 0,
    assigned_to       VARCHAR(255),
    assigned_at       TIMESTAMPTZ,
    resolved_by       VARCHAR(255),
    resolved_at       TIMESTAMPTZ,
    resolution        TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chargebacks_payment ON chargebacks (payment_id);
CREATE INDEX IF NOT EXISTS idx_chargebacks_status ON chargebacks (status);

DROP TRIGGER IF EXISTS trg_chargebacks_updated_at ON chargebacks;
CREATE TRIGGER trg_chargebacks_updated_at
    BEFORE UPDATE ON chargebacks
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TABLE IF NOT EXISTS chargeback_evidence (
    id             BIGSERIAL PRIMARY KEY,
    chargeback_id  BIGINT NOT NULL REFERENCES chargebacks(id) ON DELETE CASCADE,
    type           VARCHAR(20) NOT NULL,     -- screenshot | document | photo | text
    content        TEXT NOT NULL,
    file_name      VARCHAR(255),
    file_url       TEXT,
    uploaded_by    VARCHAR(255) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_evidence_chargeback ON chargeback_evidence (chargeback_id);

CREATE TABLE IF NOT EXISTS payout_requests (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    user_type        VARCHAR(20) NOT NULL,     -- restaurant | delivery
    amount           NUMERIC(12,2) NOT NULL,
    pix_key          VARCHAR(255) NOT NULL,
    pix_key_type     VARCHAR(10) NOT NULL,     -- CPF | CNPJ | EMAIL | PHONE | EVP
    status           VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending | processing | completed | failed
    gateway_id       VARCHAR(100),
    failure_reason   TEXT,
    balance_before   NUMERIC(12,2) NOT NULL,
    balance_after    NUMERIC(12,2) NOT NULL,
    transaction_id   BIGINT REFERENCES wallet_transactions(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payout_user ON payout_requests (user_id, user_type);
CREATE INDEX IF NOT EXISTS idx_payout_status ON payout_requests (status);

DROP TRIGGER IF EXISTS trg_payout_updated_at ON payout_requests;
CREATE TRIGGER trg_payout_updated_at
    BEFORE UPDATE ON payout_requests
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE TABLE IF NOT EXISTS payment_approval_rules (
    id                            INT PRIMARY KEY DEFAULT 1,   -- linha única (regra global)
    auto_approve_max_amount       NUMERIC(12,2) NOT NULL DEFAULT 100.00,
    auto_approve_max_risk         NUMERIC(5,2)  NOT NULL DEFAULT 39,
    manual_review_min_amount      NUMERIC(12,2) NOT NULL DEFAULT 100.01,
    manual_review_min_risk        NUMERIC(5,2)  NOT NULL DEFAULT 40,
    compliance_min_risk           NUMERIC(5,2)  NOT NULL DEFAULT 90,
    block_chargeback_active       BOOLEAN NOT NULL DEFAULT true,
    block_max_daily_withdrawals   INT NOT NULL DEFAULT 3,
    updated_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_single_row CHECK (id = 1)
);
INSERT INTO payment_approval_rules (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

DROP TRIGGER IF EXISTS trg_approval_rules_updated_at ON payment_approval_rules;
CREATE TRIGGER trg_approval_rules_updated_at
    BEFORE UPDATE ON payment_approval_rules
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

COMMENT ON TABLE payment_approval_rules IS
    'Regras globais de aprovação automática/manual de pagamentos. '
    'Tabela de UMA linha só (id sempre 1) — não crie uma linha por evento, '
    'é configuração, não histórico.';

-- admins/operadores do painel de pagamentos (Backend/Payment.User)
CREATE TABLE IF NOT EXISTS payment_admin_users (
    id            BIGSERIAL PRIMARY KEY,
    email         VARCHAR(255) NOT NULL UNIQUE,
    name          VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,            -- bcrypt, nunca texto puro
    role          VARCHAR(20) NOT NULL DEFAULT 'operator',  -- admin | operator
    active        BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS trg_payment_admin_updated_at ON payment_admin_users;
CREATE TRIGGER trg_payment_admin_updated_at
    BEFORE UPDATE ON payment_admin_users
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

INSERT INTO schema_migrations (version, description)
VALUES ('03_dominio_pagamentos', 'Unifica payment_api + Backend/Payment (payments, wallets, wallet_transactions, chargebacks, evidence, payouts, regras, admins) em um só schema Postgres')
ON CONFLICT (version) DO NOTHING;

-- ----------------------------------------------------------------------------
-- NOTA DE SEGURANÇA — leia antes de aplicar em produção:
--   1. card_token: nunca é o número do cartão (isso violaria PCI-DSS), mas
--      ainda assim é sensível. Nenhuma rota de API deve devolver esta
--      coluna. Confirme isso no código antes do go-live.
--   2. A migração dos dados HISTÓRICOS dos dois Mongos para estas tabelas
--      precisa reconciliar registros que hoje representam O MESMO
--      pagamento em dois bancos (o campo de ligação provável é order_id).
--      Isso é trabalho de ETL cuidadoso, não deste script — este script só
--      cria o destino. Peça o script de migração de dados como próximo
--      passo, depois de validar este schema em homologação.
-- ----------------------------------------------------------------------------
