#!/bin/bash
# =============================================================================
# FuuDelivery — Seed de dados de teste via API
# =============================================================================
# Este script cria registros reais via a API de produção para teste.
# Execute: bash scripts/seed-test-data.sh
# =============================================================================

set -euo pipefail

API="${FUU_API_URL:-https://fuudelivery-api-8y6l.onrender.com}"
BOOTSTRAP_SECRET="${FUU_BOOTSTRAP_SECRET:-}"
ADMIN_PASSWORD="${FUU_ADMIN_PASSWORD:-}"
RESTAURANT_PASSWORD="${FUU_RESTAURANT_PASSWORD:-}"
CLIENT_PASSWORD="${FUU_CLIENT_PASSWORD:-}"
DELIVERY_PASSWORD="${FUU_DELIVERY_PASSWORD:-}"
ADMIN_TOKEN=""
RESTAURANT_TOKEN=""
CLIENT_TOKEN=""
DELIVERY_TOKEN=""

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[✓]${NC} $1"; }
err() { echo -e "${RED}[✗]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }

check() {
    local resp="$1"
    local desc="$2"
    if echo "$resp" | grep -q '"error"'; then
        err "$desc — $(echo "$resp" | head -c 200)"
        return 1
    fi
    log "$desc"
    return 0
}

# 0) Health check
echo "============================================"
echo "  FuuDelivery — Seed de dados de teste"
echo "============================================"
echo ""
echo "API: $API"
echo ""

echo "0. Verificando API..."
HEALTH=$(curl -s "$API/health" 2>/dev/null || echo '{"status":"down"}')
if echo "$HEALTH" | grep -q '"up"'; then
    log "API está UP"
else
    err "API está DOWN: $HEALTH"
    exit 1
fi

# 1) Bootstrap admin
echo ""
echo "1. Bootstrapping admin..."
if [ -n "$BOOTSTRAP_SECRET" ]; then
    BOOTSTRAP_RESP=$(curl -s -X POST "$API/admin/bootstrap" \
        -H "Content-Type: application/json" \
        -d "{\"secret\":\"$BOOTSTRAP_SECRET\",\"email\":\"admin@fuudelivery.com\",\"phone\":\"+5511999900001\",\"name\":\"Admin Master\",\"password\":\"$ADMIN_PASSWORD\"}")
    check "$BOOTSTRAP_RESP" "Bootstrap admin" || warn "Pode já existir um admin — tentando login..."
else
    warn "FUU_BOOTSTRAP_SECRET não definido — pulando bootstrap do admin"
fi

# 2) Login admin
echo ""
echo "2. Login admin..."
LOGIN_RESP=$(curl -s -X POST "$API/users/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@fuudelivery.com","password":"'${ADMIN_PASSWORD:-Admin@2026!}'"}')
ADMIN_TOKEN=$(echo "$LOGIN_RESP" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$ADMIN_TOKEN" ]; then
    log "Admin logado com sucesso (token: ${ADMIN_TOKEN:0:20}...)"
else
    err "Falha no login admin: $LOGIN_RESP"
    exit 1
fi

# 3) Criar restaurante (usuário)
echo ""
echo "3. Criando restaurante 'Sabor da Terra'..."
REST_RESP=$(curl -s -X POST "$API/users/register" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "Sabor da Terra",
        "email": "restaurante@sabordaterra.com",
        "phone": "+5511999900002",
        "password": "'${RESTAURANT_PASSWORD:-Rest@2026!}'",
        "establishment": {
            "name": "Sabor da Terra - Matriz",
            "description": "Comida caseira brasileira com tempero especial",
            "image": "https://images.unsplash.com/photo-1517248135467-4c7edcad34c4?w=400",
            "primary_color": "#DC2626",
            "secondary_color": "#F59E0B",
            "horarioFuncionamento": "Seg-Sex 11:00-22:00, Sáb 11:00-23:00, Dom 12:00-21:00",
            "lat": -23.5505,
            "long": -46.6333,
            "location_string": "Rua Augusta, 1500 - Consolação, São Paulo - SP",
            "max_distance_delivery": 5.0
        }
    }')
check "$REST_RESP" "Criar restaurante" || warn "Restaurante pode já existir"

# 4) Login restaurante
echo ""
echo "4. Login restaurante..."
REST_LOGIN=$(curl -s -X POST "$API/users/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"restaurante@sabordaterra.com","password":"'${RESTAURANT_PASSWORD:-Rest@2026!}'"}')
RESTAURANT_TOKEN=$(echo "$REST_LOGIN" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$RESTAURANT_TOKEN" ]; then
    log "Restaurante logado (token: ${RESTAURANT_TOKEN:0:20}...)"
else
    warn "Falha login restaurante: $REST_LOGIN"
fi

# 5) Listar estabelecimento criado
echo ""
echo "5. Listando estabelecimentos..."
ESTAB_RESP=$(curl -s "$API/establishments")
check "$ESTAB_RESP" "Listar estabelecimentos"
ESTAB_ID=$(echo "$ESTAB_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
if [ -n "$ESTAB_ID" ]; then
    log "Estabelecimento ID: $ESTAB_ID"
else
    warn "Não conseguiu extrair ID do estabelecimento"
fi

# 6) Criar categoria e produtos
if [ -n "$RESTAURANT_TOKEN" ] && [ -n "$ESTAB_ID" ]; then
    echo ""
    echo "6. Criando categorias e produtos..."
    
    # Criar categorias
    CAT_RESP=$(curl -s -X POST "$API/categories/create" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $RESTAURANT_TOKEN" \
        -d '[
            {"name": "Pratos Principais", "establishment_id": '"$ESTAB_ID"'},
            {"name": "Bebidas", "establishment_id": '"$ESTAB_ID"'},
            {"name": "Sobremesas", "establishment_id": '"$ESTAB_ID"'}
        ]')
    check "$CAT_RESP" "Criar categorias"
    
    # Criar produtos
    PROD_RESP=$(curl -s -X POST "$API/products/multi-create" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $RESTAURANT_TOKEN" \
        -d '[
            {"name": "Feijão Tropeiro Mineiro", "price": 32.90, "description": "Feijão tropeiro com linguicça, bacon, ovo, couve e farofa", "image": "https://images.unsplash.com/photo-1623428187969-5d21ed0fdd39?w=400", "establishment_id": '"$ESTAB_ID"'},
            {"name": "Peixe Frito com Vinagrete", "price": 35.90, "description": "Peixe tilápia frito com vinagrete de tomate e cebola", "image": "https://images.unsplash.com/photo-1580476262798-bddd9f4b7369?w=400", "establishment_id": '"$ESTAB_ID"'},
            {"name": "Frango Assado com Batata", "price": 29.90, "description": "Frango inteiro assado com batatas e polenta", "image": "https://images.unsplash.com/photo-1598103442097-8b74394b95c6?w=400", "establishment_id": '"$ESTAB_ID"'},
            {"name": "Coca-Cola Lata", "price": 6.00, "description": "Coca-Cola 350ml gelada", "image": "https://images.unsplash.com/photo-1622483767028-3f66f32aef97?w=400", "establishment_id": '"$ESTAB_ID"'},
            {"name": "Suco Natural Laranja", "price": 8.00, "description": "Suco de laranja natural 500ml", "image": "https://images.unsplash.com/photo-1621506289937-a8e4df240d0b?w=400", "establishment_id": '"$ESTAB_ID"'},
            {"name": "Pudim de Leite", "price": 12.90, "description": "Pudim caseiro com calda de caramelo", "image": "https://images.unsplash.com/photo-1571877227200-a0d98ea607e9?w=400", "establishment_id": '"$ESTAB_ID"'}
        ]')
    check "$PROD_RESP" "Criar produtos"
fi

# 7) Criar cliente
echo ""
echo "7. Criando cliente 'Maria Silva'..."
CLIENT_RESP=$(curl -s -X POST "$API/clients/register" \
    -H "Content-Type: application/json" \
    -d '{"name":"Maria Silva","phone":"+5511999900003","password":"'${CLIENT_PASSWORD:-Client@2026!}'"}')
check "$CLIENT_RESP" "Criar cliente"

# 8) Login cliente
echo ""
echo "8. Login cliente..."
CLIENT_LOGIN=$(curl -s -X POST "$API/clients/login" \
    -H "Content-Type: application/json" \
    -d '{"phone":"+5511999900003","password":"'${CLIENT_PASSWORD:-Client@2026!}'"}')
CLIENT_TOKEN=$(echo "$CLIENT_LOGIN" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$CLIENT_TOKEN" ]; then
    log "Cliente logado (token: ${CLIENT_TOKEN:0:20}...)"
else
    warn "Falha login cliente: $CLIENT_LOGIN"
fi

# 9) Criar entregador
echo ""
echo "9. Criando entregador 'João Entregador'..."
DELIV_RESP=$(curl -s -X POST "$API/delivery-man/register" \
    -H "Content-Type: application/json" \
    -d '{"name":"João Entregador","email":"joao@entregador.com","phone":"+5511999900004","password":"'${DELIVERY_PASSWORD:-Entrega@2026!}'"}')
check "$DELIV_RESP" "Criar entregador"

# 10) Login entregador
echo ""
echo "10. Login entregador..."
DELIV_LOGIN=$(curl -s -X POST "$API/delivery-man/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"joao@entregador.com","password":"'${DELIVERY_PASSWORD:-Entrega@2026!}'"}')
DELIVERY_TOKEN=$(echo "$DELIV_LOGIN" | grep -o '"token":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$DELIVERY_TOKEN" ]; then
    log "Entregador logado (token: ${DELIVERY_TOKEN:0:20}...)"
else
    warn "Falha login entregador: $DELIV_LOGIN"
fi

# 11) Listar tudo para verificação
echo ""
echo "11. Verificação final..."
echo ""
echo "   Estabelecimentos:"
curl -s "$API/establishments" | python3 -m json.tool 2>/dev/null || curl -s "$API/establishments"
echo ""
echo "   Usuários (admin):"
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" "$API/users" | python3 -m json.tool 2>/dev/null || curl -s -H "Authorization: Bearer $ADMIN_TOKEN" "$API/users"
echo ""
echo "   Clientes:"
curl -s "$API/clients" 2>/dev/null || echo "(endpoint pode não existir)"

echo ""
echo "============================================"
echo "  RESUMO DOS DADOS CRIADOS"
echo "============================================"
echo ""
echo "👤 Admin:"
echo "   Email: admin@fuudelivery.com"
echo "   Senha: (definida via FUU_ADMIN_PASSWORD)"
echo ""
echo "🍽️  Restaurante:"
echo "   Nome: Sabor da Terra - Matriz"
echo "   Email: restaurante@sabordaterra.com"
echo "   Senha: (definida via FUU_RESTAURANT_PASSWORD)"
echo ""
echo "📦 Produtos: 6 itens (Feijão Tropeiro, Peixe Frito, Frango, etc.)"
echo ""
echo "🛒 Cliente:"
echo "   Nome: Maria Silva"
echo "   Telefone: +5511999900003"
echo "   Senha: (definida via FUU_CLIENT_PASSWORD)"
echo ""
echo "🚚 Entregador:"
echo "   Nome: João Entregador"
echo "   Email: joao@entregador.com"
echo "   Senha: (definida via FUU_DELIVERY_PASSWORD)"
echo ""
echo "============================================"
echo "  Tokens (para uso manual com curl)"
echo "============================================"
echo ""
if [ -n "$ADMIN_TOKEN" ]; then
    echo "ADMIN_TOKEN=$ADMIN_TOKEN"
fi
if [ -n "$RESTAURANT_TOKEN" ]; then
    echo "RESTAURANT_TOKEN=$RESTAURANT_TOKEN"
fi
if [ -n "$CLIENT_TOKEN" ]; then
    echo "CLIENT_TOKEN=$CLIENT_TOKEN"
fi
if [ -n "$DELIVERY_TOKEN" ]; then
    echo "DELIVERY_TOKEN=$DELIVERY_TOKEN"
fi
echo ""
echo "Pronto! Todos os registros foram criados via API. 🎉"
