# Testes e CI — FuuDelivery

> ⚠️ **Estado dos bancos nos testes (atualizado 2026-09-11):** a suíte de integração
> usa **Postgres real** (testcontainers/`docker run`) — **nenhum teste sobe MongoDB**.
> O Atlas foi aposentado (banco único = Postgres) e o `Payment` isolado foi removido;
> seções abaixo que citam MongoDB referem-se ao código arquivado em `legacy/Payment`.

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

#### Go (Payment Service — arquivado)

> ⚠️ `Backend/payment_api (monolith)` foi **arquivado e removido** do repositório. Os testes
> daquele serviço (risk scoring, carteiras, chargebacks) não rodam mais no CI;
> os fluxos equivalentes vivem em `payment_api` (embutido no monolito).

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

#### 1. Testes de integração para chargeback com MongoDB — OBSOLETO
> ✅ Obsoleta: o chargeback vivia no `Payment` arquivado; não há mais o que integrar
> com MongoDB. Em `payment_api`, o equivalente (estorno/débito de carteira) já tem
> cobertura de idempotência em `wallet_test.go`/`wallet_idempotency_test.go`.",

#### 2. Testes E2E completos
- Fluxo: pedido → pagamento → aprovação → split → carteira
- Requer: mock do AbacatePay + Postgres + Redis (Mongo não é mais necessário)
- Parcialmente atendido pelos E2E de checkout/webhook (`TestCheckoutE2E_*`, Postgres real)

#### 3. Frontend CI mais completo
- Hoje: só WebRestaurant tem test + build
- Pendente: WebAdmin e PaymentPanel

#### 4. Shared MongoDB container — OBSOLETO
> ✅ Obsoleta junto com o Atlas: a suíte compartilha Postgres por job e o tempo de CI
> já reflete isso. Encerrado sem ação.

---

## Como rodar os testes

### Todos os módulos Go (local)

```bash
cd C:\Users\acastro\Downloads\fuudelivery
go test ./...
```

### Testes de integração (requer Docker)

```bash
cd cmd/fuudelivery && go test -tags=integration -v -run 'TestFullFlow|TestErrorScenarios|TestAdminBootstrap' ./
cd Backend/payment_api && go test -tags=integration -v -run 'TestCheckoutE2E' ./app/handlers/
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

*Última atualização: 2026-09-11 (Mongo/Atlas aposentado nas seções de backlog)*
