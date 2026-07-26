# FuuDelivery

Plataforma de delivery completa com pagamento integrado, split, carteira digital, cashback, cupons, chat, rastreio e painel de pagamentos.

Fork do [vercardapio/appdelivery](https://github.com/carloshomar/appdelivery) estendido com sistema de pagamentos e infra de producao no Render.

## Arquitetura

```
  AppComida (React Native)    AppEntrega (React Native)    WebRestaurant (React)
  App de cliente               App de entregador            Kanban + Cardapio + Carteira + Relatorios
          |                            |                              |
          +----------------------------+------------------------------+
          |
  API Gateway (Go + Fiber)  ---->  Payment Service (Go + Fiber)
  MongoDB (Atlas)                  MongoDB (Atlas)
  Redis (Render)                   RabbitMQ
```

## Servicos (Render)

| Servico | Tipo | URL |
|---|---|---|
| fuudelivery-api | Go web service | https://fuudelivery-api-8y6l.onrender.com |
| fuudelivery-payment | Go web service | https://fuudelivery-payment.onrender.com |
| fuudelivery-web | Static site | https://fuudelivery-web.onrender.com |
| fuudelivery-admin | Static site | https://fuudelivery-admin-lv7f.onrender.com |
| fuudelivery-payment-panel | Static site | https://fuudelivery-payment-panel.onrender.com |
| fuudelivery-redis | Redis | Gerenciado pelo Render |

## Features

### Pagamento
- PIX e Cartao via AbacatePay
- Split de pagamento: 5% plataforma, 85% restaurante, taxa de entrega
- Score de risco: 4 fatores (valor, frequencia, historico, horario)
- Aprovacao automatica (baixo risco) ou manual (alto risco)
- Carteira digital com operacoes atomicas ($inc no MongoDB)
- Cashback e cupons de desconto
- Saque via PIX

### Comunicacao
- RabbitMQ: fila entre monolito e Payment Service
- WebSocket: atualizacoes em tempo real
- Redis: fila/pubsub com fallback para canais Go em memoria

### Frontend
- **AppComida** (React Native/Expo): app do cliente
- **AppEntrega** (React Native/Expo): app do entregador
- **WebRestaurant** (React + Tailwind): kanban, cardapio, carteira, relatorios, cadastro
- **WebAdmin** (React): painel administrativo
- **PaymentPanel** (HTML/JS): painel standalone de aprovacao de pagamentos

### Backend (microservicos Go)

| Modulo | Descricao |
|---|---|
| `cmd/fuudelivery` | Monolith principal com auth, pedidos, produtos, entregas, chat |
| `Backend/Payment` | Motor de aprovacao de pagamentos, carteiras, chargebacks, relatorios |
| `Backend/auth_api` | Autenticacao JWT, CRUD de usuarios e estabelecimentos |
| `Backend/payment_api` | Gateway de pagamento (AbacatePay/PIX/Cartao), webhook, split |
| `Backend/orders_api` | Pedidos, produtos, categorias, cupons, fidelidade |
| `Backend/delivery_api` | Rastreio de entregadores, calculo de rota |
| `Backend/chat_api` | Chat em tempo real entre cliente/restaurante |

### Seguranca
- JWT com validacao de SigningMethod
- Rate limiting: login 5req/min, pagamento 10req/min, carteira 20req/min
- CORS restrito a dominios conhecidos
- Senhas hasheadas com bcrypt
- Wallet com operacoes atomicas (eliminacao de race conditions)
- CI com govulncheck e npm audit

## Como Rodar

### Backend (Docker)

```bash
cd Backend
docker compose up --build
```

### Backend (servico individual — Payment)

```bash
cd Backend/Payment
go mod tidy
go run main.go
```

### Backend (servico individual — Monolith)

```bash
cd cmd/fuudelivery
go mod tidy
go run main.go
```

### Frontend (WebRestaurant)

```bash
cd Frontend/WebRestaurant
npm install
npm start
```

### Frontend (PaymentPanel)

```bash
cd Frontend/PaymentPanel
# Abra index.html no navegador
```

### Apps Mobile

```bash
cd Frontend/AppComida
npm install
npx expo start
```

## Variaveis de Ambiente

### API (fuudelivery-api)

| Variavel | Descricao |
|---|---|
| DATABASE_URL | PostgreSQL (Supabase) |
| REDIS_URL | Redis (Render) |
| JWT_SECRET | Secret para tokens JWT |
| MONGODB_URI | MongoDB Atlas |
| MONGO_DATABASE | fuudelivery |
| ABACATE_PAY_API_KEY | API key do AbacatePay |
| ABACATE_PAY_WEBHOOK_SECRET | Webhook secret do AbacatePay |
| RABBIT_DELIVERY_QUEUE | Nome da fila de entregas |
| RABBIT_ORDER_QUEUE | Nome da fila de pedidos |

### Payment Service (fuudelivery-payment)

| Variavel | Descricao |
|---|---|
| MONGODB_URI | MongoDB Atlas (fuudelivery_payments) |
| JWT_SECRET | Mesmo secret da API |
| ADMIN_PASSWORD | Senha do admin |
| ABACATE_PAY_API_KEY | API key do AbacatePay |
| ABACATE_PAY_WEBHOOK_SECRET | Webhook secret do AbacatePay |
| BOOTSTRAP_SECRET | Secret para bootstrap do admin |
| PORT | 8084 (default) |
| REDIS_URL | Redis para fila de pagamentos |

### Frontends

| Variavel | Descricao |
|---|---|
| REACT_APP_API_URL | URL base da API Go |
| REACT_APP_PAYMENT_API_URL | URL base do Payment Service |

## Testes

```bash
# Todos os modulos Go (via go.work)
go test ./...

# Modulos individuais
cd Backend/Payment && go test ./...
cd Backend/payment_api && go test ./...
cd Backend/orders_api && go test ./...
cd Backend/auth_api && go test ./...

# Testes de integracao (requer Docker — sobe MongoDB em container)
cd Backend/Payment
go test -tags=integration ./services/... -v

# Frontend
cd Frontend/WebRestaurant && npm test
```

## Deploy

Push para `master` triggers deploy automatico via GitHub Actions + Render.

```bash
git push origin master
```

## Documentacao

- `.fuudelivery-config/DOCUMENTATION.md` — Documentacao completa do sistema
- `references/seguranca.md` — Procedimento de rotacao de credenciais
- `references/testes-ci.md` — Plano de cobertura de testes
- `references/confiabilidade-deploy.md` — Checklist de deploy e fila
- `references/gaps-funcionais.md` — TODOs e decisoes de arquitetura

## Licenca

MIT
