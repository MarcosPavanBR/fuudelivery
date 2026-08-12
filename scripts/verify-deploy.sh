#!/usr/bin/env bash
# verify-deploy.sh — Post-deployment health check for all FuuDelivery services
# Includes retry logic (3 attempts, 10s interval) for Render free-tier cold starts
set -euo pipefail

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
TIMEOUT=60  # Render free-tier cold starts can take 30-60s
RETRIES=3
RETRY_INTERVAL=10

# ─── API Health ──────────────────────────────────────────────
echo "── API (fuudelivery-api) ──"
API_HEALTH=$(retry $RETRIES $RETRY_INTERVAL curl -s --max-time $TIMEOUT "https://fuudelivery-api-8y6l.onrender.com/health" || echo '{"status":"error"}')
API_STATUS=$(echo "$API_HEALTH" | grep -o '"status":"[^"]*"' | tail -1 | cut -d'"' -f4)
if [ "$API_STATUS" = "ok" ]; then
    ok "API is healthy"
    # Check individual components
    for component in mongodb postgres redis redis_geo batches; do
        COMP_STATUS=$(echo "$API_HEALTH" | grep -o "\"$component\":{[^}]*}" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        if [ "$COMP_STATUS" = "up" ]; then
            ok "  $component: up"
        elif [ -n "$COMP_STATUS" ]; then
            warn "  $component: $COMP_STATUS"
        fi
    done
else
    fail "API is down (status: $API_STATUS)"
    FAILURES=$((FAILURES + 1))
fi

# ─── Payment routes (no monolith — isolated service removed) ─
echo ""
echo "── Payment routes (monolith) ──"
PAY_HTTP=$(retry 3 10 curl -s --max-time 15 -o /dev/null -w "%{http_code}" "https://fuudelivery-api-8y6l.onrender.com/payments/all" || echo "000")
if [ "$PAY_HTTP" = "401" ] || [ "$PAY_HTTP" = "403" ]; then
    ok "Payment routes responding (HTTP $PAY_HTTP, auth required)"
else
    warn "Payment routes: HTTP $PAY_HTTP (esperado 401/403 sem token)"
fi

# ─── Static Sites ────────────────────────────────────────────
echo ""
echo "── Static Sites ──"
for site in "WebRestaurant:https://fuudelivery-web.onrender.com" "WebAdmin:https://fuudelivery-admin-lv7f.onrender.com"; do
    NAME=$(echo "$site" | cut -d: -f1)
    URL=$(echo "$site" | cut -d: -f2-)
    HTTP_CODE=$(retry 3 10 curl -s --max-time 15 -o /dev/null -w "%{http_code}" "$URL" || echo "000")
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
