# FuuDelivery — Documentacao Completa do Sistema

> **Versao:** 2.0.0  
> **Ultima atualizacao:** 2026-07-22  
> **Autor:** FuuDelivery Team

---

## Sumario

1. [Visao Geral do Sistema](#1-visao-geral-do-sistema)
2. [Arquitetura](#2-arquitetura)
3. [Servicos e Endpoints](#3-servicos-e-endpoints)
4. [Microservico Payment Service](#4-microservico-payment-service)
5. [Frontend WebRestaurant — Painel de Pagamentos](#5-frontend-webrestaurant--painel-de-pagamentos)
6. [Frontend PaymentPanel — Painel Standalone](#6-frontend-paymentpanel--painel-standalone)
7. [Banco de Dados](#7-banco-de-dados)
8. [Autenticacao e Seguranca](#8-autenticacao-e-seguranca)
9. [Fluxo de Pagamento Completo](#9-fluxo-de-pagamento-completo)
10. [Deploy e Infraestrutura](#10-deploy-e-infraestrutura)
11. [Credenciais e Configuracoes](#11-credenciais-e-configuracoes)
12. [Guia de Manutencao](#12-guia-de-manutencao)
13. [Endereco dos Arquivos](#13-endereco-dos-arquivos)

---

## 1. Visao Geral do Sistema

O FuuDelivery e uma plataforma completa de delivery que conecta restaurantes, entregadores e clientes. O sistema inclui:

- **API Principal** (Go monolith): Autenticacao, pedidos, produtos, entregas, chat, pagamentos
- **WebRestaurant**: Painel do restaurante para gerenciar pedidos, cardapio, carteira
- **WebAdmin**: Painel administrativo para gerenciar o plataforma
- **AppComida**: App mobile para clientes (React Native/Expo)
- **AppEntrega**: App mobile para entregadores (React Native/Expo)
- **Payment Service**: Microservico de pagamentos com motor de aprovacao
- **Payment Panel**: Painel standalone de aprovacao de pagamentos

### Stack Tecnologica

| Camada | Tecnologia |
|--------|------------|
| Backend API | Go 1.23 + Fiber + MongoDB + RabbitMQ |
| Payment Service | Go 1.23 + Fiber + MongoDB + RabbitMQ |
| Frontend Web | React 18 + Tailwind CSS + React Router |
| Mobile | React Native + Expo |
| Banco de Dados | MongoDB Atlas (fuudelivery + fuudelivery_payments) |
| Cache | Redis (Render) |
| Gateway Pagamentos | AbacatePay (PIX) + Asaas (Split) |
| Hospedagem | Render.com |

---

## 2. Arquitetura

```
┌─────────────────────────────────────────────────────────────┐
│                        CLIENTES                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ AppComida│  │AppEntrega│  │WebRest.  │  │ WebAdmin │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │
└───────┼──────────────┼──────────────┼──────────────┼────────┘
        │              │              │              │
        ▼              ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────┐
│                    NGINX (Proxy Reverso)                     │
│                    Porta 80/443                              │
└────────────────────────┬────────────────────────────────────┘
                         │
        ┌────────────────┼────────────────┐
        ▼                ▼                ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│  API Main    │ │Payment Service│ │ Payment Panel│
│  (port 3000) │ │ (port 8084)  │ │  (port 80)   │
└──────┬───────┘ └──────┬───────┘ └──────────────┘
       │                │
       ▼                ▼
┌─────────────────────────────────────────────────────────────┐
│                    MongoDB Atlas                             │
│  ┌──────────────┐              ┌──────────────┐             │
│  │  fuudelivery │              │fuudelivery_  │             │
│  │  (principal) │              │  payments    │             │
│  └──────────────┘              └──────────────┘             │
└─────────────────────────────────────────────────────────────┘
                         │
                         ▼
              ┌──────────────────┐
              │     RabbitMQ     │
              │  (message queue) │
              └──────────────────┘
```

### Microservicos

| Servico | Porta | Descricao |
|---------|-------|-----------|
| API Main | 3000 | Monolith principal com todas as APIs |
| Payment Service | 8084 | Motor de aprovacao de pagamentos |
| Nginx | 80/443 | Proxy reverso e rate limiting |
| MongoDB | 27017 | Banco de dados NoSQL |
| RabbitMQ | 5672 | Fila de mensagens assincronas |
| Redis | 12264 | Cache e sessoes |

---

## 3. Servicos e Endpoints

### 3.1 API Principal (Monolith)

**Base URL:** `https://fuudelivery-api-8y6l.onrender.com`

#### Autenticacao
| Metodo | Rota | Descricao |
|--------|------|-----------|
| POST | `/auth/login` | Login do usuario |
| POST | `/auth/register` | Cadastro de usuario |

#### Estabelecimentos
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/establishments` | Listar estabelecimentos |
| GET | `/establishments/:id` | Buscar estabelecimento |
| PUT | `/establishments/:id` | Atualizar estabelecimento |
| PUT | `/establishments/status/handler/:id` | Abrir/fechar estabelecimento |
| PUT | `/establishments/wallet/:id` | Configurar carteira Asaas |

#### Pedidos
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/orders` | Listar pedidos |
| POST | `/orders` | Criar pedido |
| PUT | `/orders/:id/status` | Atualizar status do pedido |
| GET | `/orders/:id` | Buscar pedido |

#### Produtos
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/products` | Listar produtos |
| POST | `/products` | Criar produto |
| PUT | `/products/:id` | Atualizar produto |
| DELETE | `/products/:id` | Remover produto |

#### Entregadores
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/delivery-men` | Listar entregadores |
| POST | `/delivery-men` | Cadastrar entregador |
| PUT | `/delivery-men/:id/location` | Atualizar localizacao GPS |
| PUT | `/delivery-men/wallet/:id` | Configurar carteira Asaas |

#### Chat
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/chat/:orderId` | Buscar mensagens do chat |
| POST | `/chat/:orderId` | Enviar mensagem |
| WS | `/ws/:id` | WebSocket para chat em tempo real |

#### Pagamentos (AbacatePay)
| Metodo | Rota | Descricao |
|--------|------|-----------|
| POST | `/payments/pix` | Criar pagamento PIX |
| POST | `/payments/card` | Criar pagamento cartao |
| GET | `/payments/:id` | Buscar pagamento |
| POST | `/webhooks/abacatepay` | Webhook do gateway |

#### Split Payments (Asaas)
| Metodo | Rota | Descricao |
|--------|------|-----------|
| POST | `/asaas/wallet` | Criar carteira Asaas |
| GET | `/asaas/wallet/:id` | Status da carteira |
| POST | `/asaas/split` | Criar split payment |

#### Fidelidade
| Metodo | Rota | Descricao |
|--------|------|-----------|
| POST | `/loyalty/earn` | Ganhar pontos |
| POST | `/loyalty/redeem` | Resgatar pontos por cupom |
| GET | `/loyalty/:customerId` | Saldo de pontos |

#### Cupons
| Metodo | Rota | Descricao |
|--------|------|-----------|
| POST | `/coupons` | Criar cupom |
| POST | `/coupons/validate` | Validar cupom |
| POST | `/coupons/validate-internal` | Validacao interna |

---

### 3.2 Payment Service (Microservico)

**Base URL:** `https://fuudelivery-payment.onrender.com`

#### Health Check
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/health` | Status do servico |

#### Autenticacao
| Metodo | Rota | Descricao |
|--------|------|-----------|
| POST | `/api/auth/login` | Login no painel de pagamentos |

#### Pagamentos
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/api/payments` | Listar pagamentos (filtros: status, risk_level, establishment_id) |
| GET | `/api/payments/stats` | Estatisticas de pagamentos |
| GET | `/api/payments/:id` | Buscar pagamento |
| POST | `/api/payments` | Criar pagamento |
| POST | `/api/payments/:id/approve` | Aprovar pagamento manualmente |
| POST | `/api/payments/:id/reject` | Rejeitar pagamento |

#### Aprovacoes
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/api/approvals/queue` | Fila de aprovacao (pagamentos pendentes) |
| GET | `/api/approvals/auto-approved` | Pagamentos auto-aprovados |
| GET | `/api/approvals/rules` | Regras de aprovacao |
| PUT | `/api/approvals/rules` | Atualizar regras |

#### Estornos (Chargebacks)
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/api/chargebacks` | Listar estornos |
| GET | `/api/chargebacks/stats` | Estatisticas de estornos |
| GET | `/api/chargebacks/:id` | Buscar estorno |
| POST | `/api/chargebacks` | Criar estorno |
| POST | `/api/chargebacks/:id/approve` | Aprovar estorno |
| POST | `/api/chargebacks/:id/reject` | Rejeitar estorno |
| POST | `/api/chargebacks/:id/evidence` | Adicionar evidencia |
| GET | `/api/chargebacks/:id/evidence` | Listar evidencias |

#### Carteiras (Wallets)
| Metodo | Rota | Descricao |
|--------|------|-----------|
| GET | `/api/wallets/:user_id` | Saldo da carteira |
| GET | `/api/wallets/:user_id/transactions` | Historico de transacoes |
| POST | `/api/wallets/:user_id/credit` | Creditar carteira |
| POST | `/api/wallets/:user_id/debit` | Debitar carteira |
| GET | `/api/wallets/:user_id/get-or-create` | Obter ou criar carteira |

---

## 4. Microservico Payment Service

### 4.1 Arquitetura Interna

```
Backend/Payment/
├── main.go                    # Ponto de entrada, configura Fiber e rotas
├── config/
│   └── config.go              # Carrega variaveis de ambiente
├── models/
│   ├── payment.go             # Modelo de pagamento com RiskLevel e Status
│   ├── chargeback.go          # Modelo de estorno/disputa
│   ├── wallet.go              # Modelo de carteira e transacoes
│   ├── evidence.go            # Modelo de evidencia para disputas
│   └── user.go                # Modelo de usuario do painel
├── repository/
│   ├── mongo.go               # Conexao MongoDB e criacao de indices
│   ├── payment_repo.go        # CRUD de pagamentos
│   ├── chargeback_repo.go     # CRUD de estornos
│   ├── wallet_repo.go         # CRUD de carteiras
│   ├── evidence_repo.go       # CRUD de evidencias
│   └── user_repo.go           # CRUD de usuarios
├── services/
│   ├── risk_scorer.go         # Calculo de score de risco (0-100)
│   ├── approval_engine.go     # Motor de decisao de aprovacao
│   ├── gateway_service.go     # Integracao com AbacatePay API
│   ├── wallet_service.go      # Logica de credito/debito
│   ├── responsibility_chain.go # Cadeia de responsabilidade (R->E->C)
│   ├── chargeback_service.go  # Logica de estornos
│   └── user_service.go        # Servico de autenticacao
├── handlers/
│   ├── payment_handler.go     # Endpoints de pagamentos
│   ├── approval_handler.go    # Endpoints de aprovacoes
│   ├── chargeback_handler.go  # Endpoints de estornos
│   ├── wallet_handler.go      # Endpoints de carteiras
│   └── user_handler.go        # Endpoints de autenticacao
├── consumers/
│   └── payment_consumer.go    # Consumer RabbitMQ para pagamentos aprovados
├── middleware/
│   └── auth.go                # Middleware JWT de autenticacao
├── go.mod                     # Dependencias Go
└── Dockerfile                 # Build multi-stage para producao
```

### 4.2 Motor de Aprovacao (ApprovalEngine)

O motor de aprovacao e o componente central do Payment Service. Ele decide automaticamente se um pagamento deve ser:

1. **Auto-Aprovado** (status: `approved`): Score baixo + valor baixo + sem reclamacoes
2. **Analise Manual** (status: `pending`): Score medio OU alto valor
3. **Compliance** (status: `pending`): Score alto OU estorno ativo
4. **Bloqueado** (status: `rejected`): Fraude confirmada

#### Regras de Aprovacao

| Condicao | Resultado |
|----------|-----------|
| Valor < R$ 1.000 AND Score < 20 AND sem reclamacoes | Auto-Aprovado |
| Valor > R$ 5.000 | Analise Manual |
| Score > 60 | Analise Manual |
| Score > 80 | Compliance |
| Estorno ativo | Bloqueado |
| Reclamacao nos ultimos 30 dias | Analise Manual |

#### Calculo de Score de Risco

O score vai de 0 a 100 e e calculado baseado em:

| Fator | Peso | Descricao |
|-------|------|-----------|
| Valor do pagamento | 0-30 | Acima de R$ 500 = 30pts, R$ 200-500 = 20pts, R$ 100-200 = 10pts |
| Frequencia do cliente | 0-25 | Mais de 10 pedidos/dia = 25pts, 5-10 = 15pts |
| Horario | 0-15 | Entre 1h-5h = 15pts (horario suspeito) |
| Historico do restaurante | 0-20 | Mais de 5 estornos = 20pts, 2-5 = 10pts |

#### Niveis de Risco

| Score | Nivel | Acao |
|-------|-------|------|
| 0-19 | `low` | Auto-aprovado |
| 20-59 | `medium` | Analise manual |
| 60-79 | `high` | Compliance |
| 80-100 | `critical` | Bloqueado |

### 4.3 Cadeia de Responsabilidade (Responsibility Chain)

O sistema implementa um padrao Chain of Responsibility para verificar a responsabilidade de cada parte no fluxo de pagamento:

```
Restaurante (R) → Entregador (E) → Cliente (C)
```

Cada componente pode ter status:
- `ok`: Tudo correto
- `warn`: Atencao necessaria
- `err`: Problema identificado

Exemplos:
- `R:ok → E:ok → C:ok` = Pagamento auto-aprovado
- `R:ok → E:err → C:warn` = Compliance (entrega com problema + reclamacao)
- `R:err → E:warn → C:err` = Bloqueado (restaurante com problemas + fraude)

### 4.4 Integração com RabbitMQ

O Payment Service consome mensagens da fila `payment_queue`:

```go
// Quando um pagamento e aprovado, credit automaticamente na carteira
func (pc *PaymentConsumer) processMessage(msg amqp.Delivery) {
    var payment models.Payment
    json.Unmarshal(msg.Body, &payment)
    
    if payment.Status == "approved" {
        pc.Wallet.ProcessPaymentApproval(&payment)
    }
}
```

---

## 5. Frontend WebRestaurant — Painel de Pagamentos

### 5.1 Estrutura

```
Frontend/WebRestaurant/src/
├── pages/payments/
│   ├── PaymentsPage.js          # Container com 5 abas
│   ├── PaymentDashboard.js      # Dashboard principal
│   ├── PaymentApprovals.js      # Regras de aprovacao
│   ├── PaymentChargebacks.js    # Gestao de estornos
│   ├── PaymentWorkflow.js       # Diagrama de fluxo
│   └── PaymentResponsibility.js # Cadeia de responsabilidade
├── services/
│   └── paymentApi.js            # Cliente API para Payment Service
├── styles/
│   └── payments.css             # Estilos dark theme (pp-*)
├── Routes.js                    # Rota /pagamentos
└── components/Menu.js           # Item "Pagamentos" no menu
```

### 5.2 Abas do Painel

1. **Dashboard**: Metricas, tabela de pagamentos, modal de detalhes, acoes aprovar/rejeitar
2. **Aprovacoes**: Regras de auto-aprovacao vs analise manual
3. **Estornos**: Cards de estorno, tabela de disputas, evidencias
4. **Workflow**: Diagrama de 8 etapas do fluxo de pagamento
5. **Responsabilidade**: Cards de responsabilidade (R/E/C), matriz de atribuicao

### 5.3 API Service

```javascript
// paymentApi.js - Conecta ao Payment Service
const PAYMENT_API_URL = process.env.REACT_APP_PAYMENT_API_URL || 'http://localhost:8084';

export const PaymentService = {
  getPayments: (params) => paymentApi.get('/api/payments', { params }),
  approvePayment: (id) => paymentApi.post(`/api/payments/${id}/approve`),
  rejectPayment: (id, reason) => paymentApi.post(`/api/payments/${id}/reject`, { reason }),
  // ... outros endpoints
};
```

---

## 6. Frontend PaymentPanel — Painel Standalone

### 6.1 Arquitetura

```
Frontend/PaymentPanel/
├── index.html      # App completa em HTML/CSS/JS puro
├── package.json    # Build script para Render
└── Dockerfile      # Container nginx
```

### 6.2 Funcionalidades

- Login com JWT
- Dashboard com metricas em tempo real
- Tabela de pagamentos com filtros
- Modal de detalhes com timeline
- Acoes de aprovar/rejeitar
- Toast notifications
- Dark theme completo

---

## 7. Banco de Dados

### 7.1 Database: fuudelivery (Principal)

| Colecao | Descricao |
|---------|-----------|
| `users` | Usuarios do sistema |
| `establishments` | Restaurantes |
| `delivery_men` | Entregadores |
| `orders` | Pedidos |
| `products` | Produtos do cardapio |
| `categories` | Categorias de produtos |
| `coupons` | Cupons de desconto/cashback |
| `chats` | Mensagens de chat |

### 7.2 Database: fuudelivery_payments

| Colecao | Descricao |
|---------|-----------|
| `payments` | Pagamentos processados |
| `chargebacks` | Estornos e disputas |
| `wallets` | Carteiras de restaurantes/entregadores |
| `wallet_transactions` | Historico de transacoes |
| `evidences` | Evidencias para disputas |
| `users` | Usuarios do painel de pagamentos |

### 7.3 Indices

```javascript
// payments
{ order_id: 1 }           // unique
{ customer_id: 1 }
{ establishment_id: 1 }
{ status: 1 }
{ risk_level: 1 }
{ created_at: -1 }

// wallets
{ user_id: 1 }            // unique

// wallet_transactions
{ wallet_id: 1 }
{ created_at: -1 }
```

---

## 8. Autenticacao e Seguranca

### 8.1 JWT Authentication

```go
// Middleware de autenticacao
func AuthRequired() fiber.Handler {
    return func(c *fiber.Ctx) error {
        token := extractToken(c.Get("Authorization"))
        claims := validateJWT(token)
        c.Locals("user_id", claims.UserID)
        c.Locals("role", claims.Role)
        return c.Next()
    }
}
```

### 8.2 Roles

| Role | Permissoes |
|------|------------|
| `admin` | Acesso total |
| `operator` | Aprovar/rejeitar pagamentos |
| `viewer` | Apenas visualizar |

### 8.3 CORS

```go
cors.New(cors.Config{
    AllowOrigins: "*",  // Em producao: dominios especificos
    AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
    AllowHeaders: "Origin,Content-Type,Accept,Authorization",
})
```

---

## 9. Fluxo de Pagamento Completo

```
1. Cliente faz pedido
   ↓
2. Pagamento criado (PIX ou Cartao)
   ↓
3. Webhook do gateway confirma pagamento
   ↓
4. Pagamento salvo no MongoDB (status: pending)
   ↓
5. ApprovalEngine verifica:
   ├── Score de risco
   ├── Valor do pagamento
   ├── Historico do cliente/restaurante
   └── Cadeia de responsabilidade
   ↓
6. Decisao:
   ├── Auto-aprovado → Credita na carteira automaticamente
   ├── Analise manual → Fila de aprovacao no painel
   ├── Compliance → Apenas admin pode resolver
   └── Bloqueado → Pagamento rejeitado
   ↓
7. Se aprovado → RabbitMQ publica evento
   ↓
8. Consumer credita na carteira do restaurante
   ↓
9. Restaurante pode solicitar saque
```

---

## 10. Deploy e Infraestrutura

### 10.1 Servicos no Render

| Servico | Tipo | URL |
|---------|------|-----|
| fuudelivery-api | Web Service | https://fuudelivery-api-8y6l.onrender.com |
| fuudelivery-web | Static Site | https://fuudelivery-web.onrender.com |
| fuudelivery-admin | Static Site | https://fuudelivery-admin-lv7f.onrender.com |
| fuudelivery-payment | Web Service | https://fuudelivery-payment.onrender.com |
| fuudelivery-payment-panel | Static Site | https://fuudelivery-payment-panel.onrender.com |

### 10.2 Variaveis de Ambiente

Ver `.fuudelivery-config/CREDENTIALS.md` para todas as variaveis.

### 10.3 Deploy Automatico

O auto-deploy esta habilitado para todos os servicos. Quando voce faz push para a branch `master`, o Render automaticamente:

1. Detecta a mudanca no repositorio
2. Faz build do codigo
3. Deploya a nova versao
4. Verifica a saude do servico

### 10.4 Rollback

Para reverter um deploy:
1. Acesse o dashboard do Render
2. Va para o servico
3. Clique em "Manual Deploy" → "Rollback to previous deploy"

---

## 11. Credenciais e Configuracoes

Todas as credenciais estao salvas em:
- **`.fuudelivery-config/CREDENTIALS.md`**: Credenciais completas
- **`.env`**: Variaveis de ambiente locais
- **Render Dashboard**: Variaveis de ambiente de producao

### Chave de API Render

```
rnd_uWc5UfLvn8OFUxcagUtwHSMYxFWN
```

### MongoDB Atlas

```
mongodb+srv://pavanbrtl050_db_user:q0RGpDo30LXVI0eS@fuudelivery.hj0pytw.mongodb.net/fuudelivery
```

### AbacatePay

```
API Key: abc_prod_uCfXQTJqBxJgY5MQLxSKghPL
Webhook Secret: whsec_fuudelivery_prod_2024
```

---

## 12. Guia de Manutencao

### 12.1 Adicionar Nova Rota no Payment Service

1. Crie o handler em `Backend/Payment/handlers/`
2. Registre a rota em `Backend/Payment/main.go`
3. Teste localmente com `go run .`
4. Faca push para deploy automatico

### 12.2 Alterar Regras de Aprovacao

1. Acesse o Payment Panel
2. Va na aba "Aprovacoes"
3. As regras podem ser atualizadas via API:
```bash
curl -X PUT https://fuudelivery-payment.onrender.com/api/approvals/rules \
  -H "Authorization: Bearer <token>" \
  -d '{"auto_approve_max_amount": 2000}'
```

### 12.3 Monitorar Pagamentos

1. Acesse o Payment Panel: https://fuudelivery-payment-panel.onrender.com
2. Ou via API:
```bash
curl https://fuudelivery-payment.onrender.com/api/payments/stats \
  -H "Authorization: Bearer <token>"
```

### 12.4 Verificar Logs

No Render Dashboard:
1. Acesse o servico
2. Va na aba "Logs"
3. Filtre por nivel (info, warn, error)

### 12.5 Adicionar Novo Endpoint

1. Crie o arquivo em `handlers/`
2. Adicione a rota em `main.go`
3. Execute `go vet ./...` para verificar erros
4. Faca push

---

## 13. Endereco dos Arquivos

### Backend

| Arquivo | Caminho | Descricao |
|---------|---------|-----------|
| main.go | `Backend/Payment/main.go` | Ponto de entrada do Payment Service |
| config.go | `Backend/Payment/config/config.go` | Configuracoes |
| payment.go | `Backend/Payment/models/payment.go` | Modelo de pagamento |
| risk_scorer.go | `Backend/Payment/services/risk_scorer.go` | Calculo de risco |
| approval_engine.go | `Backend/Payment/services/approval_engine.go` | Motor de aprovacao |
| auth.go | `Backend/Payment/middleware/auth.go` | Middleware JWT |
| mongo.go | `Backend/Payment/repository/mongo.go` | Conexao MongoDB |

### Frontend

| Arquivo | Caminho | Descricao |
|---------|---------|-----------|
| PaymentsPage.js | `Frontend/WebRestaurant/src/pages/payments/PaymentsPage.js` | Container principal |
| PaymentDashboard.js | `Frontend/WebRestaurant/src/pages/payments/PaymentDashboard.js` | Dashboard |
| paymentApi.js | `Frontend/WebRestaurant/src/services/paymentApi.js` | Cliente API |
| payments.css | `Frontend/WebRestaurant/src/styles/payments.css` | Estilos dark theme |
| index.html | `Frontend/PaymentPanel/index.html` | Painel standalone |

### Configuracao

| Arquivo | Caminho | Descricao |
|---------|---------|-----------|
| CREDENTIALS.md | `.fuudelivery-config/CREDENTIALS.md` | Todas as credenciais |
| render.yaml | `render.yaml` | Configuracao Render |
| docker-compose.payment.yml | `docker-compose.payment.yml` | Compose para Payment |
| go.work | `go.work` | Workspace Go |

---

*Documento gerado automaticamente em 2026-07-22*
