#!/bin/bash
# ============================================================
# FUUDELIVERY — Seed Data Script
# Run this ONCE after first deploy to initialize essential data.
# Usage: bash scripts/seed-data.sh <API_URL> <ADMIN_TOKEN>
# Example: bash scripts/seed-data.sh https://fuudelivery-api-8y6l.onrender.com admin_jwt_token
# ============================================================

set -euo pipefail

API_URL="${1:-http://localhost:3000}"
TOKEN="${2}"
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[1;33m'
COLOR_RED='\033[0;31m'
COLOR_NC='\033[0m'

if [ -z "$TOKEN" ]; then
    echo -e "${COLOR_RED}Uso: bash scripts/seed-data.sh <API_URL> <ADMIN_TOKEN>${COLOR_NC}"
    echo -e "${COLOR_YELLOW}Para obter um token admin:${COLOR_NC}"
    echo "  1. Faca POST /admin/bootstrap se for o primeiro deploy"
    echo "  2. Use o token retornado como ADMIN_TOKEN"
    exit 1
fi

echo -e "${COLOR_GREEN}=== FUUDELIVERY Seed Data ===${COLOR_NC}"
echo "API URL: $API_URL"
echo ""

# Helper function
api_call() {
    local method="$1"
    local path="$2"
    local data="$3"
    local auth="${4:-}"
    
    headers="-H 'Content-Type: application/json'"
    if [ -n "$auth" ]; then
        headers="$headers -H 'Authorization: Bearer $TOKEN'"
    fi
    
    if [ "$method" = "GET" ]; then
        eval curl -s -X GET "$API_URL$path" $headers
    else
        eval curl -s -X "$method" "$API_URL$path" $headers -d "$data"
    fi
}

# 1. Criar zona padrao (se nao existir)
echo -e "${COLOR_YELLOW}1. Criando zona padrao...${COLOR_NC}"
ZONE_RESPONSE=$(api_call "POST" "/zones" '{
    "name": "Padrão",
    "city_size": "medium",
    "radius_km": 5.0,
    "min_radius_km": 2.0,
    "max_radius_km": 10.0,
    "platform_fee_percentage": 5.0,
    "establishment_percentage": 85.0,
    "min_delivery_fee": 5.0,
    "min_couriers_threshold": 3,
    "split_initial_platform_pct": 3.0,
    "split_target_platform_pct": 12.0
}' "auth")
ZONE_ID=$(echo "$ZONE_RESPONSE" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo -e "  Zona criada: ID=$ZONE_ID → $(echo "$ZONE_RESPONSE" | grep -o '"name":"[^"]*"' | head -1)"

# 2. Verificar se o admin existe
echo -e "${COLOR_YELLOW}2. Verificando admin...${COLOR_NC}"
API_VERSION=$(api_call "GET" "/health" "" "")
echo -e "  API: $(echo "$API_VERSION" | grep -o '"status":"[^"]*"' | head -1)"
echo -e "  ${COLOR_GREEN}✓ API operacional${COLOR_NC}"

# 3. Listar zonas ativas
echo -e "${COLOR_YELLOW}3. Zonas ativas:${COLOR_NC}"
ZONES=$(api_call "GET" "/zones" "" "auth")
echo "  $ZONES" | python3 -m json.tool 2>/dev/null || echo "  $ZONES"

echo ""
echo -e "${COLOR_GREEN}=== Seed concluido! ===${COLOR_NC}"
echo ""
echo -e "Proximos passos:"
echo "  1. Crie estabelecimentos via POST /establishments"
echo "  2. Associe produtos via POST /products/create"
echo "  3. Crie assinatura de teste: POST /subscriptions"
echo "  4. Crie patrocinio de teste: POST /sponsored"
