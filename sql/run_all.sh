#!/usr/bin/env bash
# ============================================================================
# FUUDELIVERY — Consolidação para banco único
# Runner: aplica os scripts 00-08 na ordem certa contra o Supabase/Postgres.
#
# USO:
#   export DB_CONNECTION_STRING="postgresql://...."   # a mesma variável que
#                                                       # o backend Go usa
#   ./sql/run_all.sh              # aplica 00-08 (schema + RLS + testes)
#   ./sql/run_all.sh --so-testes  # roda só a auditoria (07) e os testes (08),
#                                  # sem alterar nada — seguro rodar quando
#                                  # quiser, a qualquer momento
# ============================================================================
set -euo pipefail

if [ -z "${DB_CONNECTION_STRING:-}" ]; then
    echo "ERRO: defina DB_CONNECTION_STRING antes de rodar." >&2
    echo "Ex: export DB_CONNECTION_STRING=\"postgresql://user:pass@host:5432/postgres\"" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

run() {
    echo ""
    echo "=== Aplicando: $1 ==="
    psql "$DB_CONNECTION_STRING" -v ON_ERROR_STOP=1 -f "$SCRIPT_DIR/$1"
}

if [ "${1:-}" = "--so-testes" ]; then
    run "07_auditoria_tabelas_orfas.sql"
    run "08_testes.sql"
    echo ""
    echo "Auditoria e testes concluídos. Nenhuma alteração de schema foi feita."
    exit 0
fi

echo "############################################################"
echo "# ATENÇÃO: isto vai alterar o schema do banco em:"
echo "#   $DB_CONNECTION_STRING"
echo "# Rode primeiro em homologação. Confirme que você tem um"
echo "# backup recente do Supabase antes de rodar em produção."
echo "############################################################"
read -r -p "Digite 'sim' para continuar: " confirm
if [ "$confirm" != "sim" ]; then
    echo "Cancelado."
    exit 1
fi

run "00_role_e_controle_migracoes.sql"
# 09 é um reparo que DEVE rodar ANTES dos scripts de domínio: ele renomeia
# tabelas legadas vazias com id TEXT que, se existirem, fariam o CREATE TABLE
# IF NOT EXISTS dos scripts 01–03 preservar o schema errado (bug real visto em
# produção — ver cabeçalho do próprio 09).
run "09_reparo_tabelas_legado_texto.sql"
run "01_dominio_pedidos.sql"
run "02_dominio_entrega.sql"
run "03_dominio_pagamentos.sql"
run "04_dominio_chat.sql"
run "05_audit_log.sql"
run "06_rls_seguranca.sql"
run "10_wallet_ledger_kind.sql"
run "07_auditoria_tabelas_orfas.sql"
run "08_testes.sql"

echo ""
echo "=== Concluído. Revise a saída acima: procure por qualquer linha"
echo "    com [FAIL] ou ERROR antes de considerar a migração pronta. ==="
