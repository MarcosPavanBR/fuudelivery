-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 04 — Domínio de chat (chat_api)
-- ============================================================================
-- DIAGNÓSTICO: 100% MongoDB hoje (struct ChatMessage). É o domínio mais
-- simples de migrar — sem duplicação, sem lógica financeira.
-- ============================================================================

CREATE TABLE IF NOT EXISTS chat_messages (
    id            BIGSERIAL PRIMARY KEY,
    order_id      VARCHAR(100) NOT NULL,
    sender_id     BIGINT NOT NULL,
    sender_type   VARCHAR(20) NOT NULL,     -- client | establishment | delivery_man
    sender_name   VARCHAR(255),
    message       TEXT NOT NULL,
    message_type  VARCHAR(20) NOT NULL DEFAULT 'text',  -- text | image
    image_url     TEXT,
    read_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_order ON chat_messages (order_id);
CREATE INDEX IF NOT EXISTS idx_chat_sender ON chat_messages (sender_id, sender_type);
CREATE INDEX IF NOT EXISTS idx_chat_unread ON chat_messages (order_id) WHERE read_at IS NULL;

COMMENT ON TABLE chat_messages IS
    'Mensagens de chat por pedido. Substitui a collection MongoDB do '
    'chat_api (struct ChatMessage). Sem dado financeiro, migração de baixo '
    'risco — bom domínio para testar o processo de corte antes de mexer '
    'em pagamentos.';

INSERT INTO schema_migrations (version, description)
VALUES ('04_dominio_chat', 'Cria chat_messages em Postgres, substituindo a collection MongoDB do chat_api')
ON CONFLICT (version) DO NOTHING;
