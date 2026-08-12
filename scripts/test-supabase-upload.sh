#!/bin/bash
# ============================================================================
# Script de teste: Upload para Supabase Storage
# ============================================================================
# Uso: 
#   export SUPABASE_URL='https://seu-projeto.supabase.co'
#   export SUPABASE_SERVICE_ROLE_KEY='eyJhbG...'
#   ./scripts/test-supabase-upload.sh
# ============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}=== Teste de Upload para Supabase Storage ===${NC}"
echo

# Verificar variaveis
if [ -z "$SUPABASE_URL" ]; then
    echo -e "${RED}ERRO: SUPABASE_URL nao configurado${NC}"
    exit 1
fi

if [ -z "$SUPABASE_SERVICE_ROLE_KEY" ]; then
    echo -e "${RED}ERRO: SUPABASE_SERVICE_ROLE_KEY nao configurado${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Variaveis configuradas${NC}"
echo "  SUPABASE_URL: ${SUPABASE_URL:0:30}..."
echo

# Gerar imagem PNG placeholder (1x1 pixel vermelho)
echo -e "${YELLOW}Gerando imagem placeholder...${NC}"
echo "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwADhQGAWjR9awAAAABJRU5ErkJggg==" | base64 -d > /tmp/test-upload.png
echo -e "${GREEN}✓ Imagem criada (1x1 pixel vermelho)${NC}"
echo

# Preparar upload
BUCKET="fuudelivery-images"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
FILE_PATH="test/upload-test-${TIMESTAMP}.png"
FULL_URL="${SUPABASE_URL}/storage/v1/object/${BUCKET}/${FILE_PATH}"

echo -e "${YELLOW}Fazendo upload: ${FILE_PATH}${NC}"

HTTP_CODE=$(curl -s -o /tmp/upload-response.json -w "%{http_code}" \
    -X POST \
    -H "Authorization: Bearer ${SUPABASE_SERVICE_ROLE_KEY}" \
    -H "Content-Type: image/png" \
    --data-binary @/tmp/test-upload.png \
    "${FULL_URL}")

echo

if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 201 ]; then
    echo -e "${GREEN}✅ Upload bem-sucedido! (HTTP ${HTTP_CODE})${NC}"
    echo
    PUBLIC_URL="${SUPABASE_URL}/storage/v1/object/public/${BUCKET}/${FILE_PATH}"
    echo -e "${GREEN}URL publica:${NC}"
    echo "  ${PUBLIC_URL}"
    echo
    CHECK_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${PUBLIC_URL}")
    if [ "$CHECK_CODE" -eq 200 ]; then
        echo -e "${GREEN}✅ URL publica acessivel!${NC}"
    else
        echo -e "${YELLOW}⚠️  URL retornou HTTP ${CHECK_CODE} - verifique policies RLS${NC}"
    fi
else
    echo -e "${RED}❌ Upload falhou (HTTP ${HTTP_CODE})${NC}"
    cat /tmp/upload-response.json
    echo
    echo -e "${YELLOW}Possiveis causas:${NC}"
    echo "  1. Bucket 'fuudelivery-images' nao existe"
    echo "  2. Chave API invalida"
    echo "  3. Policies RLS bloqueando"
    exit 1
fi

rm -f /tmp/test-upload.png /tmp/upload-response.json
