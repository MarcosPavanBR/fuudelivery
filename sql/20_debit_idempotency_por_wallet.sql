-- ============================================================================
-- 20_debit_idempotency_por_wallet.sql
-- Escopo correto do índice de idempotência de débito: por CARTEIRA.
--
-- Problema: sql/18 criou
--     UNIQUE (reference_id) WHERE type = 'debit' AND reference_id <> ''
-- que é único no BANCO INTEIRO. Duas carteiras diferentes debitando a mesma
-- referência legítima (por exemplo, o mesmo pedido gerando um débito para o
-- estabelecimento e outro para o entregador) colidiriam: o segundo débito,
-- válido, viraria "duplicado" e seria recusado.
--
-- Não é bug ativo hoje. As referências passaram a ser namespaced pelo dono no
-- Go (`wd:<establishment_id>:<chave>` para saque, `ord:<user_id>:<order_id>`
-- para dedução — ver handlers/wallet.go), então o dono já está DENTRO da
-- string e a colisão entre carteiras não acontece na prática. Isto aqui é
-- corrigir a FORMA: o namespacing é convenção do código e some se alguém
-- escrever um débito novo sem ele; o escopo por wallet_id é garantia do banco
-- e vale para qualquer chamador, inclusive os que ainda não existem.
--
-- Ordem das operações: CRIA o índice novo ANTES de derrubar o antigo. Nunca
-- ficar, nem por um instante, sem proteção contra duplo débito num banco que
-- está recebendo tráfego.
--
-- Idempotente: CREATE UNIQUE INDEX IF NOT EXISTS + DROP INDEX IF EXISTS.
-- ============================================================================

BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_txns_debit_ref_wallet
    ON wallet_transactions (wallet_id, reference_id)
    WHERE type = 'debit' AND reference_id <> '';

DROP INDEX IF EXISTS uq_wallet_txns_debit_ref;

COMMIT;

-- Verificação: deve listar uq_wallet_txns_debit_ref_wallet e NÃO listar
-- uq_wallet_txns_debit_ref.
--
--   SELECT indexname FROM pg_indexes
--    WHERE tablename = 'wallet_transactions'
--      AND indexname LIKE 'uq_wallet_txns_debit%';

INSERT INTO schema_migrations (version, description)
VALUES ('20', 'debit_idempotency_por_wallet')
ON CONFLICT DO NOTHING;
