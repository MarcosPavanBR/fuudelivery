# FuuDelivery

Plataforma de delivery completa com pagamento integrado, split, carteira digital, cashback, cupons, chat, rastreio e painel de pagamentos.

Fork do [vercardapio/appdelivery](https://github.com/carloshomar/appdelivery) extendido com sistema de pagamentos e infra de producao no Render.

## Arquitetura

```
  AppComida (React Native)    AppEntrega (React Native)    WebRestaurant (React)
  App de cliente               App de entregador            Kanban + Cardapio + Carteira + Relatorios
          |                            |                              |
          +----------------------------+------------------------------+
          |
  API Gateway (Go + Fiber)  ---->  Payment Service (Go + Fiber)
  PostgreSQL (Supabase)             MongoDB (Atlas)
  Redis (Render)                    RabbitMQ
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

### Comunicacao
- RabbitMQ: fila entre monolito e Payment Service
- WebSocket: atualizacoes em tempo real
- Redis: fila/pubsub com fallback para canais Go em memoria

### Frontend
- AppComida (React Native/Expo): app do cliente
- AppEntrega (React Native/Expo): app do entregador
- WebRestaurant (React + Tailwind): kanban, cardapio, carteira, relatorios, cadastro
- WebAdmin (React): painel administrativo
- PaymentPanel (React): aprovacao de pagamentos

### Seguranca
- JWT com validacao de SigningMethod
- Rate limiting: login 5req/min, pagamento 10req/min, carteira 20req/min
- CORS restrito a dominios conhecidos
- Senhas hasheadas com bcrypt
- CI com govulncheck e npm audit

## Como Rodar

### Backend (Docker)

```bash
cd Backend
docker compose up --build
```

### Backend (servico individual)

```bash
cd Backend/Payment
go mod tidy
go run main.go
```

### Frontend

```bash
cd Frontend/WebRestaurant
npm install
npm start
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
| RABBITMQ_URL | RabbitMQ (opcional) |

### Payment Service (fuudelivery-payment)

| Variavel | Descricao |
|---|---|
| MONGODB_URI | MongoDB Atlas (fuudelivery_payments) |
| JWT_SECRET | Mesmo secret da API |
| ADMIN_PASSWORD | Senha do admin |
| ABACATEPAY_API_KEY | API key do AbacatePay |
| ABACATEPAY_WEBHOOK_SECRET | Webhook secret do AbacatePay |
| BOOTSTRAP_SECRET | Secret para bootstrap do admin |
| PORT | 8084 (default) |

## Testes

```bash
# Todos os testes do Payment Service (44 testes)
cd Backend/Payment
go test ./... -v

# Modulos individuais
cd Backend/payment_api && go test ./...
cd Backend/orders_api && go test ./...
cd Backend/auth_api && go test ./...
```

## Deploy

Push para `master` triggers deploy automatico via GitHub Actions + Render.

```bash
git push origin master
```

## Documentacao

- `references/seguranca.md` — Procedimento de rotacao de credenciais
- `references/testes-ci.md` — Plano de cobertura de testes
- `references/confiabilidade-deploy.md` — Checklist de deploy e fila
- `references/gaps-funcionais.md` — TODOs e decisoes de arquitetura

## Licenca

MIT
