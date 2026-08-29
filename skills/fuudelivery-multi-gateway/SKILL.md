# Skill: FuuDelivery — Framework Multi-Gateway de Pagamentos

## Visão Geral

O FuuDelivery é uma plataforma de delivery (Go monolito + React/React Native) que precisava de uma arquitetura de pagamentos com **split automático**, **múltiplos gateways**, **pré-autorização de cartão**, e **escrow (custódia)**. Esta skill documenta tudo o que foi implementado e o que falta para produção.

---

## Arquitetura do Sistema

```
┌─────────────────────────────────────────────────┐
│              Payment Router (Go)                │
│  Seleciona gateway por: método + split + CB     │
│  Flags: GATEWAY_PRIMARY, GATEWAY_FALLBACK       │
└──┬──────────┬──────────┬──────────┬────────────┘
   │          │          │          │
┌──▼──┐  ┌───▼───┐  ┌───▼───┐  ┌───▼───┐
│Pagar│  │ Asaas │  │Abacate│  │  MP   │
│.me  │  │       │  │ Pay   │  │       │
└─────┘  └───────┘  └───────┘  └───────┘
PRINCIPAL  ALTERNAT.  FALLBACK   RESERVA
```

### Papel de Cada Gateway

| Gateway | Papel | PIX | Cartão | Débito | Split | Pre-Auth | 3DS | Escrow | Taxa PIX |
|---------|-------|:---:|:------:|:------:|:-----:|:--------:|:---:|:------:|----------|
| **Pagar.me** | 🔵 **Principal** | ✅ | ✅ | ✅ | ✅ Nativo | ✅ | ✅ | ✅ D+X | R$ 0,39 |
| **Asaas** | 🟢 **Alternativo** | ✅ | ✅ | ✅ | ✅ Nativo | ✅ | ✅ | ✅ D+X | R$ 0,99 |
| **AbacatePay** | 🟡 **Fallback PIX** | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | R$ 0,99 |
| **Mercado Pago** | ⚪ **Reserva** | ✅ | ✅ | ✅ | ⚠️ 1:1 | ❌ | ✅ | ❌ | R$ 0,39 |

---

## O Que Foi Implementado

### Arquivos Criados

```
pkg/gateway/
├── gateway.go              Interface Gateway + 15 tipos + 12 enums + 9 WebhookEventTypes
├── circuitbreaker.go       Circuit breaker (Closed → Open → HalfOpen)
├── router.go               Fallback chain + retry automático
├── registry.go             Discovery + factory de gateways
├── events.go               Webhook normalizer + Redis Pub/Sub subscriber
├── go.mod                  Module Go (com go-redis v9)
├── go.sum
├── pagarme/
│   ├── types.go            20 structs (requests, responses, webhooks)
│   ├── client.go           HTTP client com retry 3x + backoff
│   ├── gateway.go          Implementa 15 métodos da interface Gateway
│   ├── webhook.go          HMAC-SHA256 validation
│   └── gateway_test.go     18 testes ✅
├── asaas/
│   ├── types.go            18 structs
│   ├── client.go           HTTP client com retry 3x
│   ├── gateway.go          Implementa 15 métodos
│   ├── webhook.go          Token validation
│   └── gateway_test.go     20 testes ✅
├── abacatepay/
│   ├── types.go            12 structs
│   ├── client.go           HTTP client com retry 3x
│   ├── gateway.go          Implementa 15 métodos (PIX only)
│   ├── webhook.go          Webhook parsing
│   └── gateway_test.go     8 testes ✅
└── mercadopago/
    ├── types.go            18 structs
    ├── client.go           HTTP client com retry 3x + put/delete
    ├── gateway.go          Implementa 15 métodos
    └── gateway_test.go     8 testes ✅
```

### SQL Migrations Criadas

```
sql/
├── 14_recipients.sql                    Tabela de recebedores multi-gateway
├── 15_split_rules.sql                   Tabela de regras de split
└── 16_payments_gateway_columns.sql      +16 colunas na tabela payments
```

### Arquivos Modificados

```
go.work              Adicionado ./pkg/gateway
render.yaml          Adicionadas 11 env vars (gateways + config)
sql/run_all.sh       Adicionadas migrations 14, 15, 16
README.md            Atualizado com arquitetura multi-gateway
```

### Documentação Criada

```
references/arquitetura-split-pagamentos.md   ~1.650 linhas (doc definitiva)
skills/fuudelivery-multi-gateway/SKILL.md    Este arquivo
```

---

## Interface Gateway (pkg/gateway/gateway.go)

```go
type Gateway interface {
    Name() string

    // Transações
    CreateTransaction(ctx, req) (*TransactionResponse, error)
    CaptureTransaction(ctx, gatewayID, amount) error
    RefundTransaction(ctx, gatewayID, amount) (*RefundResponse, error)
    VoidTransaction(ctx, gatewayID) error
    GetTransactionStatus(ctx, gatewayID) (TransactionStatus, error)

    // Recebedores
    CreateRecipient(ctx, req) (*RecipientResponse, error)
    UpdateRecipient(ctx, recipientID, req) error
    GetRecipientBalance(ctx, recipientID) (int64, int64, error)

    // Webhook
    ValidateWebhook(body, headers) bool
    ParseWebhook(body) (*WebhookEvent, error)

    // Capacidades
    SupportsMethod(PaymentMethod) bool
    SupportsSplit() bool
    SupportsPreAuth() bool
    Supports3DS() bool
    SupportsEscrow() bool
    MaxSplitRecipients() int
}
```

---

## Schema SQL Completo

### Tabela `recipients` (14_recipients.sql)

```sql
CREATE TABLE IF NOT EXISTS recipients (
    id                   BIGSERIAL PRIMARY KEY,
    user_type            VARCHAR(20) NOT NULL,        -- 'establishment' ou 'delivery_man'
    user_id              INTEGER NOT NULL,
    gateway              VARCHAR(20) NOT NULL,        -- 'pagarme','asaas','abacatepay','mercadopago'
    gateway_recipient_id VARCHAR(128) NOT NULL,       -- ID da sub-conta no gateway
    status               VARCHAR(20) NOT NULL DEFAULT 'pending',
    bank_account_last4   VARCHAR(4),
    transfer_interval    VARCHAR(20) DEFAULT 'daily',
    transfer_day         INTEGER,
    metadata             JSONB DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_type, user_id, gateway)
);
```

### Tabela `payment_split_rules` (15_split_rules.sql)

```sql
CREATE TABLE IF NOT EXISTS payment_split_rules (
    id                BIGSERIAL PRIMARY KEY,
    payment_id        BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    recipient_id      BIGINT NOT NULL REFERENCES recipients(id),
    gateway           VARCHAR(20) NOT NULL,
    gateway_split_id  VARCHAR(128),
    percentage        DECIMAL(5,2),
    fixed_value       INTEGER,
    amount            INTEGER NOT NULL,
    status            VARCHAR(20) NOT NULL DEFAULT 'pending',
    processed_at      TIMESTAMPTZ,
    failure_reason    TEXT,
    metadata          JSONB DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(payment_id, recipient_id)
);
```

### Colunas adicionadas em `payments` (16_payments_gateway_columns.sql)

```sql
ALTER TABLE payments ADD COLUMN IF NOT EXISTS gateway VARCHAR(20) DEFAULT 'abacatepay';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS gateway_transaction_id VARCHAR(128);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20) DEFAULT 'pix';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(36);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS authorized_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS captured_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS refunded_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS refund_amount INTEGER DEFAULT 0;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS split_applied BOOLEAN DEFAULT FALSE;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(64);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_expires_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_attempts INTEGER DEFAULT 0;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS card_brand VARCHAR(20);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS card_last4 VARCHAR(4);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS installments INTEGER DEFAULT 1;
```

---

## Variáveis de Ambiente Necessárias

```yaml
# Seleção de gateway
PAYMENT_GATEWAY_PRIMARY=pagarme
PAYMENT_GATEWAY_FALLBACK=asaas
PAYMENT_SPLIT_ENABLED=true
PAYMENT_PIN_REQUIRED=true
PAYMENT_SPLIT_PERCENTAGE_PLATFORM=10

# Pagar.me (principal)
PAGARME_API_KEY=sk_live_xxx           # SECRET
PAGARME_ENCRYPTION_KEY=ek_live_xxx    # SECRET
PAGARME_WEBHOOK_SECRET=whsec_xxx      # SECRET

# Asaas (alternativo)
ASAAS_API_KEY=$aas_xxx                # SECRET
ASAAS_WEBHOOK_TOKEN=xxx               # SECRET
ASAAS_ENVIRONMENT=Production

# AbacatePay (fallback PIX)
ABACATE_PAY_API_KEY=abc_prod_xxx      # Já configurado
ABACATE_PAY_WEBHOOK_SECRET=xxx        # Já configurado

# Mercado Pago (reserva)
MERCADOPAGO_ACCESS_TOKEN=APP_USR-xxx  # SECRET
MERCADOPAGO_WEBHOOK_SECRET=xxx        # SECRET
```

---

## O Que Falta (Próximos Passos)

### 1. ⚠️ RODAR MIGRATIONS NO SUPABASE (IMEDIATO)

Este é o passo mais urgente. As migrations 14-16 precisam ser executadas no banco de produção:

```bash
export DB_CONNECTION_STRING="postgresql://postgres.PROJECT_REF:PASSWORD@aws-0-REGION.pooler.supabase.com:6543/postgres"

# Rodar apenas as novas migrations (14-16)
psql "$DB_CONNECTION_STRING" -f sql/14_recipients.sql
psql "$DB_CONNECTION_STRING" -f sql/15_split_rules.sql
psql "$DB_CONNECTION_STRING" -f sql/16_payments_gateway_columns.sql
```

**OU rodar todas as migrations de uma vez:**
```bash
cd sql && bash run_all.sh
```

**Verificar que rodou:**
```sql
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'public'
AND table_name IN ('recipients', 'payment_split_rules');
-- Deve retornar 2 linhas
```

### 2. Configurar API Keys no Render Dashboard

O `render.yaml` já lista todas as env vars necessárias, mas as secrets precisam ser definidas no Render Dashboard → Environment:

- `PAGARME_API_KEY`
- `PAGARME_ENCRYPTION_KEY`
- `PAGARME_WEBHOOK_SECRET`
- `ASAAS_API_KEY`
- `ASAAS_WEBHOOK_TOKEN`
- `MERCADOPAGO_ACCESS_TOKEN`
- `MERCADOPAGO_WEBHOOK_SECRET`

### 3. Integrar o Router no Monolito (cmd/fuudelivery/main.go)

Adicionar no `main.go` a inicialização do payment router:

```go
// Inicializar gateways
reg := gateway.NewRegistry()
if os.Getenv("PAGARME_API_KEY") != "" {
    pgw, _ := pagarme.NewGateway()
    reg.Register(pgw)
}
if os.Getenv("ASAAS_API_KEY") != "" {
    agw, _ := asaas.NewGateway()
    reg.Register(agw)
}
// ... etc

// Criar router
router := gateway.NewRouter(reg.List()...)
```

### 4. Criar Webhook Endpoints

Adicionar rotas de webhook para cada gateway:
- `POST /payments/webhook/pagarme`
- `POST /payments/webhook/asaas`
- `POST /payments/webhook/abacatepay` (já existe)
- `POST /payments/webhook/mercadopago`

### 5. Implementar Onboarding de Recebedores

Quando um restaurante ou entregador é aprovado, criar sub-conta no gateway:
- `POST /admin/recipients/{userType}/{userId}/onboard`

### 6. Implementar Split no Fluxo de Pagamento

No momento da captura do pagamento, enviar split rules ao gateway:
- PIX: split no webhook de confirmação
- Cartão: split no momento da captura (após PIN)

---

## Convenções do Projeto

- **Linguagem**: Go 1.25 (workspace com 10 módulos)
- **Framework HTTP**: Fiber v2
- **Banco**: PostgreSQL único (Supabase) com GORM
- **Cache/Fila**: Redis (go-redis v8)
- **ORM**: GORM v2 com AutoMigrate
- **Testes**: testify + miniredis (sem Docker)
- **Commits**: Conventional Commits (`feat(payments): ...`)
- **Branch**: `master` (deploy automático via CI)

---

## Comandos Úteis

```bash
# Build
export PATH=/usr/local/go/bin:$PATH
cd cmd/fuudelivery && go build -o ../../server .

# Testar gateway
go test ./pkg/gateway/... -v

# Verificar compilação
go vet ./pkg/gateway/...

# Rodar migrations (precisa DB_CONNECTION_STRING)
cd sql && bash run_all.sh

# Deploy (push para master)
git add -A && git commit -m "feat(payments): ..." && git push origin master
```

---

## Contato / Contexto

- **Projeto**: FuuDelivery (delivery completo)
- **Dono**: Marcos (MarcosPavanBR no GitHub)
- **Produção**: Render (https://fuudelivery-api-8y6l.onrender.com)
- **Banco**: Supabase PostgreSQL
- **Redis**: Externo (*.db.redis.io)
- **Status do deploy**: ✅ `live` (build compila, startup com DB connections em goroutine)
