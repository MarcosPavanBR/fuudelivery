# Gaps Funcionais — FuuDelivery

## TODOs (resolvidos ✅)

### 1. Pagamento — Ponte entre monólito e Payment Service ✅

**Resolvido em**: commit `2b45b15`

**Solução implementada**: `publishToPaymentQueue()` em `payment_api/app/handlers/webhook.go`
- Quando o webhook do AbacatePay confirma um pagamento, publica em `RABBIT_PAYMENT_QUEUE`
- O `PaymentConsumer` no `Backend/Payment` consome a mensagem e credita na carteira do restaurante
- Se RabbitMQ não estiver configurado, a mensagem é ignorada silenciosamente

### 2. Página de cadastro de restaurante ✅

**Resolvido em**: commit `2b45b15`

**Solução implementada**: `Frontend/WebRestaurant/src/pages/registration/RegisterEstablishment.js`
- Formulário completo: nome, responsável, email, senha, telefone, endereço, horários
- Validação client-side, tratamento de erros, toast notifications
- Rota pública `#/cadastrar-restaurante` (sem autenticação)

### 3. Feature de relatórios ✅

**Resolvido em**: commits `2b45b15` + `7b6bf02`

**Solução implementada**:
- **Frontend**: `Frontend/WebRestaurant/src/pages/reports/Reports.js`
  - Cards de estatísticas (receita, pedidos, ticket médio, entrega)
  - Seletor de período (semana/mês/trimestre/ano)
  - Gráfico de receita diária (barras horizontais)
  - Pedidos por status (entregues/pendentes/cancelados)
  - Rota `#/relatorios` + link no sidebar
- **Backend**: `GET /api/reports/establishment/:id?period=month`
  - `repository/report_repo.go`: MongoDB aggregation pipeline
  - `handlers/report_handler.go`: Handler HTTP com validação
  - 17 testes de integração

### 4. CI/CD — Testar todos os módulos Go ✅

**Resolvido em**: 2026-07-26

**Solução implementada**: `.github/workflows/ci.yml`
- Matrix strategy para testar 7 módulos Go em paralelo
- Fail-fast: false para não parar se um módulo falhar
- Módulos: cmd/fuudelivery, Backend/Payment, auth_api, payment_api, orders_api, delivery_api, chat_api

### 5. Vulnerability scanning no CI ✅

**Resolvido em**: 2026-07-26

**Solução implementada**:
- **govulncheck**: Matrix strategy paralela para todos os 7 módulos Go
- **npm audit**: Roda para WebRestaurant, WebAdmin, PaymentPanel

### 6. Frontend CI ✅

**Resolvido em**: 2026-07-26

**Solução implementada**:
- Job separado `frontend-webrestaurant` com: npm install → npm test → npm run build
- npm audit em paralelo para 3 frontends

### 7. Integration tests do Payment Service ✅

**Resolvido em**: 2026-07-26

**Solução implementada**: `Backend/Payment/services/integration_test.go`
- Testes com MongoDB real via testcontainers
- Cobre: happy path, idempotência, saldo insuficiente, créditos concorrentes, múltiplos pagamentos

### 8. Bug fix: GetWalletTransactions não aplicava limit ✅

**Resolvido em**: 2026-07-26

**Bug**: `repository/wallet_repo.go` validava o parâmetro `limit` mas nunca passava para o `Find()` do MongoDB. Resultado: todas as transações eram carregadas na memória.

**Fix**: Adicionado `options.Find().SetSort().SetLimit()` na query.

---

## Duplicação: payment_api vs Backend/Payment

### O que existe

| Módulo | Localização | Banco | Escopo |
|---|---|---|---|
| `payment_api` | `Backend/payment_api/` | PostgreSQL (Supabase) | Processamento de pagamento (criar, webhook AbacatePay) |
| `Payment` | `Backend/Payment` | MongoDB (Atlas) | Painel de aprovação, carteiras, score de risco, chargebacks, relatórios |

### Documentação da separação

**`payment_api`** (monólito):
- Recebe pedidos do frontend
- Cria cobranças via AbacatePay (PIX/cartão)
- Processa webhooks de confirmação
- Publica em `RABBIT_ORDER_QUEUE` (para orders_api) e `RABBIT_PAYMENT_QUEUE` (para Payment Service)
- Calcula split de pagamento
- Callback de loyalty points

**`Payment`** (microsserviço):
- Consome mensagens da fila de pagamentos
- Aprova/rejeita pagamentos (automático ou manual)
- Calcula score de risco (4 fatores)
- Gerencia carteiras digitais (credit/debit atômico)
- Processa chargebacks
- Gera relatórios de vendas

### Status: Documentado e funcional

A separação é intencional e benéfica:
- `payment_api` é leve e rápido (gateway)
- `Payment` é pesado e analítico (approvals, wallets, reports)
- Conectados via RabbitMQ com fallback para memória

---

## README atualizado ✅

O README.md foi atualizado em 2026-07-26 para refletir o FuuDelivery:
- [x] Nome do projeto (vercardapio → FuuDelivery)
- [x] Features novas (pagamento, carteira, chat, rastreio, relatórios, cadastro)
- [x] Arquitetura (7 módulos Go + 5 frontends)
- [x] Variáveis de ambiente necessárias
- [x] Guia de setup local atualizado
- [x] Licença (MIT)

---

## Ainda pendente

### Rate limiting (P1)

- [ ] Implementar rate limiting para login, registro, pagamento e carteira
- [ ] Ver `references/seguranca.md` para especificações

### Shared MongoDB container nos testes (tech-debt)

- [ ] Cada teste de integração sobe um container MongoDB separado
- [ ] Ideal: usar TestMain ou Repository struct para compartilhar container
- [ ] Reduziría tempo de CI de ~5min para ~1min

### Repository struct/interface (tech-debt)

- [ ] Introduzir `Repository` struct para desacoplar globals do `repository/mongo.go`
- [ ] Permitir injeção de dependência nos service constructors
- [ ] Melhorar testabilidade e reduzir acoplamento

---

*Última atualização: 2026-07-26*
