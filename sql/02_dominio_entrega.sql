-- ============================================================================
-- FUUDELIVERY — Consolidação para banco único
-- 02 — Domínio de entrega (delivery_api)
-- ============================================================================
-- DIAGNÓSTICO:
--   delivery_api é 100% MongoDB hoje. O documento central é o "OrderDTO"
--   (Backend/delivery_api/app/dto/order.go): uma cópia achatada do pedido
--   (estabelecimento, cliente, produtos, entregador, pagamento) usada só
--   para o motor de despacho (matching por geolocalização/zona/lote).
--   Abaixo, uma tabela relacional equivalente. Mantemos os dados
--   denormalizados (establishment_name, product snapshot etc.) de
--   propósito: o motor de despacho precisa ler rápido, sem JOIN pesado
--   contra orders_api a cada tentativa de match. Isso não é "duplicação
--   ruim" — é o mesmo padrão de read-model que o sistema já usa hoje,
--   só que trocando Mongo por uma tabela Postgres (JSONB para o que é
--   de fato variável/aninhado, colunas normais para o que é filtrado
--   com frequência pelo motor de despacho).
-- ============================================================================

CREATE TABLE IF NOT EXISTS delivery_solicitations (
    id                  BIGSERIAL PRIMARY KEY,
    order_id            VARCHAR(100) NOT NULL UNIQUE,   -- referencia orders.id (orders_api)
    status              VARCHAR(30)  NOT NULL DEFAULT 'pending',

    establishment_id    BIGINT NOT NULL,
    establishment_name  VARCHAR(255),
    establishment_lat   DOUBLE PRECISION,
    establishment_long  DOUBLE PRECISION,
    establishment_address VARCHAR(500),
    establishment_phone VARCHAR(30),
    establishment_image TEXT,

    user_id             BIGINT NOT NULL,
    user_name           VARCHAR(255),
    user_phone          VARCHAR(30),

    delivery_man_id     BIGINT,
    delivery_man_name   VARCHAR(255),
    delivery_man_status VARCHAR(20),

    -- snapshot dos produtos do pedido no momento do despacho (não é a
    -- fonte de verdade — a fonte é order_items em orders_api; isto é
    -- cache de leitura para o motor de matching)
    products            JSONB NOT NULL DEFAULT '[]'::jsonb,

    total               NUMERIC(12,2),
    payment_method      VARCHAR(30),
    payment_change      NUMERIC(12,2),

    zone_id             BIGINT,
    match_radius_km     DOUBLE PRECISION DEFAULT 5.0,
    batch_id            BIGINT,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE delivery_solicitations IS
    'Read-model do pedido usado pelo motor de despacho. Substitui a '
    'collection MongoDB "orders" do delivery_api (struct OrderDTO). '
    'order_id referencia orders.id em orders_api (mesmo Postgres agora, '
    'FK física pode ser adicionada depois de validar que os IDs batem).';

CREATE INDEX IF NOT EXISTS idx_delivery_solic_status ON delivery_solicitations (status);
CREATE INDEX IF NOT EXISTS idx_delivery_solic_zone ON delivery_solicitations (zone_id);
CREATE INDEX IF NOT EXISTS idx_delivery_solic_deliveryman ON delivery_solicitations (delivery_man_id);
CREATE INDEX IF NOT EXISTS idx_delivery_solic_batch ON delivery_solicitations (batch_id);

DROP TRIGGER IF EXISTS trg_delivery_solic_updated_at ON delivery_solicitations;
CREATE TRIGGER trg_delivery_solic_updated_at
    BEFORE UPDATE ON delivery_solicitations
    FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

INSERT INTO schema_migrations (version, description)
VALUES ('02_dominio_entrega', 'Cria delivery_solicitations em Postgres, substituindo a collection MongoDB usada pelo delivery_api')
ON CONFLICT (version) DO NOTHING;
