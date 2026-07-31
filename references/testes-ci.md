# Testes e CI — FuuDelivery

## Estado atual (2026-07-31)

### CI atual (`.github/workflows/ci.yml`)

#### Jobs implementados

| Job | Tipo | Módulos | Status |
|---|---|---|---|
| `go-modules` | Matrix (7 paralelos) | cmd/fuudelivery, Payment, auth_api, payment_api, orders_api, delivery_api, chat_api | ✅ |
| `lint` | Único | gofmt em Backend/ e cmd/ | ✅ |
| `govulncheck` | Matrix (7 paralelos) | Mesmos 7 módulos Go | ✅ |
| `frontend-webrestaurant` | Único | Frontend/WebRestaurant (test + build) | ✅ |
| `npm-audit` | Matrix (3 paralelos) | WebRestaurant, WebAdmin, PaymentPanel | ✅ |

#### O que cada job faz

**go-modules** (matrix):
```yaml
steps:
  - go mod tidy
  - go build ./...
  - go vet ./...
  - go test ./... -count=1 -timeout 60s
```

**govulncheck** (matrix):
```yaml
steps:
  - go install golang.org/x/vuln/cmd/govulncheck@latest
  - govulncheck ./...
```

**frontend-webrestaurant**:
```yaml
steps:
  - npm install
  - npm test -- --watchAll=false
  - npm run build
```

### Arquivos de teste existentes

#### Go (Payment Service — `Backend/Payment/`)

| Arquivo | Tipo | Testes | O que testa |
|---|---|---|---|
| `services/risk_scorer_test.go` | Unit | 11 | calculateLevel, NormalizeScore, checkAmount, checkTimeOfDay |
| `services/wallet_service_test.go` | Unit | 14 | Validação de input, ProcessPaymentApproval lógica, WalletBalance |
| `services/chargeback_service_test.go` | Unit | 8 | Status, Reasons, valid transitions, amount boundaries |
| `services/responsibility_chain_test.go` | Unit | 12 | ValidationHandler, ApprovalHandler, NotificationHandler, encadeamento |
| `services/integration_test.go` | Integration | 7 | Happy path, idempotência, saldo insuficiente, concorrência, múltiplos pagamentos |

#### Go (outros módulos)

| Arquivo | Módulo | Tipo |
|---|---|---|
| `auth_api/app/middlewares/jwt_test.go` | auth_api | Unit |
| `payment_api/app/handlers/wallet_test.go` | payment_api | Unit |
| `payment_api/app/handlers/card_test.go` | payment_api | Unit |
| `payment_api/app/handlers/pix_test.go` | payment_api | Unit |
| `payment_api/app/handlers/split_test.go` | payment_api | Unit |
| `payment_api/app/handlers/webhook_test.go` | payment_api | Unit |
| `orders_api/app/handlers/pickup_code_test.go` | orders_api | Unit |
| `orders_api/app/handlers/coupon_test.go` | orders_api | Unit |
| `orders_api/app/handlers/loyalty_test.go` | orders_api | Unit |
| `orders_api/app/handlers/orders_test.go` | orders_api | Unit |
| `orders_api/app/handlers/integration_test.go` | orders_api | Integration |

#### Frontend

| Arquivo | Módulo | Tipo |
|---|---|---|
| `Frontend/WebRestaurant/src/App.test.js` | WebRestaurant | Smoke (React) |

### Cobertura por área de dinheiro

| Área | Fluxo crítico | Testes | Status |
|---|---|---|---|
| **Pagamento** | Criar → Aprovar → Creditar carteira | integration_test.go (happy path + idempotência) | ✅ |
| **Carteira** | Credit/Debit atômico | integration_test.go (saldo insuficiente, concorrência) | ✅ |
| **Split** | Valor líquido = total - taxa | split_test.go (payment_api) | ✅ |
| **Cupons** | Aplicar → Descontar → Validar expiração | coupon_test.go (orders_api) | ✅ |
| **Chargeback** | Estornar → Debitar carteira | chargeback_service_test.go (unit) | ⚠️ Só unit |
| **Fidelidade** | Ganhar/Resgatar pontos | loyalty_test.go (orders_api) | ✅ |

### O que falta (backlog)

#### 1. Testes de integração para chargeback com MongoDB
- Hoje: só testes unitários do chargeback_service
- Necessário: teste de integração que cria chargeback → debita carteira → verifica saldo

#### 2. Testes E2E completos
- Fluxo: pedido → pagamento → aprovação → split → carteira
- Requer: mock do AbacatePay + MongoDB + Redis

#### 3. Frontend CI mais completo
- Hoje: só WebRestaurant tem test + build
- Pendente: WebAdmin e PaymentPanel

#### 4. Shared MongoDB container
- Cada teste de integração sobe um container separado (~5-10s cada)
- Ideal: TestMain ou Repository struct para compartilhar
- Reduziria tempo de CI significativamente

---

## Como rodar os testes

### Todos os módulos Go (local)

```bash
cd C:\Users\acastro\Downloads\fuudelivery
go test ./...
```

### Módulo individual

```bash
cd Backend/Payment
go test ./...
```

### Testes de integração (requer Docker)

```bash
cd Backend/Payment
go test -tags=integration ./services/... -v
```

### Frontend

```bash
cd Frontend/WebRestaurant
npm test
```

### CI local (simular GitHub Actions)

```bash
# Verificar formatação
gofmt -l -s Backend/ cmd/

# Verificar vulnerabilidades
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

---

*Última atualização: 2026-07-31*
