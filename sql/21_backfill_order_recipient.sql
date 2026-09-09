-- ============================================================================
-- 21_backfill_order_recipient.sql
--
-- Preenche as colunas tipadas establishment_id/user_phone de
-- order_documents a partir do JSONB payload (payload->establishmentId,
-- payload->user->phone) para os pedidos que ficaram NULL.
--
-- POR QUE: a cobranca (payment_api/bindRecipientToOrder ->
-- lookupOrderRecipient) le o destinatario das COLUNAS TIPADAS, nao do
-- payload. Pedidos criados antes da extracao das colunas tem payload
-- completo mas colunas NULL — para esses, toda cobranca PIX responde 400
-- "Pedido invalido para cobranca" (falso bloqueio). O backfill destrava a
-- cobranca desses pedidos sem mudar uma linha de codigo.
--
-- Onde existe a coluna: o corte 5 (order_documents) ja criou as colunas
-- tipadas; o AutoMigrate do GORM tambem as cria em qualquer banco novo.
-- Mesmo assim, este script garante a existencia (ADD COLUMN IF NOT EXISTS)
-- antes do backfill.
--
-- FONTE DA VERDADE: payload->>'establishmentId' e payload->'user'->>'phone'
-- (dto.RequestPayload: EstablishmentId int64 `json:"establishmentId"`,
-- User.Phone `json:"phone"`). O backfill NAO sobrescreve coluna ja
-- populada — so NULL/0.
--
-- Idempotente: pode rodar N vezes; reexecucao so preenche o que segue NULL.
-- ============================================================================

-- 0. Defesa: garante que as colunas tipadas existem.
ALTER TABLE order_documents ADD COLUMN IF NOT EXISTS establishment_id BIGINT;
ALTER TABLE order_documents ADD COLUMN IF NOT EXISTS user_phone VARCHAR(32);

-- 1. Backfill do establishment_id (somente onde NULL ou 0 — nunca sobrescreve).
UPDATE order_documents
   SET establishment_id = (payload->>'establishmentId')::bigint
 WHERE (establishment_id IS NULL OR establishment_id = 0)
   AND payload->>'establishmentId' IS NOT NULL
   AND payload->>'establishmentId' ~ '^[0-9]+$'
   AND (payload->>'establishmentId')::bigint > 0;

-- 2. Backfill do user_phone (somente onde vazio — nunca sobrescreve).
UPDATE order_documents
   SET user_phone = payload->'user'->>'phone'
 WHERE (user_phone IS NULL OR user_phone = '')
   AND payload->'user'->>'phone' IS NOT NULL
   AND payload->'user'->>'phone' <> '';

-- 3. Verificacao: pedidos que CONTINUAM sem destinatario conhecido.
--    Estes seguem recusando cobranca (correto — nao ha para quem creditar),
--    mas a contagem precisa ser conhecida do lado da operacao.
--
--    SELECT count(*) AS sem_destinatario
--      FROM order_documents
--     WHERE (establishment_id IS NULL OR establishment_id = 0);

INSERT INTO schema_migrations (version, description)
VALUES ('21', 'backfill_order_recipient')
ON CONFLICT DO NOTHING;
