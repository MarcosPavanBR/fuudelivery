#!/bin/bash
# ============================================================================
# Script: Listar env vars do Render para auditoria de segurança
# ============================================================================
# Uso:
#   export RENDER_API_KEY="rnd_..."
#   ./scripts/list-render-env-vars.sh
#
# Este script:
#   1. Lista todas as env vars de cada serviço
#   2. Identifica quais são sensíveis (precisam rotação)
#   3. Mostra um resumo de segurança
# ============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# Verificar API key
if [ -z "$RENDER_API_KEY" ]; then
    echo -e "${RED}ERRO: RENDER_API_KEY não configurado${NC}"
    echo
    echo "Como obter:"
    echo "  1. Acesse https://dashboard.render.com"
    echo "  2. Account Settings → API Keys"
    echo "  3. Create API Key (ou copie existente)"
    echo
    echo "Depois execute:"
    echo "  export RENDER_API_KEY='rnd_...'"
    echo "  ./scripts/list-render-env-vars.sh"
    exit 1
fi

echo -e "${CYAN}====================================================================${NC}"
echo -e "${CYAN}  Auditoria de Env Vars do Render - FuuDelivery${NC}"
echo -e "${CYAN}====================================================================${NC}"
echo

# IDs dos serviços (do deploy.yml)
declare -A SERVICES=(
    ["fuudelivery-api"]="srv-d9e55qf41pts73e8q8dg"
    ["fuudelivery-web"]="srv-d9edpar7uimc73fdotp0"
    ["fuudelivery-admin"]="srv-d9elp2n41pts73f5kvf0"
)

# Env vars que são sensíveis e PRECISAM ser rotacionadas
SENSITIVE_VARS=(
    "JWT_SECRET"
    "ADMIN_PASSWORD"
    "MONGO_URI"
    "DB_CONNECTION_STRING"
    "REDIS_URL"
    "ABACATE_PAY_API_KEY"
    "ABACATE_PAY_WEBHOOK_SECRET"
    "SUPABASE_URL"
    "SUPABASE_SERVICE_ROLE_KEY"
    "RENDER_API_KEY"
)

# Env vars que são públicas (não precisam rotação)
PUBLIC_VARS=(
    "NODE_ENV"
    "GO_VERSION"
    "WEB_CONCURRENCY"
    "REACT_APP_API_URL"
    "REACT_APP_PAYMENT_API_URL"
)

echo -e "${YELLOW}Serviços encontrados: ${#SERVICES[@]}${NC}"
echo

for SERVICE_NAME in "${!SERVICES[@]}"; do
    SERVICE_ID="${SERVICES[$SERVICE_NAME]}"
    
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}📦 ${SERVICE_NAME}${NC} (${SERVICE_ID})"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    # Buscar env vars
    RESPONSE=$(curl -s \
        -H "Authorization: Bearer ${RENDER_API_KEY}" \
        "https://api.render.com/v1/services/${SERVICE_ID}/env-vars" 2>/dev/null)
    
    # Verificar se houve erro
    if echo "$RESPONSE" | grep -q '"error"'; then
        echo -e "${RED}  ❌ Erro ao buscar env vars: $(echo $RESPONSE | grep -o '"message":"[^"]*"' | head -1)${NC}"
        echo
        continue
    end
    
    # Contar env vars
    ENV_COUNT=$(echo "$RESPONSE" | grep -o '"key":' | wc -l)
    echo -e "  Total de env vars: ${ENV_COUNT}"
    echo
    
    # Listar env vars sensíveis
    echo -e "  ${RED}🔒 Env vars SENSÍVEIS (precisam rotação):${NC}"
    FOUND_SENSITIVE=0
    
    for VAR in "${SENSITIVE_VARS[@]}"; do
        VALUE=$(echo "$RESPONSE" | grep -A1 "\"key\":\"${VAR}\"" | grep -o '"value":"[^"]*"' | cut -d'"' -f4)
        if [ -n "$VALUE" ]; then
            # Mascarar valor
            if [ ${#VALUE} -gt 10 ]; then
                MASKED="${VALUE:0:6}...${VALUE: -4}"
            else
                MASKED="***"
            fi
            echo -e "    • ${VAR} = ${MASKED}"
            FOUND_SENSITIVE=1
        fi
    done
    
    if [ $FOUND_SENSITIVE -eq 0 ]; then
        echo -e "    (nenhum encontrado)"
    fi
    
    echo
    
    # Listar env vars públicas
    echo -e "  ${GREEN}📋 Env vars PÚBLICAS (não precisam rotação):${NC}"
    FOUND_PUBLIC=0
    
    for VAR in "${PUBLIC_VARS[@]}"; do
        VALUE=$(echo "$RESPONSE" | grep -A1 "\"key\":\"${VAR}\"" | grep -o '"value":"[^"]*"' | cut -d'"' -f4)
        if [ -n "$VALUE" ]; then
            echo -e "    • ${VAR} = ${VALUE}"
            FOUND_PUBLIC=1
        fi
    done
    
    if [ $FOUND_PUBLIC -eq 0 ]; then
        echo -e "    (nenhum encontrado)"
    fi
    
    echo
done

echo -e "${CYAN}====================================================================${NC}"
echo -e "${YELLOW}📋 RESUMO DE SEGURANÇA${NC}"
echo -e "${CYAN}====================================================================${NC}"
echo
echo -e "${RED}Env vars que PRECISAM ser rotacionadas:${NC}"
for VAR in "${SENSITIVE_VARS[@]}"; do
    echo -e "  • ${VAR}"
done
echo
echo -e "${YELLOW}Como rotacionar:${NC}"
echo "  1. Gere novos valores nos painéis dos serviços"
echo "  2. Atualize via Render Dashboard ou API"
echo "  3. O Render faz auto-deploy"
echo "  4. Teste que o sistema continua funcionando"
echo
echo -e "${GREEN}Dica: Para atualizar via API, use:${NC}"
echo '  curl -X PATCH \'
echo '    -H "Authorization: Bearer $RENDER_API_KEY" \'
echo '    -H "Content-Type: application/json" \'
echo '    "https://api.render.com/v1/services/SERVICE_ID/env-vars" \'
echo '    -d '"'"'{"envVars": [{"key": "VAR_NAME", "value": "NEW_VALUE"}]}'"'"''
echo
