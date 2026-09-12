#!/usr/bin/env bash
# pre-launch-check_test.sh — prova que o verificador sabe dizer NÃO.
#
# POR QUE ESTE TESTE EXISTE
#
# Um verificador que só sabe dizer "verde" é pior que nenhum: ele transforma
# "não olhei" em "está tudo bem" e some com a pergunta. Foi exatamente isso
# que aconteceu com o Monitor Production, que ficou vermelho 82 execuções
# seguidas sem nenhuma queda real — ninguém olhava mais.
#
# Por isso cada checagem da Seção C é FALSIFICADA aqui: derruba a trava no
# banco, exige que o veredito vire BLOQUEADO, recria, exige que volte. Se o
# veredito não mudar, a checagem é decorativa e o teste quebra.
#
# Roda contra um Postgres LOCAL e mexe no schema — nunca aponte para produção.
#
# USO:
#   export TEST_DB_URL="postgresql://postgres:senha@127.0.0.1:5432/postgres"
#   ./scripts/pre-launch-check_test.sh
set -uo pipefail

cd "$(dirname "$0")/.." || exit 1

DB="${TEST_DB_URL:-postgresql://postgres:local@127.0.0.1:5432/postgres}"
CHECK="scripts/pre-launch-check.sh"

case "$DB" in
    *supabase*|*onrender*|*amazonaws*)
        echo "RECUSADO: TEST_DB_URL aponta para um banco remoto." >&2
        echo "Este teste APAGA índices para falsificar as checagens." >&2
        exit 2 ;;
esac

if ! psql "$DB" -tAc "SELECT 1" >/dev/null 2>&1; then
    echo "PULADO: sem Postgres local em $DB" >&2
    echo "  (inicie um Postgres 16 e rode ./sql/run_all.sh antes deste teste)" >&2
    exit 0
fi

PASS=0; FAIL=0
sql() { psql "$DB" -q -v ON_ERROR_STOP=1 -c "$1" >/dev/null 2>&1; }

# veredito_atual roda o verificador só na seção do banco e devolve a palavra.
veredito_atual() {
    local saida
    saida=$(DB_CONNECTION_STRING="$DB" bash "$CHECK" --so-banco 2>&1)
    if echo "$saida" | grep -q "BLOQUEADO"; then echo "BLOQUEADO"
    elif echo "$saida" | grep -q "PRONTO COM RESSALVAS"; then echo "RESSALVAS"
    elif echo "$saida" | grep -q "PRONTO"; then echo "PRONTO"
    else echo "INDEFINIDO"; fi
}

exige() { # exige <veredito esperado> <descrição>
    local obtido; obtido=$(veredito_atual)
    if [ "$obtido" = "$1" ]; then
        PASS=$((PASS + 1)); echo "  ✔ $2"
    else
        FAIL=$((FAIL + 1)); echo "  ✘ $2"
        echo "      esperado: $1"
        echo "      obtido:   $obtido"
    fi
}

# falsifica_indice derruba um índice, exige BLOQUEADO, recria e exige que saia
# do bloqueio. É o par completo: sem a metade da volta, um verificador que
# grita BLOQUEADO para tudo passaria neste teste.
falsifica_indice() { # falsifica_indice <nome> <DDL de recriação> <descrição>
    local nome="$1" ddl="$2" desc="$3"

    if ! psql "$DB" -tAc "SELECT 1 FROM pg_indexes WHERE indexname='$nome'" | grep -q 1; then
        FAIL=$((FAIL + 1))
        echo "  ✘ $desc — pré-condição falhou: $nome não existe antes do teste"
        echo "      (rode ./sql/run_all.sh contra este banco primeiro)"
        return
    fi

    sql "DROP INDEX IF EXISTS $nome"
    # Prova que o mundo mudou de verdade. Falsificação sem esta asserção não
    # prova nada — nesta sessão já houve três que passaram verdes sem nunca
    # terem alterado o alvo.
    if psql "$DB" -tAc "SELECT 1 FROM pg_indexes WHERE indexname='$nome'" | grep -q 1; then
        FAIL=$((FAIL + 1)); echo "  ✘ $desc — o DROP não surtiu efeito, teste inválido"
        return
    fi

    exige BLOQUEADO "$desc: sem $nome → BLOQUEADO"

    sql "$ddl"
    local depois; depois=$(veredito_atual)
    if [ "$depois" = "BLOQUEADO" ]; then
        FAIL=$((FAIL + 1)); echo "  ✘ $desc: recriei $nome e continuou BLOQUEADO"
    else
        PASS=$((PASS + 1)); echo "  ✔ $desc: com $nome de volta → $depois"
    fi
}

echo ""
echo "── Falsificação das travas do dinheiro ──"

falsifica_indice uq_wallet_txns_credit_ref \
    "CREATE UNIQUE INDEX uq_wallet_txns_credit_ref ON wallet_transactions (reference_id) WHERE type = 'credit' AND reference_id <> ''" \
    "crédito idempotente"

falsifica_indice uq_payments_abacatepay_id \
    "CREATE UNIQUE INDEX uq_payments_abacatepay_id ON payments (abacatepay_id) WHERE abacatepay_id IS NOT NULL AND abacatepay_id <> ''" \
    "cobrança única"

echo ""
echo "── Débito: os dois nomes não são intercambiáveis ──"
# Este é o caso que mais fácil viraria um BLOQUEADO FALSO. O sql/20 apaga o
# índice global de propósito e o substitui pelo por-carteira; um verificador
# que exigisse os dois reprovaria todo banco em dia. E um que aceitasse
# qualquer um dos dois em silêncio esconderia que o sql/20 não rodou.
DDL_WALLET="CREATE UNIQUE INDEX uq_wallet_txns_debit_ref_wallet ON wallet_transactions (wallet_id, reference_id) WHERE type = 'debit' AND reference_id <> ''"
DDL_GLOBAL="CREATE UNIQUE INDEX uq_wallet_txns_debit_ref ON wallet_transactions (reference_id) WHERE type = 'debit' AND reference_id <> ''"

sql "DROP INDEX IF EXISTS uq_wallet_txns_debit_ref_wallet"
sql "DROP INDEX IF EXISTS uq_wallet_txns_debit_ref"
exige BLOQUEADO "nenhum dos dois índices de débito → BLOQUEADO"

sql "$DDL_GLOBAL"
exige RESSALVAS "só o índice global antigo → ressalva (funciona, mas o sql/20 não rodou)"

sql "DROP INDEX IF EXISTS uq_wallet_txns_debit_ref"
sql "$DDL_WALLET"
saida_ok=$(DB_CONNECTION_STRING="$DB" bash "$CHECK" --so-banco 2>&1)
if echo "$saida_ok" | grep -q "débito idempotente por carteira"; then
    PASS=$((PASS + 1)); echo "  ✔ com o índice por carteira → aprovado sem ressalva"
else
    FAIL=$((FAIL + 1)); echo "  ✘ índice por carteira presente mas o script não o reconheceu"
fi

echo ""
echo "── Travas do frete por região ──"
ANTES_CHECKS=$(psql "$DB" -tAc "SELECT count(*) FROM pg_constraint WHERE conrelid='delivery_region_fees'::regclass AND contype='c'" | tr -d ' ')
if [ "$ANTES_CHECKS" -ge 2 ]; then
    sql "ALTER TABLE delivery_region_fees DROP CONSTRAINT delivery_region_fees_fee_check"
    saida=$(DB_CONNECTION_STRING="$DB" bash "$CHECK" --so-banco 2>&1)
    if echo "$saida" | grep -q "sem os 2 CHECKs"; then
        PASS=$((PASS + 1)); echo "  ✔ CHECK removido → o script avisa"
    else
        FAIL=$((FAIL + 1)); echo "  ✘ removi um CHECK e o script não notou"
    fi
    sql "ALTER TABLE delivery_region_fees ADD CONSTRAINT delivery_region_fees_fee_check CHECK (fee >= 0)"
    saida=$(DB_CONNECTION_STRING="$DB" bash "$CHECK" --so-banco 2>&1)
    if echo "$saida" | grep -q "travas do frete por região (2 CHECKs)"; then
        PASS=$((PASS + 1)); echo "  ✔ CHECK recriado → volta a aprovar"
    else
        FAIL=$((FAIL + 1)); echo "  ✘ recriei o CHECK e o script continuou reclamando"
    fi
else
    echo "  · pulado: delivery_region_fees sem os CHECKs nesta base"
fi

echo ""
echo "── Dinheiro pendurado é BLOQUEIO, não aviso ──"
# Um pagamento CONFIRMED sem crédito no restaurante há mais de 10 min é a
# assinatura exata da falha que a reconciliação existe para curar. Se ele
# sobrou, a rede de segurança não está dando conta — e isso tem que doer.
sql "INSERT INTO payments (order_id, customer_id, establishment_id, amount, method, status, confirmed_at, abacatepay_id)
     VALUES ('test-pendurado-999', 1, 42, 10.00, 'pix', 'CONFIRMED', now() - interval '30 minutes', 'charge-test-pendurado-999')"
exige BLOQUEADO "pagamento confirmado sem liquidar há 30 min → BLOQUEADO"

sql "UPDATE payments SET establishment_credited_at = now() WHERE order_id = 'test-pendurado-999'"
depois=$(veredito_atual)
if [ "$depois" != "BLOQUEADO" ]; then
    PASS=$((PASS + 1)); echo "  ✔ depois de creditado → $depois"
else
    FAIL=$((FAIL + 1)); echo "  ✘ creditei o pagamento e continuou BLOQUEADO"
fi
sql "DELETE FROM payments WHERE order_id = 'test-pendurado-999'"

echo ""
echo "── Regiões cadastradas: avisa, mas nunca bloqueia ──"
# Região faltando é o caso mais provável no dia do lançamento, e o mais fácil
# de classificar errado nas duas direções: bloquear seria mentira (sem região
# o pedido cai na taxa por km e a venda acontece), e calar seria pior ainda
# (ninguém descobriria que o frete inteiro está vindo do fallback).
sql "DELETE FROM delivery_region_fees"
saida=$(DB_CONNECTION_STRING="$DB" bash "$CHECK" --so-banco 2>&1)
if echo "$saida" | grep -q "nenhuma região cadastrada"; then
    PASS=$((PASS + 1)); echo "  ✔ sem nenhuma região → avisa"
else
    FAIL=$((FAIL + 1)); echo "  ✘ base sem região e o script não avisou"
fi
if echo "$saida" | grep -q "BLOQUEADO"; then
    FAIL=$((FAIL + 1)); echo "  ✘ falta de região virou BLOQUEIO — não deveria, a venda funciona sem ela"
else
    PASS=$((PASS + 1)); echo "  ✔ e não bloqueia o lançamento"
fi

sql "INSERT INTO delivery_region_fees (name, cep_start, cep_end, fee)
     VALUES ('teste-falsificacao', 1000000, 1999999, 7.50)"
saida=$(DB_CONNECTION_STRING="$DB" bash "$CHECK" --so-banco 2>&1)
if echo "$saida" | grep -qE "1 região\(ões\) de frete cadastrada"; then
    PASS=$((PASS + 1)); echo "  ✔ com 1 região cadastrada → o aviso some"
else
    FAIL=$((FAIL + 1)); echo "  ✘ cadastrei uma região e o aviso não sumiu"
    echo "$saida" | grep -i "regi" | sed 's/^/      /'
fi
sql "DELETE FROM delivery_region_fees WHERE name = 'teste-falsificacao'"

echo ""
echo "── 'Não olhei' NÃO pode parecer 'está ok' ──"
# A regra que sustenta o script inteiro. Sem DB_CONNECTION_STRING a Seção C
# não roda; se ela fosse pulada em silêncio, o veredito final diria PRONTO
# sem nunca ter olhado nenhuma trava do dinheiro.
saida=$(env -u DB_CONNECTION_STRING bash "$CHECK" --so-banco 2>&1)
if echo "$saida" | grep -q "NÃO VERIFICADO"; then
    PASS=$((PASS + 1)); echo "  ✔ sem DB_CONNECTION_STRING → imprime NÃO VERIFICADO"
else
    FAIL=$((FAIL + 1)); echo "  ✘ seção C pulada em silêncio"
fi
if echo "$saida" | grep -qE "^\s+.*PRONTO COM RESSALVAS"; then
    PASS=$((PASS + 1)); echo "  ✔ e o veredito não passa de PRONTO COM RESSALVAS"
else
    FAIL=$((FAIL + 1)); echo "  ✘ veredito sem banco verificado não foi rebaixado"
    echo "$saida" | tail -12 | sed 's/^/      /'
fi

# O caminho que quase escapou: --so-repo num repositório limpo não gera
# ressalva nenhuma, então o contador chegaria a zero e o veredito sairia
# PRONTO sem nunca ter olhado uma trava do dinheiro. O verde tem que dizer
# PARCIAL.
saida_repo=$(bash "$CHECK" --so-repo 2>&1)
if echo "$saida_repo" | grep -q "C · Banco"; then
    FAIL=$((FAIL + 1)); echo "  ✘ --so-repo rodou a seção do banco assim mesmo"
else
    PASS=$((PASS + 1)); echo "  ✔ --so-repo não toca no banco"
fi
if echo "$saida_repo" | grep -qE "PRONTO —"; then
    FAIL=$((FAIL + 1)); echo "  ✘ --so-repo declarou PRONTO sem ter olhado o banco"
    echo "$saida_repo" | tail -10 | sed 's/^/      /'
else
    PASS=$((PASS + 1)); echo "  ✔ --so-repo nunca declara PRONTO (no máximo PARCIAL)"
fi

echo ""
echo "════════════════════════════════════════"
if [ "$FAIL" -eq 0 ]; then
    echo "  $PASS passaram, 0 falharam"
    exit 0
else
    echo "  $PASS passaram, $FAIL FALHARAM"
    exit 1
fi
