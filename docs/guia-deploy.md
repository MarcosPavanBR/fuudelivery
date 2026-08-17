# Guia de Deploy - FuuDelivery

## Deploy no Render (Produção)

### Pré-requisitos
1. Conta no Render.com
2. Repositório no GitHub
3. MongoDB Atlas (cluster gratuito)
4. Supabase (projeto gratuito)
5. Redis (Render ou Upstash)
6. Conta AbacatePay (para pagamentos PIX)

### Variáveis de Ambiente Obrigatórias

#### Backend (Monolito + Payment Service)
```
# Banco de Dados
DB_CONNECTION_STRING=postgresql://...
MONGO_URI=mongodb+srv://...
REDIS_URL=redis://...

# Autenticação
JWT_SECRET=<gerar com: openssl rand -hex 32>
ADMIN_PASSWORD=<senha forte>

# Pagamentos
ABACATE_PAY_API_KEY=abc_prod_...
ABACATE_PAY_WEBHOOK_SECRET=whsec_...

# Storage (opcional)
SUPABASE_URL=https://...
SUPABASE_KEY=...
```

### Configuração dos Serviços

#### 1. FuuDelivery API (Monolito)
- **Build Command**: `cd cmd/fuudelivery && go build -o ../../server .`
- **Start Command**: `./server`
- **Port**: 10000
- **Plan**: Starter (para produção)

#### 3. Frontend (WebAdmin + WebRestaurant)
- **Build Command**: `cd Frontend/WebAdmin && npm install && npm run build`
- **Publish Directory**: `Frontend/WebAdmin/dist`

### Deploy Automático

O `render.yaml` na raiz configura deploy automático:
1. Push para `main` triggera deploy
2. CI executa testes antes do deploy
3. Deploy é feito apenas se CI passar

### Verificação Pós-Deploy

```bash
# Health check
curl https://fuudelivery-api-8y6l.onrender.com/health

# Verificar logs
# Acesse Render Dashboard → Service → Logs
```

### Troubleshooting

| Erro | Causa | Solução |
|------|-------|---------|
| `panic: DB_CONNECTION_STRING não configurado` | Env vars faltando | Adicionar no Render Dashboard |
| `panic: constraint does not exist` | Migration falhou | Verificar logs, pode ignorar se servidor subir |
| Deploy timeout | Build lento | Verificar cache, considerar plano pago |

## Deploy Local (Desenvolvimento)

### 1. Clonar o repositório
```bash
git clone https://github.com/MarcosPavanBR/fuudelivery.git
cd fuudelivery
```

### 2. Configurar variáveis de ambiente
```bash
cp .env.example .env
# Editar .env com suas credenciais
```

### 3. Iniciar serviços
```bash
# Backend (monolito)
cd cmd/fuudelivery
go run main.go

# Frontend (em outro terminal)
cd Frontend/WebAdmin
npm install
npm run dev
```

### 4. Docker (alternativa)
```bash
docker-compose -f docker/docker-compose.dev.yml up
```

## Backup

### PostgreSQL (Supabase)
- Backup automático diário (gratuito)
- Point-in-Time Recovery (plano pago)

### MongoDB Atlas
- Backup automático diário
- Oplog habilitado para restauração pontual

### Redis
- Persistência habilitada (RDB + AEOF)
- Snapshots a cada 15 minutos
