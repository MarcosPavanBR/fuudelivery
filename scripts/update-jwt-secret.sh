#!/bin/bash

# Script para atualizar JWT_SECRET no Render
# Uso: ./scripts/update-jwt-secret.sh

set -e

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}================================================================${NC}"
echo -e "${YELLOW}  Atualizar JWT_SECRET no Render - FuuDelivery${NC}"
echo -e "${YELLOW}================================================================${NC}"
echo

# Verificar se RENDER_API_KEY está configurada
if [ -z "$RENDER_API_KEY" ]; then
    echo -e "${RED}ERRO: RENDER_API_KEY não configurado${NC}"
    echo
    echo "Para obter a API key:"
    echo "1. Acesse https://dashboard.render.com"
    echo "2. Vá em Account Settings → API Keys"
    echo "3. Crie uma nova API Key"
    echo
    echo "Para configurar:"
    echo "  export RENDER_API_KEY='rnd_...'"
    exit 1
fi

# Gerar novo JWT_SECRET
echo -e "${GREEN}Gerando novo JWT_SECRET seguro...${NC}"
NEW_JWT_SECRET=$(openssl rand -hex 32)
echo -e "Novo JWT_SECRET: ${GREEN}${NEW_JWT_SECRET}${NC}"
echo

# Service ID do Render (fuudelivery-api)
SERVICE_ID="srv-d9e55qf41pts73e8q8dg"

echo -e "${YELLOW}Atualizando JWT_SECRET no Render...${NC}"
echo "Service ID: $SERVICE_ID"
echo

# Atualizar via API
RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH \
  -H "Authorization: Bearer $RENDER_API_KEY" \
  -H "Content-Type: application/json" \
  "https://api.render.com/v1/services/$SERVICE_ID/env-vars" \
  -d "{\"envVars\": [{\"key\": \"JWT_SECRET\", \"value\": \"$NEW_JWT_SECRET\"}]}")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" -eq 200 ]; then
    echo -e "${GREEN}✅ JWT_SECRET atualizado com sucesso!${NC}"
    echo
    echo -e "${YELLOW}Próximos passos:${NC}"
    echo "1. O Render fará auto-deploy em ~2 minutos"
    echo "2. Teste o sistema: https://fuudelivery-api-8y6l.onrender.com/health"
    echo "3. Verifique se o login funciona: https://fuudelivery-admin-lv7f.onrender.com/"
else
    echo -e "${RED}❌ Erro ao atualizar JWT_SECRET${NC}"
    echo "HTTP Code: $HTTP_CODE"
    echo "Resposta: $BODY"
    exit 1
fi

echo
echo -e "${YELLOW}IMPORTANTE:${NC}"
echo "- Todas as sessões JWT anteriores serão invalidadas"
echo "- Usuários precisarão fazer login novamente"
echo "- O novo JWT_SECRET está salvo no Render"
echo
echo -e "${GREEN}Script concluído!${NC}"
