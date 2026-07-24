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

## README desatualizado

O README.md atual reflete o projeto original (vercardapio), não o fork (FuuDelivery). Informações que precisam ser atualizadas:

- [ ] Nome do projeto (vercardapio → FuuDelivery)
- [x] Features novas (pagamento, carteira, chat, rastreio, relatórios, cadastro)
- [x] Arquitetura (5 serviços no Render)
- [ ] Variáveis de ambiente necessárias
- [ ] Guia de setup local atualizado
- [ ] Licença (verificar se mantém MIT)
