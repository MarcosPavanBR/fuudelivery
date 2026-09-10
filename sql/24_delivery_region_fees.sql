-- ============================================================================
-- 24_delivery_region_fees.sql
-- Frete por REGIÃO (faixa de CEP), decidido no servidor.
--
-- O que isto corrige, em duas frentes:
--
-- 1. DINHEIRO. Até aqui o preço do frete saía de
--    `(distance * per_km) + fixed_taxa`, e `distance` vinha no CORPO da
--    requisição de criação do pedido (dto.RequestPayload.Distance). Mandar
--    "distance": 0 pagava só a taxa fixa, em qualquer pedido, sempre. O valor
--    era recalculado no servidor, mas a partir de uma entrada escolhida pelo
--    cliente.
--
-- 2. NEGÓCIO. A distância vinha do GPS do celular no momento do pedido
--    (Location.getCurrentPositionAsync), não do endereço de entrega — quem
--    pedia do trabalho para entregar em casa era cobrado pela distância até o
--    trabalho.
--
-- Faixa de CEP resolve as duas: o número que decide o preço é o endereço para
-- onde a comida vai, e não exige geocodificação (o CEP já chega preenchido
-- pelo ViaCEP no app).
--
-- cep_start/cep_end são INTEIROS de propósito. Como texto a comparação seria
-- lexicográfica e "01000000" < "9" daria verdadeiro — a faixa casaria errado.
--
-- Idempotente: CREATE TABLE IF NOT EXISTS + ADD CONSTRAINT dentro de DO.
-- ============================================================================

CREATE TABLE IF NOT EXISTS delivery_region_fees (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(100)  NOT NULL,
    cep_start  INTEGER       NOT NULL,
    cep_end    INTEGER       NOT NULL,
    city       VARCHAR(100)  NOT NULL DEFAULT '',
    uf         VARCHAR(2)    NOT NULL DEFAULT '',
    fee        NUMERIC(12,2) NOT NULL,
    priority   INTEGER       NOT NULL DEFAULT 100,
    active     BOOLEAN       NOT NULL DEFAULT TRUE
);

-- A busca é sempre "qual faixa contém este CEP". Sem índice, cada pedido faz
-- varredura na tabela inteira.
CREATE INDEX IF NOT EXISTS idx_delivery_region_fees_faixa
    ON delivery_region_fees (cep_start, cep_end)
    WHERE active;

-- Travas no BANCO, não só no Go: quem editar direto no Postgres (script,
-- correção manual) não consegue criar regra que o resolvedor não saiba tratar.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'delivery_region_fees_faixa_check') THEN
        ALTER TABLE delivery_region_fees
            ADD CONSTRAINT delivery_region_fees_faixa_check
            CHECK (cep_start BETWEEN 0 AND 99999999
               AND cep_end   BETWEEN 0 AND 99999999
               AND cep_start <= cep_end);
    END IF;

    -- Frete negativo viraria crédito para o cliente no split.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'delivery_region_fees_fee_check') THEN
        ALTER TABLE delivery_region_fees
            ADD CONSTRAINT delivery_region_fees_fee_check
            CHECK (fee >= 0);
    END IF;
END $$;

INSERT INTO schema_migrations (version, description)
VALUES ('24', 'delivery_region_fees')
ON CONFLICT DO NOTHING;
