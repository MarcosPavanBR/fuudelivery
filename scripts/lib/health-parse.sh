#!/usr/bin/env bash
# health-parse.sh — leitura do /health do monólito, separada do script de rede
# para poder ser testada sem depender de produção (verify-deploy_test.sh).
#
# Por que isto existe separado: o monitor de produção ficou VERMELHO em 82
# execuções seguidas (runs 486→567, 2026-08-27 a 2026-09-10) por causa da
# interpretação do corpo, não por causa da rede. Parsing sem teste foi o que
# deixou o alarme mentindo por duas semanas.

# health_top_status <json> — o "status" de TOPO do /health.
#
# Sem jq o fallback usa `tail -1`, e isso não é acaso: o handler devolve um
# fiber.Map (map do Go) e o encoding/json ordena as chaves alfabeticamente, de
# modo que "checks" (que carrega os status aninhados dos componentes) sempre
# vem ANTES de "status". A última ocorrência é, portanto, a de topo.
health_top_status() {
    local body="$1"
    if command -v jq >/dev/null 2>&1; then
        printf '%s' "$body" | jq -r '.status // empty' 2>/dev/null
    else
        printf '%s' "$body" | grep -o '"status":"[^"]*"' | tail -1 | cut -d'"' -f4
    fi
}

# health_check_status <json> <componente> — status de um check individual.
health_check_status() {
    local body="$1" name="$2"
    if command -v jq >/dev/null 2>&1; then
        printf '%s' "$body" | jq -r --arg n "$name" '.checks[$n].status // empty' 2>/dev/null
    else
        printf '%s' "$body" | grep -o "\"$name\":{[^}]*}" | grep -o '"status":"[^"]*"' | cut -d'"' -f4
    fi
}

# health_verdict <status-de-topo> <http-code> — traduz para UMA palavra:
#
#   healthy   tudo de pé
#   degraded  Postgres e gateway (os críticos) de pé, Redis/batches não.
#             O handler devolve 200 aqui DE PROPÓSITO — não é queda.
#   down      crítico fora do ar (o handler devolve 503)
#   starting  models.DB ainda nil; o handler devolve 200 para o health check do
#             Render passar durante a janela de até 125s de conexão. NÃO é
#             veredicto: quem chama deve continuar esperando.
#   unusable  sem resposta, ou resposta que não dá para interpretar
#
# `starting` ser não-terminal é exatamente o bug de 2026-08-27: como o curl
# devolve 200 e sai com código 0, a antiga função retry() (que só repetia em
# falha de transporte) nunca disparava, e o cold start do Render — que responde
# "starting" em ~14s — era cravado como "API is down".
health_verdict() {
    local status="$1" http="$2"
    case "$status" in
        up)
            [ "$http" = "200" ] && echo "healthy" || echo "unusable"
            ;;
        degraded)
            [ "$http" = "200" ] && echo "degraded" || echo "unusable"
            ;;
        down)      echo "down" ;;
        starting)  echo "starting" ;;
        *)         echo "unusable" ;;
    esac
}

# health_is_terminal <veredicto> — 0 quando já dá para decidir, 1 quando ainda
# vale esperar (starting, ou nada respondendo durante um cold start).
health_is_terminal() {
    case "$1" in
        healthy|degraded|down) return 0 ;;
        *)                     return 1 ;;
    esac
}
