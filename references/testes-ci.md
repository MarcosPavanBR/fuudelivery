# Testes e CI — FuuDelivery

## Estado atual

### Arquivos de teste existentes (4)

| Arquivo | Módulo | Tipo |
|---|---|---|
| `Backend/auth_api/app/middlewares/jwt_test.go` | auth_api | Unit |
| `Backend/payment_api/app/handlers/wallet_test.go` | payment_api | Unit |
| `Backend/orders_api/app/handlers/pickup_code_test.go` | orders_api | Unit |
| `Frontend/WebRestaurant/src/App.test.js` | WebRestaurant | Smoke (React) |

### CI atual (`.github/workflows/ci.yml`)

- Build/Vet/Test rodam em `cmd/fuudelivery` (monólito)
- Lint com `gofmt`
- **NÃO testa**: `auth_api`, `payment_api`, `orders_api`, `delivery_api`, `chat_api`, `Backend/Payment`
- **NÃO há**: frontend CI, integration tests, coverage reports

## O que precisa ser corrigido

### 1. CI — Testar todos os módulos

O CI atual só executa `go test ./...` dentro de `cmd/fuudelivery`, que é o monólito. Os testes dos módulos separados (`auth_api`, `payment_api`, `orders_api`) ficam de fora.

**Solução**: Adicionar etapas separadas para cada módulo ou usar um matrix strategy.

```yaml
strategy:
  matrix:
    module:
      - cmd/fuudelivery
      - Backend/auth_api
      - Backend/payment_api
      - Backend/orders_api
```

### 2. Cobertura mínima por área de dinheiro

| Área | Fluxo crítico | Testes necessários |
|---|---|---|
| **Pagamento** | Criar → Aprovar → Creditar carteira | Happy path + duplicidade |
| **Carteira** | Credit/Debit atômico | Race condition, saldo insuficiente |
| **Split** | Valor líquido = total - taxa | Cálculo correto, edge cases |
| **Cupons** | Aplicar → Descontar → Validar expiração | Uso múltiplo, valor máximo |
| **Chargeback** | Estornar → Debitar carteira | Saldo insuficiente, duplicidade |

### 3. Testes de integração

- Health check de cada serviço
- Fluxo completo: pedido → pagamento → carteira
- Webhook do AbacatePay → aprovação → split

### 4. Frontend CI

```yaml
- name: Test WebRestaurant
  run: npm test -- --watchAll=false
  working-directory: Frontend/WebRestaurant

- name: Lint WebRestaurant
  run: npm run lint
  working-directory: Frontend/WebRestaurant
```

## Prioridade de implementação

1. **Agora**: Corrigir CI para testar módulos Go existentes
2. **Antes de dinheiro real**: Testes de pagamento (wallet_test.go existente + novos)
3. **Antes de escalar**: Testes de integração com mock do AbacatePay
4. **Backlog**: Frontend CI, E2E tests
