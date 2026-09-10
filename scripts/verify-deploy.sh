#!/usr/bin/env bash
# verify-deploy.sh — Post-deployment health check for all FuuDelivery services
#
# O check da API espera pela CONDIÇÃO (status terminal no /health), não pela
# falha do curl: o cold start do Render responde 200 com "starting" e precisa
# ser aguardado, não cravado como queda. Ver o comentário longo na seção da API.
#
# URLs vêm de env var para o teste poder apontar tudo para um servidor local
# (scripts/verify-deploy_e2e_test.sh) sem tocar em produção.
set -euo pipefail

# Leitura do /health mora em lib/ para ter teste próprio, sem rede:
# scripts/verify-deploy_test.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/health-parse.sh"

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
ok() { echo -e "${GREEN}✔${NC}  $*"; }
fail() { echo -e "${RED}✘${NC}  $*"; }
warn() { echo -e "${YELLOW}⚠${NC}  $*"; }

# Retry logic: try a command up to $1 times with $2 seconds between attempts
# Usage: retry COUNT INTERVAL command args...
retry() {
    local count=$1; shift
    local interval=$1; shift
    local attempt=1
    while [ $attempt -le $count ]; do
        if output=$("$@" 2>&1); then
            echo "$output"
            return 0
        fi
        if [ $attempt -lt $count ]; then
            warn "  Attempt $attempt/$count failed, retrying in ${interval}s..."
            sleep "$interval"
        fi
        attempt=$((attempt + 1))
    done
    echo "$output"
    return 1
}

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║  FuuDelivery — Production Health Check   ║"
echo "╚══════════════════════════════════════════╝"
echo ""

FAILURES=0
TIMEOUT=${TIMEOUT:-60}  # Render free-tier cold starts can take 30-60s
RETRIES=${RETRIES:-3}
RETRY_INTERVAL=${RETRY_INTERVAL:-10}

# ─── API Health ──────────────────────────────────────────────
# A espera aqui é pela CONDIÇÃO (status terminal no corpo), não pela falha do
# curl — que é o que a função retry() acima faz, e o motivo de este monitor ter
# ficado vermelho em 82 execuções seguidas (runs 486→567, 2026-08-27 a
# 2026-09-10) sem nenhuma queda real.
#
# O que acontecia: o /health devolve 200 com {"status":"starting"} enquanto
# models.DB ainda é nil, de propósito, para o health check do Render passar
# durante a janela de até 125s de conexão dos 5 módulos
# (cmd/fuudelivery/main.go). Como o curl recebe 200 e sai com código 0, o
# retry() nunca disparava: o cold start do free tier respondia "starting" em
# ~14s, o script desistia ali e cravava "API is down". Um monitor sempre
# vermelho não avisa nada quando cai de verdade.
echo "── API (fuudelivery-api) ──"
API_URL="${API_URL:-https://fuudelivery-api-8y6l.onrender.com/health}"
API_BUDGET=${API_BUDGET:-180}   # cold start (~15-60s) + conexão dos bancos (até 125s)
API_INTERVAL=${API_INTERVAL:-10}

API_BODY=""; API_HTTP="000"; API_STATUS=""; API_VERDICT="unusable"
API_DEADLINE=$(( $(date +%s) + API_BUDGET ))
while :; do
    API_RESPONSE=$(curl -s --max-time $TIMEOUT -w $'\n%{http_code}' "$API_URL" 2>/dev/null || true)
    API_HTTP=$(printf '%s' "$API_RESPONSE" | tail -1)
    API_BODY=$(printf '%s' "$API_RESPONSE" | sed '$d')
    API_STATUS=$(health_top_status "$API_BODY")
    API_VERDICT=$(health_verdict "$API_STATUS" "$API_HTTP")

    health_is_terminal "$API_VERDICT" && break
    [ "$(date +%s)" -ge "$API_DEADLINE" ] && break

    warn "  API em '${API_STATUS:-sem resposta}' (HTTP $API_HTTP) — ainda subindo, nova tentativa em ${API_INTERVAL}s"
    sleep "$API_INTERVAL"
done

case "$API_VERDICT" in
    healthy)
        ok "API is healthy (HTTP $API_HTTP)"
        ;;
    degraded)
        # 200 aqui é decisão do handler: Postgres e gateway de pagamento — os
        # críticos — estão de pé, e só Redis/batches degradaram. A plataforma
        # vende normalmente, então isto é aviso, não queda.
        warn "API degradada (HTTP $API_HTTP) — críticos de pé; veja os componentes abaixo"
        ;;
    down)
        fail "API is down (HTTP $API_HTTP) — algum crítico fora do ar"
        FAILURES=$((FAILURES + 1))
        ;;
    *)
        if [ "$API_STATUS" = "starting" ]; then
            fail "API presa em 'starting' após ${API_BUDGET}s — subiu, mas o banco não conectou"
        else
            fail "API sem resposta utilizável (HTTP $API_HTTP, status: '${API_STATUS:-vazio}')"
        fi
        FAILURES=$((FAILURES + 1))
        ;;
esac

# Componentes individuais. mongodb saiu daqui porque saiu do projeto (o Mongo
# foi removido na consolidação para Postgres único); payment_gateways entrou
# porque é um dos dois checks CRÍTICOS e não estava sendo mostrado.
if [ -n "$API_STATUS" ] && [ "$API_STATUS" != "starting" ]; then
    for component in postgres payment_gateways redis redis_geo batches; do
        COMP_STATUS=$(health_check_status "$API_BODY" "$component")
        if [ "$COMP_STATUS" = "up" ]; then
            ok "  $component: up"
        elif [ "$COMP_STATUS" = "down" ]; then
            fail "  $component: down"
        elif [ -n "$COMP_STATUS" ]; then
            warn "  $component: $COMP_STATUS"
        fi
    done
fi

# ─── Payment routes (no monolith — isolated service removed) ─
echo ""
echo "── Payment routes (monolith) ──"
PAY_HTTP=$(retry $RETRIES $RETRY_INTERVAL curl -s --max-time 15 -o /dev/null -w "%{http_code}" "${PAYMENTS_URL:-https://fuudelivery-api-8y6l.onrender.com/payments/all}" || echo "000")
if [ "$PAY_HTTP" = "401" ] || [ "$PAY_HTTP" = "403" ]; then
    ok "Payment routes responding (HTTP $PAY_HTTP, auth required)"
else
    warn "Payment routes: HTTP $PAY_HTTP (esperado 401/403 sem token)"
fi

# ─── Static Sites ────────────────────────────────────────────
echo ""
echo "── Static Sites ──"
for site in "${WEBRESTAURANT_SITE:-WebRestaurant:https://fuudelivery-web.onrender.com}" "${WEBADMIN_SITE:-WebAdmin:https://fuudelivery-admin-lv7f.onrender.com}"; do
    NAME=$(echo "$site" | cut -d: -f1)
    URL=$(echo "$site" | cut -d: -f2-)
    HTTP_CODE=$(retry $RETRIES $RETRY_INTERVAL curl -s --max-time 15 -o /dev/null -w "%{http_code}" "$URL" || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        ok "$NAME: HTTP $HTTP_CODE"
    else
        fail "$NAME: HTTP $HTTP_CODE"
        FAILURES=$((FAILURES + 1))
    fi
done

echo ""
echo "════════════════════════════════════════════"
if [ "$FAILURES" -eq 0 ]; then
    echo -e "${GREEN}All services healthy!${NC}"
else
    echo -e "${RED}$FAILURES service(s) unhealthy${NC}"
fi
echo "════════════════════════════════════════════"
echo ""

exit $FAILURES
