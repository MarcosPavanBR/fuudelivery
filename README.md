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
│  │  Queue (Redis Streams + consumer groups + DLQ + fallback)    │  │
│  └──────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
                    ▼                   ▼
┌───────────────────────┐  ┌───────────────────────────────────────────┐
│   PAGAMENTOS          │  │           INFRAESTRUTURA                   │
│   (payment_api —      │  │                                           │
│   embutido no         │  │  PostgreSQL (Supabase) — auth, pedidos    │
│   monolito)           │  │  MongoDB (Atlas) — pagamentos, chat,      │
│                       │  │               entregas, pedidos          │
│   • PIX/Cartão        │  │  Redis (externo) — fila + cache           │
│   • Carteiras         │  │  AbacatePay — gateway PIX/Cartão          │
│   • Chargebacks       │  │  OSRM — rotas e cálculo de entrega        │
│   • Aprovações        │  │  Supabase Storage — upload de imagens     │
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
| **Redis** | Provedor externo (`*.db.redis.io`) | Fila de mensagens (Streams + consumer groups) + cache |
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
| **Render** | Hosting (API monolito + 2 sites estáticos) |
| **GitHub Actions** | CI/CD (build, test, lint, deploy) |
| **Docker** | Containerização (desenvolvimento local) |
| **EAS (Expo)** | Build de APKs Android |
| **govulncheck** | Scan de vulnerabilidades Go |
| **npm audit** | Scan de vulnerabilidades npm |

## Serviços (Render)

| Serviço | Tipo | URL |
|---|---|---|
| fuudelivery-api | Go web service (monolito) | https://fuudelivery-api-8y6l.onrender.com |
| fuudelivery-web | Static site | https://fuudelivery-web.onrender.com |
| fuudelivery-admin | Static site | https://fuudelivery-admin-lv7f.onrender.com |

> Os serviços isolados `fuudelivery-payment` e `fuudelivery-payment-panel` foram
> removidos (2026-08): todas as rotas de pagamento vivem no monolito.

> **📌 Referência de URLs:** todos os links de produção (serviços, health checks,
> CORS, apps mobile) estão organizados em [`references/URLS.md`](references/URLS.md).
> O Redis NÃO é serviço Render — `REDIS_URL` aponta para provedor externo
> (`*.db.redis.io`). Não há bloco `type: redis` no render.yaml.

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
- Redis: fila via Streams (XAdd/XReadGroup) entre monolito e Payment Service, com retry e DLQ
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
- **PaymentPanel** (HTML/JS): painel standalone de aprovação de pagamentos — conecta ao MONOLITO (o serviço isolado de pagamentos foi removido em 2026-08; não é deployado pelo render.yaml)

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
│   │   ├── consumers/        # Consumer Redis (Streams/XReadGroup)
│   │   ├── handlers/         # Handlers HTTP (pagamentos, carteiras, chargebacks, relatórios)
│   │   ├── middleware/       # JWT auth + rate limiting
│   │   ├── models/          # Modelos de dados (Payment, Wallet, Chargeback, etc.)
│   │   ├── queue/           # Fila Redis (Streams + retry + DLQ)
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
│           ├── queue/       # Fila Redis Streams + fallback + DLQ
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

### Frontend (WebRestaurant)

```bash
cd Frontend/WebRestaurant
npm install
npm start
```

### Frontend (PaymentPanel) — arquivado

O painel standalone foi **arquivado em `legacy/PaymentPanel/`** (2026-08).
A funcionalidade equivalente está na aba **Financeiro** do WebAdmin.

### Apps Mobile

```bash
cd Frontend/AppComida
npm install
npx expo start

cd Frontend/AppEntrega
npm install
npx expo start
```

## Building APKs

Os APKs são gerados via **EAS Build** (Expo Application Services), que compila na nuvem.

### Pré-requisitos

```bash
# Instalar EAS CLI globalmente
npm install -g eas-cli

# Login na sua conta Expo
eas login

# Verificar que está logado
eas whoami
```

### AppComida (App do Cliente)

```bash
cd Frontend/AppComida
npx eas build --platform android --profile preview
```

O `preview` profile gera um **APK** (não AAB), pronto para instalar diretamente no Android.

### AppEntrega (App do Entregador)

```bash
cd Frontend/AppEntrega
npx eas build --platform android --profile preview
```

### Monitorar Builds

```bash
# Listar builds recentes
eas build:list --platform android --limit 5
```

Ou acesse: https://expo.dev/accounts/pavanbr/projects/my-app/builds

### Download do APK

Após o build completar, o link de download aparece no terminal e no dashboard do Expo. O APK fica disponível por 30 dias.

### Erros Comuns e Soluções

#### ❌ `Plugin [id: 'expo-module-gradle-plugin'] was not found`

**Causa:** O `expo prebuild` não gera o `settings.gradle` com autolinking correto porque:
- Versões do `expo`, `expo-secure-store` e `expo-modules-core` estão desalinhadas no `package.json`
- Um config plugin (como `withGradleWorkaround`) interfere no autolinking
- Um patch do `patch-package` modifica arquivos Gradle que conflitam com o plugin

**Solução:**
```bash
# 1. Alinhar versões com Expo
npx expo install --check

# 2. Regenerar a pasta android/ do zero
npx expo prebuild --clean

# 3. Remover config plugins desnecessários do app.json
#    Mantenha apenas: ["expo-router"]
#    Remova: ["./plugins/withGradleWorkaround"]

# 4. Remover patches que modificam Gradle
rm -f patches/expo-modules-core+*.patch

# 5. Remover patch-package se não houver mais patches
#    Remova "postinstall": "patch-package" do package.json

# 6. Build novamente
eas build --platform android --profile preview
```

> **Regra de ouro:** Se AppComida builda e AppEntrega não, verifique se o `app.json` de ambos
> tem os mesmos plugins e nenhuma patch que modifique Gradle. O problema quase sempre é um
> config plugin ou patch interferindo no autolinking do `expo-module-gradle-plugin`.

#### ❌ `npm install` ou `yarn install` trava / timeout

**Causa:** Cache corrompido, proxy corporativo, ou Windows Defender escaneando `node_modules`.

**Solução:**
```bash
# Limpar cache
rm -rf node_modules package-lock.json
npm cache clean --force

# Instalar com flags de compatibilidade
npm install --legacy-peer-deps

# Ou usar yarn
yarn install
```

#### ❌ `expo-router` plugin not found during EAS build

**Causa:** `node_modules` incompleto — o EAS CLI precisa resolver plugins localmente antes de enviar para a nuvem.

**Solução:**
```bash
rm -rf node_modules
npm install --legacy-peer-deps
eas build --platform android --profile preview
```

#### ❌ Build funciona local mas falha no EAS

**Causa:** O EAS Cloud pode ter versões diferentes de Node.js ou Expo CLI.

**Solução:** Verifique o `eas.json` e adicione:
```json
{
  "cli": {
    "version": ">= 7.3.0"
  },
  "build": {
    "preview": {
      "android": {
        "buildType": "apk"
      }
    }
  }
}
```

### Gerar APKs Localmente (alternativa)

```bash
# Requer Android SDK instalado localmente
cd Frontend/AppComida
npx expo prebuild --clean
cd android && ./gradlew assembleRelease

# APK gerado em:
# android/app/build/outputs/apk/release/app-release.apk
```

> **Nota:** Certifique-se de que `ANDROID_HOME` está configurado apontando para o Android SDK.
> Exemplo: `export ANDROID_HOME=$HOME/Library/Android/sdk` (macOS/Linux) ou
> `set ANDROID_HOME=%LOCALAPPDATA%\Android\Sdk` (Windows).

Ou use o script automatizado:
```bash
bash scripts/build-apks.sh
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
| `METRICS_TOKEN` | Token para proteger `GET /metrics` (header Bearer ou ?token=) | Recomendado em produção |
| `OSRM_BASE_URL` | Base do servidor OSRM (rotas). Default: demo público — configure instância própria em produção | Não |
| `URL_CHECK_ESTABLISHMENT_OPEN` | Template da checagem de estabelecimento aberto (`%d` = ID). Default: localhost:PORT | Não |
| `ALLOWED_ORIGINS` | Domínios permitidos (CORS) | Não (usa defaults) |
| `PORT` | Porta do servidor | Não (default: 3000) |
| `GO_ENV` | Ambiente (production/development) | Não |

> **⚠️ O Payment Service isolado (`fuudelivery-payment`) foi removido (2026-08).**
> Todas as rotas de pagamento vivem no monolito — as variáveis abaixo são do
> monolito `fuudelivery-api` (ver tabela anterior).

### Frontends

| Variável | Descrição |
|---|---|
| `REACT_APP_API_URL` | URL base da API Go (monolito) |

## Testes

```bash
# Todos os módulos Go (via go.work)
go test ./...

# Módulos individuais
cd Backend/payment_api && go test ./...
cd Backend/orders_api && go test ./...
cd Backend/auth_api && go test ./...
cd Backend/delivery_api && go test ./...
cd Backend/chat_api && go test ./...
cd cmd/fuudelivery && go test ./...

# Testes de integração (requer Docker — sobe MongoDB em container)
cd cmd/fuudelivery && go test -tags=integration -v -run 'TestFullFlow|TestErrorScenarios|TestAdminBootstrap' ./
cd Backend/payment_api && go test -tags=integration -v -run 'TestCheckoutE2E' ./app/handlers/

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
