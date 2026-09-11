# Guia de Deploy - FuuDelivery

## Deploy no Render (Produção)

### Pré-requisitos
1. Conta no Render.com
2. Repositório no GitHub
3. Supabase (projeto gratuito)
4. Redis (Render ou Upstash)
5. Conta AbacatePay (para pagamentos PIX)

### Variáveis de Ambiente Obrigatórias

#### Backend (Monolito + Payment Service)
```
# Banco de Dados
DB_CONNECTION_STRING=postgresql://...
REDIS_URL=redis://...

# Autenticação
JWT_SECRET=<gerar com: openssl rand -hex 32>
# Bootstrap do primeiro admin (POST /admin/bootstrap).
# NAO deixe esta variavel configurada de forma permanente: enquanto ela
# existe, quem tiver o valor promove qualquer conta a admin. Adicione,
# chame o endpoint uma vez, e apague. Numa instalacao que ja tem admin o
# endpoint recusa, entao ela so agrega risco.
# ADMIN_BOOTSTRAP_SECRET=<senha forte>   # descomente so no setup inicial

# Pagamentos
ABACATE_PAY_API_KEY=abc_prod_...
ABACATE_PAY_WEBHOOK_SECRET=whsec_...

# Storage de imagens (obrigatório p/ upload — sem isso o endpoint responde 503)
SUPABASE_URL=https://seu-projeto.supabase.co
SUPABASE_SERVICE_ROLE_KEY=...
```

### Configuração dos Serviços

#### 1. FuuDelivery API (Monolito)
- **Build Command**: `cd cmd/fuudelivery && go build -o ../../server .`
- **Start Command**: `./server`
- **Port**: 3000 (definido em render.yaml; o Render roteia 443 → essa porta)
- **Plan**: Free (1 serviço dinâmico ≈730h cabe nas 750h/mês) ou Starter

#### 3. Frontend (WebAdmin + WebRestaurant)
- **Build Command**: `cd Frontend/WebAdmin && npm install --legacy-peer-deps && npm run build`
- **Publish Directory**: `Frontend/WebAdmin/build` (outDir do Vite)

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

## Monitoramento (obrigatório antes de dinheiro real)

### Por que não dá para confiar no workflow do GitHub

Existe um `Monitor Production` no GitHub Actions, com cron de 30 minutos. **Ele
não cumpre esse intervalo.** Medindo os 99 intervalos reais entre execuções
agendadas:

| | |
|---|---|
| mínimo | 47 min |
| **mediana** | **212 min (3h32)** |
| máximo | 776 min (~13 h) |

Nenhum dos 99 chegou perto de 30 minutos — o GitHub despriorizA schedules de
alta frequência em runners compartilhados. Trate aquele workflow como **rede de
segurança, não alarme**: a produção pode ficar fora do ar uma noite inteira
antes de ele rodar.

### Monitor externo — o alarme de verdade

Aponte um serviço externo para o `/health`. UptimeRobot e BetterStack têm plano
gratuito com checagem de 1–5 min e notificação no celular.

```
URL:        https://<sua-api>/health
Intervalo:  1–5 min
Alertar se: status HTTP != 200
```

**Como ler a resposta do `/health`:**

| `status` | HTTP | O que significa |
|---|---|---|
| `up` | 200 | tudo de pé |
| `degraded` | 200 | Postgres e gateway (os críticos) de pé; só Redis/batches degradados. A plataforma vende normalmente — é aviso, não queda |
| `down` | 503 | algum crítico fora do ar |
| `starting` | 200 | subindo; o banco ainda conecta (até 125 s). Se **persistir**, o banco não conectou |

Cuidado ao configurar: `starting` devolve **200**. Um monitor que só olha o
código HTTP vai achar que está tudo bem durante um cold start — o que é
correto — mas também durante uma falha permanente de banco. Se o seu serviço
permitir, alerte também quando o corpo contiver `"status":"starting"` por mais
de 5 minutos seguidos.

### `METRICS_TOKEN` — obrigatório em produção

O `GET /metrics` expõe dado operacional (volume de pedidos, backlog de
pagamentos, profundidade de fila). Em produção ele **falha fechado**: com
`GO_ENV=production` e `METRICS_TOKEN` vazio, devolve **403** e registra no log
por quê. Em dev local, sem token, segue aberto.

```bash
# Gere um token e cole no Render → Environment
openssl rand -hex 32

# Para ler as métricas:
curl -H "Authorization: Bearer $METRICS_TOKEN" https://<sua-api>/metrics
```

### O que alertar no `/metrics`

Estas métricas existem para serem alertadas — sem regra configurada, elas são
decoração:

| Métrica | Alertar quando | O que significa |
|---|---|---|
| `fuudelivery_reconciliation_healed_total` | subir no período | **não é boa notícia.** O webhook síncrono falhou e a rede de segurança segurou. A causa precisa ser investigada, não só o sintoma |
| `fuudelivery_reconciliation_pending` | > 0 por mais de 15 min | a própria reconciliação está falhando — dinheiro parado e ninguém buscando |
| `fuudelivery_reconciliation_last_run_seconds` | > 900 | o job de reconciliação morreu |
| `fuudelivery_orders_stuck` | > 0 | pedido pago que continua sem entregador (ninguém chamou `POST /dispatch/trigger`) |
| `fuudelivery_dispatch_dlq_depth` | crescendo sem cair | pedidos não estão achando entregador |
| `fuudelivery_queue_publish_errors_total` | subir rápido | Redis caiu depois do boot; a fila não reconecta sozinha (só notificação é afetada, não dinheiro) |

---

## Frete por região — cadastro e conferência

### Aplicar a migração

Cole o conteúdo de `sql/24_delivery_region_fees.sql` no **SQL Editor do
Supabase**. É idempotente (`CREATE TABLE IF NOT EXISTS` + `ADD CONSTRAINT`
dentro de `DO $$`), então rodar duas vezes não faz mal.

Sem a migração o serviço **não quebra**: o `AutoMigrate` cria a tabela sozinho
na subida. O que falta nesse caso são os CHECKs e o índice parcial — faixa de
CEP invertida, CEP fora de 0–99999999 e frete negativo passariam a ser aceitos
pelo banco.

Conferir se os CHECKs existem — são **dois**, e o primeiro cobre duas regras
(limites do CEP e `cep_start <= cep_end`):

```sql
SELECT conname FROM pg_constraint
WHERE conrelid = 'delivery_region_fees'::regclass AND contype = 'c'
ORDER BY conname;
-- esperado, exatamente estas 2 linhas:
--   delivery_region_fees_faixa_check   -- CEP em 0..99999999 E cep_start <= cep_end
--   delivery_region_fees_fee_check     -- fee >= 0
```

### Cadastrar as regiões

No **WebAdmin → Regiões**. Como o desempate funciona:

1. Menor `prioridade` vence.
2. Empatou: vence a **faixa mais estreita**.

É isso que permite cadastrar a cidade inteira numa faixa ampla e depois
recortar um bairro mais caro, sem reordenar nada. `Cidade` e `UF` em branco
valem para qualquer cidade.

**Enquanto não houver nenhuma região cadastrada, todo pedido cai na taxa por km
do estabelecimento** — nunca em frete grátis. A tela avisa isso em vermelho, e
o servidor registra no log cada CEP sem cobertura:

```
[FRETE] CEP "01310100" sem região cadastrada — caindo na taxa por km do estabelecimento 42
```

É assim que você descobre bairro não coberto depois de lançar. Conferir quantas
regiões estão ativas:

```sql
SELECT count(*) FILTER (WHERE active) AS ativas, count(*) AS total
FROM delivery_region_fees;
```

---

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

### Redis
- Persistência habilitada (RDB + AEOF)
- Snapshots a cada 15 minutos
