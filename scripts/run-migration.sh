#!/bin/bash
# Script para executar a migration no banco de dados do Render
# Uso: ./scripts/run-migration.sh

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}================================================${NC}"
echo -e "${YELLOW}  FuuDelivery - Executar Migration Corretiva${NC}"
echo -e "${YELLOW}================================================${NC}"
echo

# Verificar se DB_CONNECTION_STRING está configurada
if [ -z "$DB_CONNECTION_STRING" ]; then
    echo -e "${RED}ERRO: DB_CONNECTION_STRING não configurado${NC}"
    echo
    echo "Para obter a connection string:"
    echo "1. Acesse https://dashboard.render.com"
    echo "2. Vá em fuudelivery-api → Environment"
    echo "3. Copie o valor de DB_CONNECTION_STRING"
    echo
    echo "Para configurar:"
    echo "  export DB_CONNECTION_STRING='postgresql://...'"
    exit 1
fi

echo -e "${GREEN}DB_CONNECTION_STRING configurado${NC}"
echo

# Verificar se psql está disponível
if ! command -v psql &> /dev/null; then
    echo -e "${RED}ERRO: psql não encontrado${NC}"
    echo "Instale o PostgreSQL client:"
    echo "  Windows: choco install postgresql"
    echo "  Mac: brew install postgresql"
    echo "  Linux: sudo apt-get install postgresql-client"
    exit 1
fi

echo -e "${YELLOW}Executando migration...${NC}"
echo

# Executar migration
psql "$DB_CONNECTION_STRING" -f scripts/migrate-fix-all.sql

echo
echo -e "${GREEN}✅ Migration executada com sucesso!${NC}"
echo
echo -e "${YELLOW}Próximos passos:${NC}"
echo "1. Reinicie o serviço no Render (ou espere auto-deploy)"
echo "2. Verifique os logs: deve mostrar [CALIBRATION] Starting calibration cycle"
echo "3. Teste a API: curl https://fuudelivery-api-8y6l.onrender.com/health"
