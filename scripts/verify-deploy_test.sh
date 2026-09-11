#!/usr/bin/env bash
# verify-deploy_test.sh — testes da leitura do /health.
#
# Roda sem rede e sem produção. Executa a suíte DUAS vezes: com jq e com jq
# escondido do PATH, porque o fallback em grep é o que roda na máquina de quem
# não tem jq instalado e era justamente onde ninguém olhava.
set -uo pipefail

cd "$(dirname "$0")/.." || exit 1
source scripts/lib/health-parse.sh

PASS=0; FAIL=0
check() { # check <descrição> <esperado> <obtido>
    if [ "$2" = "$3" ]; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
        echo "  ✘ $1"
        echo "      esperado: '$2'"
        echo "      obtido:   '$3'"
    fi
}

# Corpo real do cold start — é o que produção devolveu em todas as 82 execuções
# vermelhas. Chaves em ordem alfabética porque fiber.Map é map do Go.
STARTING='{"message":"database initializing, please retry","service":"fuudelivery","status":"starting","time":"2026-09-10T18:39:14Z","version":"1.0.0"}'

UP='{"checks":{"batches":{"name":"batches","status":"up","latency":"3ms"},"payment_gateways":{"name":"payment_gateways","status":"up"},"postgres":{"name":"postgres","status":"up","latency":"12ms"},"redis":{"name":"redis","status":"up"},"redis_geo":{"name":"redis_geo","status":"up"}},"service":"fuudelivery","status":"up","time":"2026-09-10T18:39:14Z","version":"1.0.0"}'

DEGRADED='{"checks":{"batches":{"name":"batches","status":"degraded","error":"timeout"},"payment_gateways":{"name":"payment_gateways","status":"up"},"postgres":{"name":"postgres","status":"up","latency":"12ms"},"redis":{"name":"redis","status":"up"},"redis_geo":{"name":"redis_geo","status":"degraded","error":"timeout"}},"service":"fuudelivery","status":"degraded","time":"2026-09-10T18:39:14Z","version":"1.0.0"}'

DOWN='{"checks":{"batches":{"name":"batches","status":"down"},"payment_gateways":{"name":"payment_gateways","status":"down","error":"no gateway configured"},"postgres":{"name":"postgres","status":"down","error":"connection refused"},"redis":{"name":"redis","status":"down"},"redis_geo":{"name":"redis_geo","status":"down"}},"service":"fuudelivery","status":"down","time":"2026-09-10T18:39:14Z","version":"1.0.0"}'

run_suite() {
    local label="$1"
    echo "── $label ──"

    # --- status de topo não pode ser confundido com o de um componente -------
    # Se pegasse o primeiro "status" do JSON, o corpo UP devolveria o do
    # "batches" e um degraded de componente viraria veredicto global.
    check "$label: topo de UP" "up" "$(health_top_status "$UP")"
    check "$label: topo de DEGRADED" "degraded" "$(health_top_status "$DEGRADED")"
    check "$label: topo de DOWN" "down" "$(health_top_status "$DOWN")"
    check "$label: topo de STARTING" "starting" "$(health_top_status "$STARTING")"

    # --- componentes --------------------------------------------------------
    check "$label: postgres em UP" "up" "$(health_check_status "$UP" postgres)"
    check "$label: payment_gateways em DOWN" "down" "$(health_check_status "$DOWN" payment_gateways)"
    check "$label: redis_geo em DEGRADED" "degraded" "$(health_check_status "$DEGRADED" redis_geo)"
    # redis vs redis_geo: prefixo comum não pode casar o componente errado.
    check "$label: redis em DEGRADED (não é o redis_geo)" "up" "$(health_check_status "$DEGRADED" redis)"
    # mongodb saiu do projeto; perguntar por ele devolve vazio, não erro.
    check "$label: mongodb não existe mais" "" "$(health_check_status "$UP" mongodb)"

    # --- veredictos ---------------------------------------------------------
    check "$label: up + 200 = healthy" "healthy" "$(health_verdict up 200)"
    check "$label: degraded + 200 = degraded" "degraded" "$(health_verdict degraded 200)"
    check "$label: down + 503 = down" "down" "$(health_verdict down 503)"
    check "$label: starting + 200 = starting" "starting" "$(health_verdict starting 200)"
    check "$label: sem resposta = unusable" "unusable" "$(health_verdict "" 000)"
    # 200 no corpo mas HTTP errado é resposta que não bate — não é "de pé".
    check "$label: up + 503 = unusable" "unusable" "$(health_verdict up 503)"

    # --- O BUG DE 2026-08-27 ------------------------------------------------
    # "starting" NÃO pode ser terminal: era isso que fazia o cold start do
    # Render (que responde 200 "starting" em ~14s) ser cravado como "API is
    # down" em 82 execuções seguidas.
    if health_is_terminal "$(health_verdict starting 200)"; then
        FAIL=$((FAIL + 1))
        echo "  ✘ $label: 'starting' foi tratado como veredicto final (regressão do bug do cold start)"
    else
        PASS=$((PASS + 1))
    fi

    # "degraded" tampouco pode virar falha: o handler devolve 200 de propósito.
    if health_is_terminal "$(health_verdict degraded 200)"; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
        echo "  ✘ $label: 'degraded' não foi aceito como veredicto"
    fi

    # Sem resposta ainda não é queda — durante um cold start o curl falha antes
    # de o serviço subir.
    if health_is_terminal "unusable"; then
        FAIL=$((FAIL + 1))
        echo "  ✘ $label: 'unusable' foi tratado como veredicto final"
    else
        PASS=$((PASS + 1))
    fi
}

run_suite "com jq"

# Esconde o jq e repete: o fallback em grep depende da ordem alfabética das
# chaves do fiber.Map e precisa ser exercitado de verdade.
JQ_FREE_BIN="$(mktemp -d)"
cat > "$JQ_FREE_BIN/jq" <<'STUB'
#!/usr/bin/env bash
echo "jq nao deveria ser chamado nesta suite" >&2
exit 127
STUB
chmod +x "$JQ_FREE_BIN/jq"
# command -v jq ainda encontra o stub, então o fallback só roda se ele sair do
# PATH de verdade: aqui o PATH é reduzido a um diretório sem jq nenhum.
FAKE_PATH="$(mktemp -d)"
for tool in bash grep cut tail printf sed mktemp; do
    src="$(command -v "$tool" 2>/dev/null)" && [ -n "$src" ] && ln -sf "$src" "$FAKE_PATH/$tool"
done
PATH="$FAKE_PATH" run_suite "sem jq (fallback grep)"

rm -rf "$JQ_FREE_BIN" "$FAKE_PATH"

echo ""
echo "════════════════════════════════════════════"
if [ "$FAIL" -eq 0 ]; then
    echo "OK — $PASS asserções"
else
    echo "FALHOU — $FAIL de $((PASS + FAIL)) asserções"
fi
echo "════════════════════════════════════════════"
[ "$FAIL" -eq 0 ] && exit 0
exit 1
