#!/usr/bin/env bash
# verify-deploy.sh — Post-deployment health check for all FuuDelivery services
set -euo pipefail

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
ok() { echo -e "${GREEN}✔${NC}  $*"; }
fail() { echo -e "${RED}✘${NC}  $*"; }
warn() { echo -e "${YELLOW}⚠${NC}  $*"; }

echo ""
echo "╔══════════════════════════════════════════╗"
echo "║  FuuDelivery — Production Health Check   ║"
echo "╚══════════════════════════════════════════╝"
echo ""

FAILURES=0
TIMEOUT=60  # Render free-tier cold starts can take 30-60s

# API Health (with detailed checks)
echo "── API (fuudelivery-api) ──"
# Warmup: first request may trigger cold start
curl -s --max-time $TIMEOUT -o /dev/null https://fuudelivery-api-8y6l.onrender.com/ 2>/dev/null || true
API_HEALTH=$(curl -s --max-time $TIMEOUT https://fuudelivery-api-8y6l.onrender.com/health 2>/dev/null || echo '{"status":"error"}')
API_STATUS=$(echo "$API_HEALTH" | grep -o '"status":"[^"]*"' | tail -1 | cut -d'"' -f4)
if [ "$API_STATUS" = "ok" ]; then
    ok "API is healthy"
    # Check individual components
    for component in mongodb postgres redis; do
        COMP_STATUS=$(echo "$API_HEALTH" | grep -o "\"$component\":{[^}]*}" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        if [ "$COMP_STATUS" = "up" ]; then
            ok "  $component: up"
        else
            warn "  $component: $COMP_STATUS"
        fi
    done
else
    fail "API is down (status: $API_STATUS)"
    FAILURES=$((FAILURES + 1))
fi

# Payment Service
echo ""
echo "── Payment Service ──"
# Warmup: first request may trigger cold start
curl -s --max-time $TIMEOUT -o /dev/null https://fuudelivery-payment.onrender.com/ 2>/dev/null || true
PAY_HEALTH=$(curl -s --max-time $TIMEOUT https://fuudelivery-payment.onrender.com/health 2>/dev/null || echo '{"status":"error"}')
PAY_STATUS=$(echo "$PAY_HEALTH" | grep -o '"status":"[^"]*"' | tail -1 | cut -d'"' -f4)
if [ "$PAY_STATUS" = "ok" ]; then
    ok "Payment Service is healthy"
else
    fail "Payment Service is down (status: $PAY_STATUS)"
    FAILURES=$((FAILURES + 1))
fi

# Static sites
echo ""
echo "── Static Sites ──"
for site in "WebRestaurant:https://fuudelivery-web.onrender.com" "WebAdmin:https://fuudelivery-admin-lv7f.onrender.com" "PaymentPanel:https://fuudelivery-payment-panel.onrender.com"; do
    # Warmup static sites (fast, but include for consistency)
    curl -s --max-time 10 -o /dev/null "$(echo $site | cut -d: -f2-)" 2>/dev/null || true
    NAME=$(echo "$site" | cut -d: -f1)
    URL=$(echo "$site" | cut -d: -f2-)
    HTTP_CODE=$(curl -s --max-time 10 -o /dev/null -w "%{http_code}" "$URL" 2>/dev/null || echo "000")
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
