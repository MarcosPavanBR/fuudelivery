# FuuDelivery

Plataforma de delivery completa com pagamento integrado, split, carteira digital, cashback, cupons, chat, rastreio e painel de pagamentos.

Fork do [vercardapio/appdelivery](https://github.com/carloshomar/appdelivery) estendido com sistema de pagamentos e infra de producao no Render.

## Arquitetura

```
┌─────────────────────────────────────────────────────────────────────┐
│                        FRONTENDS                                    │
├─────────────────┬─────────────────┬─────────────────────────────────┤
│ AppComida       │ AppEntrega      │ WebRestaurant                   │
│ React Native    │ React Native    │ React + Tailwind                │
│ (Expo)          │ (Expo)          │ (Webpack)                       │
│ App do Cliente  │ App Entregador  │ Kanban + Cardápio + Carteira    │
├─────────────────┴─────────────────┴─────────────────────────────────┤
│ WebAdmin                │ PaymentPanel                              │
│ React                   │ HTML/JS standalone                        │
│ Painel Administrativo   │ Painel de Aprovação de Pagamentos         │
└─────────────────────────┴───────────────────────────────────────────┘
                              │
                              │ HTTP / WebSocket
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    MONOLITH (cmd/fuudelivery)                       │
│                    Go 1.23 + Fiber v2                               │
│                                                                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │  Auth    │ │ Orders   │ │ Delivery │ │ Payment  │ │  Chat    │ │
│  │  API     │ │ API      │ │ API      │ │ API      │ │  API     │ │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Dispatch Engine (matching + calibração + split decay)       │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Queue (Redis LPush/BRPop + Go channels fallback)            │  │
│  └──────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
                    ▼                   ▼
┌───────────────────────┐  ┌───────────────────────────────────────────┐
│   PAYMENT SERVICE     │  │           INFRAESTRUTURA                   │
│   (Backend/Payment)   │  │                                           │
│   Go 1.23 + Fiber     │  │  PostgreSQL (Supabase) — auth, pedidos    │
│                       │  │  MongoDB (Atlas) — pagamentos, chat,      │
│   • Aprovação         │  │               entregas, pedidos          │
│   • Score de risco    │  │  Redis (Render) — fila + cache            │
│   • Carteiras         │  │  AbacatePay — gateway PIX/Cartão          │
│   • Chargebacks       │  │  OSRM — rotas e cálculo de entrega        │
│   • Relatórios        │  │  Supabase Storage — upload de imagens     │
└───────────────────────┘  └───────────────────────────────────────────┘
```

## Stack Tecnológica

### Backend

| Tecnologia | Versão | Uso |
|---|---|---|
| **Go** | 1.23.0 | Linguagem principal do backend |
| **Fiber** | v2 | Framework HTTP (inspirado em Express) |
| **MongoDB** | Atlas | Banco para pagamentos, chat, entregas, pedidos |
| **PostgreSQL** | Supabase | Banco para auth, usuários, estabelecimentos, zonas |
| **Redis** | Render | Fila de mensagens (LPush/BRPop) + cache |
| **GORM** | v2 | ORM para PostgreSQL |
| **go-redis** | v8 | Cliente Redis |
| **golang-jwt** | v5 | Autenticação JWT |
| **AbacatePay** | API | Gateway de pagamento (PIX + Cartão) |
| **OSRM** | API | Cálculo de rotas e distâncias |
| **bcrypt** | - | Hash de senhas |
| **testcontainers** | - | Testes de integração com MongoDB real |

### Frontend

| Tecnologia | Versão | Uso |
|---|---|---|
| **React** | 18.2 | WebRestaurant + WebAdmin + PaymentPanel |
| **React Native** | Expo | AppComida + AppEntrega |
| **Tailwind CSS** | 3.4 | Estilização dos web apps |
| **Webpack** | 5.91 | Bundler dos web apps |
| **Axios** | 1.6 | Cliente HTTP |
| **react-router-dom** | 6.22 | Roteamento SPA |
| **@hello-pangea/dnd** | 18.0 | Drag-and-drop (Kanban) |
| **socket.io-client** | 4.7 | WebSocket (chat) |
| **react-toastify** | 10.0 | Notificações toast |
| **jwt-decode** | 4.0 | Decodificação de tokens JWT |

### Infra & CI/CD

| Tecnologia | Uso |
|---|---|
| **Render** | Hosting (web services + Redis) |
| **GitHub Actions** | CI/CD (build, test, lint, deploy) |
| **Docker** | Containerização (desenvolvimento local) |
| **EAS (Expo)** | Build de APKs Android |
| **govulncheck** | Scan de vulnerabilidades Go |
| **npm audit** | Scan de vulnerabilidades npm |

## Serviços (Render)

| Serviço | Tipo | URL |
|---|---|---|
| fuudelivery-api | Go web service | https://fuudelivery-api-8y6l.onrender.com |
| fuudelivery-payment | Go web service | https://fuudelivery-payment.onrender.com |
| fuudelivery-web | Static site | https://fuudelivery-web.onrender.com |
| fuudelivery-admin | Static site | https://fuudelivery-admin-lv7f.onrender.com |
| fuudelivery-payment-panel | Static site | https://fuudelivery-payment-panel.onrender.com |
| fuudelivery-redis | Redis | Gerenciado pelo Render |

## Features

### Pagamento
- PIX e Cartão via AbacatePay
- Split de pagamento: 5% plataforma, 85% restaurante, taxa de entrega
- Score de risco: 4 fatores (valor, frequência, histórico, horário)
- Aprovação automática (baixo risco) ou manual (alto risco)
- Carteira digital com operações atômicas ($inc no MongoDB)
- Cashback e cupons de desconto
- Saque via PIX

### Comunicação
- Redis: fila LPush/BRPop entre monolito e Payment Service
- WebSocket: atualizações em tempo real (pedidos, entregas, chat)
- Go channels: fallback in-memory quando Redis não configurado

### Dispatch Engine
- Matching engine com score de proximidade e densidade de entregadores
- Calibração automática de raios de zona (24h)
- Decay de split baseado em métricas de zona
- Dead-letter queue para pedidos não matchados
- Batch de pedidos (batching)

### Frontend
- **AppComida** (React Native/Expo): app do cliente
- **AppEntrega** (React Native/Expo): app do entregador
- **WebRestaurant** (React + Tailwind): kanban, cardápio, carteira, relatórios, cadastro
- **WebAdmin** (React): painel administrativo
- **PaymentPanel** (HTML/JS): painel standalone de aprovação de pagamentos

### Segurança
- JWT com validação de SigningMethod
- Rate limiting: login 5req/min, pagamento 10req/min, carteira 20req/min
- CORS restrito a domínios conhecidos
- Senhas hasheadas com bcrypt
- Wallet com operações atômicas (eliminação de race conditions)
- CI com govulncheck e npm audit

## Estrutura do Projeto

```
fuudelivery/
├── Backend/
│   ├── Payment/              # Microserviço de pagamentos (aprovação, carteiras, chargebacks)
│   │   ├── config/           # Configuração e variáveis de ambiente
│   │   ├── consumers/        # Consumer Redis (BRPop)
│   │   ├── handlers/         # Handlers HTTP (pagamentos, carteiras, chargebacks, relatórios)
│   │   ├── middleware/       # JWT auth + rate limiting
│   │   ├── models/          # Modelos de dados (Payment, Wallet, Chargeback, etc.)
│   │   ├── queue/           # Fila Redis (LPush/BRPop)
│   │   ├── repository/      # Acesso a dados (MongoDB)
│   │   └── services/        # Lógica de negócio (aprovação, risco, carteiras)
│   ├── auth_api/            # Autenticação JWT, CRUD de usuários e estabelecimentos
│   ├── payment_api/         # Gateway de pagamento (AbacatePay/PIX/Cartão)
│   ├── orders_api/          # Pedidos, produtos, categorias, cupons, fidelidade
│   ├── delivery_api/        # Rastreio de entregadores, matching engine
│   ├── chat_api/            # Chat em tempo real
│   └── storage/             # Upload de imagens (Supabase Storage)
├── cmd/
│   └── fuudelivery/         # Monolith principal (API Gateway)
│       └── pkg/
│           ├── health/      # Health checks (Postgres, MongoDB, Redis)
│           ├── queue/       # Fila Redis + Go channels fallback
│           ├── storage/     # Conexão MongoDB
│           └── upload/      # Upload de imagens
├── Frontend/
│   ├── AppComida/           # React Native/Expo — app do cliente
│   ├── AppEntrega/          # React Native/Expo — app do entregador
│   ├── WebRestaurant/       # React + Tailwind — kanban, cardápio, carteira
│   ├── WebAdmin/            # React — painel administrativo
│   └── PaymentPanel/        # HTML/JS — painel de pagamentos
├── scripts/                 # Scripts auxiliares (keepalive, migração, seed)
├── references/              # Documentação (segurança, gaps, testes, deploy)
├── .github/workflows/       # CI/CD (ci.yml, deploy.yml, build-apps)
├── Arquitetura/             # Diagramas (draw.io)
├── brand/                   # Identidade visual e materiais comerciais
├── render.yaml              # Configuração de deploy no Render
├── docker-compose.yml       # Docker (desenvolvimento local)
├── go.work                  # Go workspace (7 módulos)
├── MANIFEST.md              # Manifesto do projeto
└── README.md                # Este arquivo
```

## Como Rodar

### Pré-requisitos
- Go 1.23+
- Node.js 20+
- Docker (para MongoDB local)
- variáveis de ambiente configuradas (ver abaixo)

### Backend (Docker — todos os serviços)

```bash
cd Backend
docker compose up --build
```

### Backend (serviço individual — Monolith)

```bash
cd cmd/fuudelivery
go mod tidy
go run main.go
```

### Backend (serviço individual — Payment Service)

```bash
cd Backend/Payment
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

cd Frontend/AppEntrega
npm install
npx expo start
```

## Variáveis de Ambiente

### Monolith (fuudelivery-api)

| Variável | Descrição | Obrigatória |
|---|---|---|
| `JWT_SECRET` | Secret para tokens JWT | Sim |
| `DB_CONNECTION_STRING` | PostgreSQL (Supabase) | Sim |
| `MONGO_URI` | MongoDB Atlas | Sim |
| `MONGO_DATABASE` | Nome do banco MongoDB | Sim |
| `REDIS_URL` | Redis (Render) | Não (fallback Go channels) |
| `ABACATE_PAY_API_KEY` | API key do AbacatePay | Sim (pagamentos) |
| `ABACATE_PAY_WEBHOOK_SECRET` | Webhook secret do AbacatePay | Sim (pagamentos) |
| `ALLOWED_ORIGINS` | Domínios permitidos (CORS) | Não (usa defaults) |
| `PORT` | Porta do servidor | Não (default: 3000) |
| `GO_ENV` | Ambiente (production/development) | Não |

### Payment Service (fuudelivery-payment)

| Variável | Descrição | Obrigatória |
|---|---|---|
| `MONGO_URI` | MongoDB Atlas (fuudelivery_payments) | Sim |
| `JWT_SECRET` | Mesmo secret da API | Sim |
| `ADMIN_PASSWORD` | Senha do admin | Sim |
| `ABACATE_PAY_API_KEY` | API key do AbacatePay | Sim |
| `ABACATE_PAY_WEBHOOK_SECRET` | Webhook secret do AbacatePay | Sim |
| `REDIS_URL` | Redis para fila de pagamentos | Recomendado (necessario para credito em producao) |
| `PORT` | Porta do servidor | Não (default: 8084) |

### Frontends

| Variável | Descrição |
|---|---|
| `REACT_APP_API_URL` | URL base da API Go |
| `REACT_APP_PAYMENT_API_URL` | URL base do Payment Service |

## Testes

```bash
# Todos os módulos Go (via go.work)
go test ./...

# Módulos individuais
cd Backend/Payment && go test ./...
cd Backend/payment_api && go test ./...
cd Backend/orders_api && go test ./...
cd Backend/auth_api && go test ./...
cd Backend/delivery_api && go test ./...
cd Backend/chat_api && go test ./...
cd cmd/fuudelivery && go test ./...

# Testes de integração (requer Docker — sobe MongoDB em container)
cd Backend/Payment
go test -tags=integration ./services/... -v

# Frontend
cd Frontend/WebRestaurant && npm test
```

## CI/CD

Push para `master` triggers deploy automático via GitHub Actions + Render.

### Pipelines

1. **CI Gate** (`ci.yml`): Build, vet, test, gofmt, govulncheck para todos os módulos Go; npm test + build para WebRestaurant; npm audit para todos os frontends
2. **Deploy** (`deploy.yml`): Deploy automático para Render (só se CI passar)
3. **Build AppComida** (`build-appcomida.yml`): Build APK Android via EAS
4. **Build AppEntrega** (`build-appentrega.yml`): Build APK Android via EAS

```bash
git push origin master
```

## Documentação

- `references/seguranca.md` — Procedimento de rotação de credenciais
- `references/testes-ci.md` — Plano de cobertura de testes
- `references/confiabilidade-deploy.md` — Checklist de deploy e fila
- `references/gaps-funcionais.md` — TODOs e decisões de arquitetura
- `references/avaliacao-modelo-negocio` — Avaliação do modelo de negócio
- `references/brand.md` — Identidade visual
- `.fuudelivery-config/DOCUMENTATION.md` — Documentação completa do sistema

## Licença

MIT
