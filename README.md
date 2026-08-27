# FuuDelivery

![CI Gate](https://github.com/MarcosPavanBR/fuudelivery/actions/workflows/ci.yml/badge.svg)

Plataforma de delivery completa: pedidos, pagamento integrado (PIX/Cartão) com split, carteira digital, cashback, cupons, chat em tempo real, rastreio de entrega com dispatch engine e painéis web + apps mobile.

Fork do [vercardapio/appdelivery](https://github.com/carloshomar/appdelivery) estendido com sistema de pagamentos próprio, estratégia de banco único PostgreSQL e infraestrutura de produção (Render + Docker/VPS).

## Funcionalidades

### Pedidos e catálogo
- Cardápio com categorias, produtos e adicionais (relacionamento N:N)
- Pedidos agendados, repetição de pedido, cupons e programa de fidelidade
- Reviews e batches (pedidos em lote para o restaurante)
- Busca full-text (`/search`) sobre produtos e estabelecimentos
- Horário de funcionamento, destaques e checagem de estabelecimento aberto

### Pagamentos (Multi-Gateway)
- **4 gateways**: Pagar.me (principal), Asaas (alternativo), AbacatePay (fallback PIX), Mercado Pago (reserva)
- **Métodos**: PIX, Cartão de Crédito (com 3DS), Cartão de Débito (com 3DS)
- **Split automático**: divisão instantânea entre plataforma, restaurante e entregador via sub-contas (recipients)
- **Pré-autorização**: cartão de crédito autoriza na criação, captura na confirmação de entrega (PIN)
- **3D Secure (3DS)**: autenticação obrigatória para débito e crédito > R$ 200
- **Circuit breaker**: fallback automático se um gateway falhar 5 vezes em 1 minuto
- **PIN de verificação**: motoboy confirma entrega com PIN de 4 dígitos (TTL 30min, máx 3 tentativas)
- **Escrow (D+X)**: repasse D+1 entregador, D+7 restaurante (configurável por gateway)
- **Idempotência**: constraints UNIQUE + idempotency_key (UUID v4) em duas camadas
- **Score de risco**: 4 fatores (valor, frequência, histórico, horário) → aprovação automática ou manual
- **Carteira digital**: operações atômicas, ledger imutável, saque via PIX
- Detalhes: [`references/arquitetura-split-pagamentos.md`](references/arquitetura-split-pagamentos.md)

### Dispatch Engine (entregas)
- Matching engine com score de proximidade e densidade de entregadores
- Calibração automática de raios de zona (job 24h)
- Decay de split baseado em métricas de zona
- Dead-letter queue (DLQ) para pedidos não matchados + batch de pedidos

### Comunicação
- WebSocket nativo (não socket.io): atualizações de pedidos/entregas em tempo real e chat por pedido
- Fila Redis Streams (`XADD`/`XREADGROUP`, consumer groups, retry, DLQ) com fallback in-memory via Go channels quando `REDIS_URL` não está configurado

### Segurança e contas
- JWT HS256 com validação de algoritmo, rate limiting por IP, CORS com trusted proxies
- **Reset de senha assistido** (`POST /admin/password-reset/code` + `POST /auth/reset-password`): o suporte gera um código de 8 caracteres no WebAdmin e informa por telefone/WhatsApp; o usuário define a nova senha na página pública `/resetar-senha` (WebRestaurant). Código com TTL de 15 min, uso único e teto de 5 tentativas.

## Arquitetura

```
┌─────────────────────────────────────────────────────────────────────┐
│                        FRONTENDS                                    │
├─────────────────┬─────────────────┬─────────────────────────────────┤
│ AppComida       │ AppEntrega      │ AppRestaurante                  │
│ React Native    │ React Native    │ React Native                    │
│ (Expo SDK 54)   │ (Expo SDK 54)   │ (Expo SDK 54)                   │
│ App do Cliente  │ App Entregador  │ App do Restaurante              │
├─────────────────┴─────────────────┼─────────────────────────────────┤
│ WebRestaurant                     │ WebAdmin                        │
│ React 19 + Vite 6 + Tailwind 4    │ React 19 + Vite 6 + Tailwind 4  │
│ Kanban + Cardápio + PWA           │ Dashboard + Financeiro          │
└───────────────────────────────────┴─────────────────────────────────┘
                              │
                              │ HTTP / WebSocket nativo
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    MONOLITH (cmd/fuudelivery)                       │
│                    Go 1.25 + Fiber v2                               │
│                                                                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │
│  │ Auth API │ │ Orders   │ │ Delivery │ │ Payment  │ │  Chat    │ │
│  │          │ │ API      │ │ API      │ │ API      │ │  API     │ │
│  └──────────┘ └──────────┘ └──────────┘ └─────┬────┘ └──────────┘ │
│                                                │                    │
│  ┌─────────────────────────────────────────────▼────────────────┐  │
│  │  Payment Router (gateway selection + circuit breaker + retry)│  │
│  │  Pagar.me (principal) · Asaas (alternativo)                 │  │
│  │  AbacatePay (fallback PIX) · Mercado Pago (reserva)         │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Dispatch Engine (matching + calibração + split decay)       │  │
│  ├──────────────────────────────────────────────────────────────┤  │
│  │  Queue (Redis Streams + consumer groups + DLQ + fallback)    │  │
│  ├──────────────────────────────────────────────────────────────┤  │
│  │  pkg/health (health checks) · /metrics (Prometheus)          │  │
│  └──────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                              │
                     ┌────────┴─────────┐
                     ▼                  ▼
┌───────────────────────────┐  ┌─────────────────────────────────────┐
│      INTEGRAÇÕES          │  │         INFRAESTRUTURA              │
│                           │  │                                     │
│  • AbacatePay — PIX/      │  │  PostgreSQL único (Supabase) —      │
│    Cartão + webhook       │  │  TODOS os domínios (auth, pedidos,  │
│  • OSRM — rotas/distância │  │  entregas, pagamentos, chat)        │
│  • Supabase Storage —     │  │  MongoDB Atlas (opcional — apenas   │
│    upload de imagens      │  │  dual-write legado até aposentar)   │
│  • Asaas — wallet         │  │  Redis externo — fila + cache       │
│                           │  │  + rate limit                       │
└───────────────────────────┘  └─────────────────────────────────────┘
```

**Banco único:** o PostgreSQL (Supabase) é o banco primário de todos os domínios. O MongoDB Atlas sobrevive apenas como *dual-write* legado opcional — basta não definir `MONGO_URI` para desligá-lo. A migração é feita pelos ETLs idempotentes `cmd/etl-orders` e `cmd/etl-payments`. Detalhes em [`docs/ARQUITETURA-BANCO-UNICO.md`](docs/ARQUITETURA-BANCO-UNICO.md).

> Os serviços isolados `fuudelivery-payment` e `fuudelivery-payment-panel` foram **removidos do Render (2026-08)**: todas as rotas de pagamento vivem no monolito. O painel standalone foi arquivado em `legacy/PaymentPanel/` e substituído pela aba **Financeiro** do WebAdmin.

## Stack Tecnológica

### Backend

| Tecnologia | Versão | Uso |
|---|---|---|
| **Go** | 1.25.0 | Linguagem principal (todos os go.mod do workspace) |
| **Fiber** | v2 | Framework HTTP (+ contrib/websocket v1.3.4 para WS nativo) |
| **PostgreSQL** | Supabase (pooler PgBouncer :6543) | Banco único primário |
| **GORM** | v2 (v1.31.2 + driver/postgres v1.6.2) | ORM PostgreSQL com AutoMigrate no startup |
| **go-redis** | v8 (v8.11.5) | Fila Redis Streams + cache + rate limit |
| **golang-jwt** | v5 (v5.3.1, HS256) | Autenticação JWT + refresh tokens |
| **mongo-driver** | v1.17.9 | Dual-write legado (opcional) |
| **Pagar.me** | API REST v4 | Gateway principal (PIX + Cartão + Débito + Split + 3DS) |
| **Asaas** | API REST | Gateway alternativo (PIX + Cartão + Débito + Split) |
| **AbacatePay** | API REST | Gateway fallback PIX |
| **Mercado Pago** | API REST | Gateway reserva (PIX + Cartão + Split 1:1) |
| **OSRM** | API | Cálculo de rotas e distâncias |
| **bcrypt** (golang.org/x/crypto) | - | Hash de senhas |
| **testify + testcontainers-go** | v0.44 | Testes de integração com Postgres/Mongo reais |
| **miniredis** | v2.38 | Redis em memória para testes da fila |

### Frontend

| Tecnologia | Versão | Uso |
|---|---|---|
| **React Native** | 0.81.5 + Expo SDK ~54 | AppComida, AppEntrega, AppRestaurante |
| **expo-router** | ~6.0 | Navegação dos apps mobile |
| **NativeWind** | 4.1 (+ Tailwind 3.4) | Estilização dos apps mobile |
| **@maplibre/maplibre-react-native** | 11.3 | Mapas nos apps mobile |
| **React** | ^19.1 | WebRestaurant + WebAdmin |
| **Vite** | ^6.0 | Bundler/dev server dos web apps |
| **Tailwind CSS** | ^4.1 (@tailwindcss/vite) | Estilização dos web apps |
| **react-router-dom** | ^6.30 | Roteamento SPA |
| **@hello-pangea/dnd** | ^18 | Drag-and-drop (Kanban) |
| **react-use-websocket** | ^4.8 | WebSocket nativo (chat/tempo real) |
| **axios** | ^1.6 | Cliente HTTP |
| **Vitest** | ^4 (WebRestaurant) / ^3 (WebAdmin) | Testes unitários + Testing Library |

### Infra & CI/CD

| Tecnologia | Uso |
|---|---|
| **Render** | Hosting (API Go + 2 sites estáticos) |
| **Docker** | Multi-stage build + compose para VPS |
| **GitHub Actions** | 7 workflows (CI, deploy, monitoramento, releases) |
| **EAS (Expo)** | Build de APKs Android na nuvem |
| **govulncheck + npm audit** | Scan de vulnerabilidades no CI |
| **Dependabot** | Atualização semanal agrupada (gomod + npm) |

## Serviços em produção (Render)

| Serviço | Tipo | URL |
|---|---|---|
| fuudelivery-api | Go web service (monolito) | https://fuudelivery-api-8y6l.onrender.com |
| fuudelivery-web | Static site (WebRestaurant) | https://fuudelivery-web.onrender.com |
| fuudelivery-admin | Static site (WebAdmin) | https://fuudelivery-admin-lv7f.onrender.com |

> **Referência canônica de URLs:** todos os links de produção (serviços, health checks,
> CORS, apps mobile compilados) estão em [`references/URLS.md`](references/URLS.md).
> Renomear serviços quebra clientes compilados — não renomeie.
>
> O Redis **não** é serviço Render — `REDIS_URL` aponta para provedor externo
> (`*.db.redis.io`). Não há bloco `type: redis` no render.yaml.

## Estrutura do Projeto

```
fuudelivery/
├── Backend/
│   ├── auth_api/             # Usuários, clientes, estabelecimentos, entregadores, zonas, assinaturas, refresh tokens
│   ├── orders_api/           # Pedidos, produtos, categorias, adicionais, cupons, fidelidade, reviews, pickup-code
│   ├── delivery_api/         # Solicitações de entrega, extrato; Dispatch Engine (matching, auto-calibração, split decay)
│   ├── payment_api/          # PIX/Cartão (AbacatePay), carteiras, chargebacks, split, webhook, Asaas wallet
│   ├── chat_api/             # Chat por pedido via WebSocket
│   └── storage/supabase.go   # Upload de imagens (Supabase Storage)
├── cmd/
│   ├── fuudelivery/          # Monolito principal (aglutina os 5 APIs) + pkg interno:
│   │                         #   health/ · queue/ · storage/ · upload/ · metrics/ · search/
│   ├── etl-orders/           # ETL one-shot Mongo → Postgres (orders → order_documents)
│   └── etl-payments/         # ETL one-shot Mongo → Postgres (payments/wallets/ledger)
├── pkg/
│   ├── gateway/              # Camada de abstração multi-gateway (interface Gateway + Router + CircuitBreaker)
│   │   ├── gateway.go        # Interface Gateway + tipos + enums (PaymentMethod, SplitRule, etc.)
│   │   ├── router.go         # Router com fallback chain e circuit breaker
│   │   ├── circuitbreaker.go # Circuit breaker (Closed → Open → HalfOpen)
│   │   ├── registry.go       # Registro e discovery de gateways
│   │   ├── pagarme/          # Adapter Pagar.me v4 (principal)
│   │   ├── asaas/            # Adapter Asaas (alternativo)
│   │   ├── abacatepay/       # Adapter AbacatePay (fallback PIX)
│   │   └── mercadopago/      # Adapter Mercado Pago (reserva)
│   ├── health/               # Health checks compartilháveis (Postgres, Redis)
│   └── queue/                # Fila Redis Streams + DLQ + fallback in-memory (compartilhável)
├── Frontend/
│   ├── AppComida/            # React Native/Expo — app do cliente (mapas MapLibre)
│   ├── AppEntrega/           # React Native/Expo — app do entregador (i18n)
│   ├── AppRestaurante/       # React Native/Expo — app do restaurante
│   ├── WebRestaurant/        # React 19 + Vite 6 + Tailwind 4 — kanban, cardápio, carteira, PWA
│   └── WebAdmin/             # React 19 + Vite 6 + Tailwind 4 — dashboard, pedidos, financeiro
├── legacy/PaymentPanel/      # Painel standalone arquivado (substituído pelo Financeiro do WebAdmin)
├── sql/                      # 13 migrações SQL versionadas + run_all.sh (ver seção Banco de Dados)
├── scripts/                  # Build APKs, deploy VPS, seeds, migrações, checks de CI (28 itens)
├── docs/                     # Arquitetura, banco de dados, deploy, segurança, FAQ, changelog
├── references/               # Docs internos (URLs, roadmap, gaps, testes, release notes)
├── .github/workflows/        # 7 workflows de CI/CD (ver seção CI/CD)
├── .pg-embed/                # Binários PostgreSQL 18.3 locais (dev/testes sem Docker)
├── Arquitetura/              # Diagramas (draw.io) e materiais visuais
├── brand/                    # Identidade visual e materiais comerciais
├── skills/fuudelivery-banco-unico/  # Regras obrigatórias p/ IA tocar no banco
├── Dockerfile                # Multi-stage (builder Go + runtime alpine, usuário não-root)
├── docker-compose.vps.yml    # Stack VPS: api + web-restaurant + web-admin + redis:7
├── render.yaml               # Blueprint Render (3 serviços)
├── Procfile                  # web: ./server
├── go.work                   # Go workspace com 10 módulos
└── MANIFEST.md · PRODUCTION.md · CONTRIBUTING.md · SECURITY.md · TRADEMARK.md
```

Os módulos Go individuais seguem a convenção `app/{handlers,models,routes,dto,services}` (ex.: `Backend/payment_api/app/handlers/`).

## Banco de Dados

- **PostgreSQL único (Supabase)** com GORM AutoMigrate no startup + 16 migrações SQL versionadas em `sql/` aplicadas via `sql/run_all.sh` (controle pela tabela `schema_migrations`):
  - `00_role_e_controle_migracoes` — role `app_backend` com least privilege + controle de migrações
  - `01–04` — domínios: pedidos, entrega, pagamentos (carteiras, ledger, chargebacks, payout), chat
  - `05_audit_log` + `06_rls_seguranca` — trilha de auditoria e Row Level Security
  - `07–11` — tabelas órfãs, reparos legado, ledger kinds, idempotência financeira, refresh tokens
  - `14–16` — **multi-gateway**: recipients (sub-contas), split_rules (divisão de valores), colunas gateway na tabela payments
- Dicionário completo de tabelas: [`docs/banco-de-dados.md`](docs/banco-de-dados.md)
- Regras para alterar o schema (obrigatórias): [`skills/fuudelivery-banco-unico/SKILL.md`](skills/fuudelivery-banco-unico/SKILL.md)
- Migração Mongo → Postgres: rode os binários `cmd/etl-orders` e `cmd/etl-payments` (idempotentes)

## Como Rodar

### Pré-requisitos
- Go 1.25+
- Node.js 20+
- Docker (para testes de integração) — ou `.pg-embed` para Postgres local sem Docker
- Variáveis de ambiente (ver [`.env.example`](.env.example))

### Backend (monolito)

```bash
cp .env.example .env   # preencha JWT_SECRET e DB_CONNECTION_STRING (obrigatórios)
cd cmd/fuudelivery
go mod tidy
go run main.go          # porta 3000 (configurável via PORT)
```

> Em `GO_ENV=production` o processo aborta o startup se `JWT_SECRET` ou `DB_CONNECTION_STRING` estiverem ausentes. Graceful shutdown de 10s.

Health check: `GET /health` (Postgres/Mongo/Redis) · Métricas: `GET /metrics` (protegido por `METRICS_TOKEN`).

### Frontends web

```bash
# WebRestaurant (http://localhost:3000)
cd Frontend/WebRestaurant
npm install --legacy-peer-deps
npm run dev

# WebAdmin
cd Frontend/WebAdmin
npm install --legacy-peer-deps
npm run dev
```

### Apps mobile (Expo)

```bash
cd Frontend/AppComida && npm install && npx expo start
cd Frontend/AppEntrega && npm install && npx expo start
```

A URL da API dos apps fica centralizada em `Frontend/<app>/config/api.ts`.

### Deploy local via Docker (estilo VPS)

O [`docker-compose.vps.yml`](docker-compose.vps.yml) sobe a stack completa presa em `127.0.0.1`
(api :3000, web :3002, admin :3003, Redis :6379 com AOF) — o nginx do host faz o TLS:

```bash
docker compose -f docker-compose.vps.yml up --build
```

Guia completo de VPS: [`scripts/deploy-vps.md`](scripts/deploy-vps.md).

## Variáveis de Ambiente

Referência completa: [`.env.example`](.env.example). Principais:

### Monolito (fuudelivery-api)

| Variável | Descrição | Obrigatória |
|---|---|---|
| `JWT_SECRET` | Secret dos tokens JWT (64 chars aleatórios recomendado) | **Sim** |
| `DB_CONNECTION_STRING` | PostgreSQL Supabase — pooler `:6543` (PgBouncer transaction mode) | **Sim** |
| `SUPABASE_URL` + `SUPABASE_SERVICE_ROLE_KEY` | Upload de imagens (sem isso o endpoint responde 503) | Sim (upload) |
| `ADMIN_BOOTSTRAP_SECRET` | Bootstrap único do primeiro admin via `POST /admin/bootstrap`. Remover após uso | Recomendado |
| `REDIS_URL` | Redis gerenciado (fila financeira + cache + rate limit; prefira política `noeviction`) | Não (fallback Go channels) |
| `MONGO_URI` + `MONGO_DATABASE` | MongoDB Atlas — dual-write legado. Omitir = desligado | Não |
| `ABACATE_PAY_API_KEY` + `ABACATE_PAY_WEBHOOK_SECRET` | Gateway PIX/Cartão | Sim (pagamentos) |
| `METRICS_TOKEN` | Protege `GET /metrics` (Bearer token). Vazio = endpoint público | Recomendado em produção |
| `ALLOWED_ORIGINS` | Domínios permitidos (CORS), separados por vírgula | Não (usa defaults) |
| `OSRM_BASE_URL` | Servidor OSRM próprio. Vazio = demo público (só dev) | Não |
| `PORT` | Porta do servidor | Não (default: 3000) |
| `GO_ENV` | `production` / `development` | Não |

### Frontends

| Variável | Onde | Descrição |
|---|---|---|
| `REACT_APP_API_URL` | WebRestaurant/WebAdmin (Render/compose) | URL base do monolito |
| `VITE_API_URL` | WebRestaurant (dev local) | URL base do monolito |
| `API_URL` | Apps mobile (`config/api.ts`) | URL base do monolito (compilada no build) |

## Testes

```bash
# Todos os módulos Go (via go.work) — testify + miniredis, sem Docker
go test ./...

# Módulo individual
cd cmd/fuudelivery && go test ./...
cd Backend/payment_api && go test ./...

# Integração (build tag integration; requer Docker — sobe containers reais)
cd cmd/fuudelivery && go test -tags=integration -v -run 'TestFullFlow|TestErrorScenarios|TestAdminBootstrap' ./
cd Backend/payment_api && go test -tags=integration -v -run 'TestCheckoutE2E' ./app/handlers/

# Frontends web (Vitest)
cd Frontend/WebRestaurant && npm test
cd Frontend/WebAdmin && npm test
```

- **35 arquivos de teste Go** distribuídos em Backend (26), cmd (6) e pkg (3), incluindo E2E de checkout, webhook HMAC, rate limiting com Redis e fluxo completo de pedido.
- Integração usa **testcontainers-go** (postgres/mongodb) localmente; no CI, containers `mongo:7` e `postgres:16-alpine` via variáveis `MONGO_TEST_URI`/`POSTGRES_TEST_URI`.
- Mobile usa jest-expo com `--passWithNoTests`.
- Cobertura planejada e status: [`references/testes-ci.md`](references/testes-ci.md).

## CI/CD

7 workflows em `.github/workflows/`:

| Workflow | Trigger | O que faz |
|---|---|---|
| `ci.yml` (**CI Gate**) | push/PR em master | Matriz dos 10 módulos Go (tidy/build/vet/test), integração PG+Mongo, checkout E2E, gofmt, govulncheck, shellcheck, vitest+build dos webs, npm audit (critical), validação de workflows, consistência de URLs mobile e guarda de regressão CSS (Tailwind v4 @layer), e2e dual-write |
| `deploy.yml` | `workflow_run` após CI verde (ou manual) | Deploy matriz api/web/admin na API do Render com retry 3x + polling até `live` + health-check pós-deploy + verificação das rotas SPA |
| `monitor.yml` | cron `*/30` | `scripts/verify-deploy.sh` contra os 3 serviços; falha o job se algum cair; log salvo como artefato |
| `release.yml` | tag `v*` | EAS Build dos APKs AppComida/AppEntrega → anexa em GitHub Release |
| `build-appcomida.yml` / `build-appentrega.yml` / `build-apprestaurante.yml` | push na pasta do app | `expo prebuild` + `gradlew assembleRelease` (Java 17) → artefato APK |

Deploy em produção: push direto na `master` (CI Gate → Deploy automático).

## Build dos APKs (Android)

Via **EAS Build** (nuvem):

```bash
npm install -g eas-cli
eas login

cd Frontend/AppComida
npx eas build --platform android --profile preview   # gera APK instalável
```

Mesmo comando para `AppEntrega` (e `AppRestaurante`). Alternativa automatizada: `bash scripts/build-apks.sh`. Alternativa 100% local (requer Android SDK + `ANDROID_HOME` configurado):

```bash
cd Frontend/AppComida
npx expo prebuild --clean
cd android && ./gradlew assembleRelease
# APK em android/app/build/outputs/apk/release/app-release.apk
```

### Erros comuns

<details>
<summary><b>Plugin <code>expo-module-gradle-plugin</code> was not found</b></summary>

Causa: autolinking quebrado por versões desalinhadas de `expo`/`expo-secure-store`/`expo-modules-core`, config plugin interferindo ou patch do patch-package modificando Gradle.

```bash
npx expo install --check        # alinhar versões
npx expo prebuild --clean       # regenerar android/
# Remover config plugins extras do app.json (manter só expo-router)
# Remover patches que mexem em Gradle e o postinstall patch-package
eas build --platform android --profile preview
```

> Se um app builda e outro não, compare os `app.json` — quase sempre é plugin/patch interferindo no autolinking.
</details>

<details>
<summary><b>npm install trava / timeout</b></summary>

```bash
rm -rf node_modules package-lock.json
npm cache clean --force
npm install --legacy-peer-deps
```
</details>

<details>
<summary><b>Build funciona local mas falha no EAS</b></summary>

Fixe a versão do CLI no `eas.json`:

```json
{ "cli": { "version": ">= 7.3.0" }, "build": { "preview": { "android": { "buildType": "apk" } } } }
```
</details>

## Documentação

| Documento | Conteúdo |
|---|---|
| [`PRODUCTION.md`](PRODUCTION.md) | Checklist e configuração de produção |
| [`docs/arquitetura.md`](docs/arquitetura.md) | Visão arquitetural detalhada |
| [`docs/banco-de-dados.md`](docs/banco-de-dados.md) | Dicionário de dados completo |
| [`docs/ARQUITETURA-BANCO-UNICO.md`](docs/ARQUITETURA-BANCO-UNICO.md) | Estratégia e cortes do banco único |
| [`docs/guia-deploy.md`](docs/guia-deploy.md) + [`scripts/deploy-vps.md`](scripts/deploy-vps.md) | Deploy Render e VPS |
| [`docs/runbook-rotacao-credenciais.md`](docs/runbook-rotacao-credenciais.md) | Rotação de credenciais |
| [`docs/seguranca.md`](docs/seguranca.md) + [`SECURITY.md`](SECURITY.md) | Práticas de segurança e reporte de vulnerabilidades |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Guia de contribuição |
| [`references/URLS.md`](references/URLS.md) | Mapa canônico de URLs de produção |
| [`references/gaps-funcionais.md`](references/gaps-funcionais.md) | TODOs e decisões de arquitetura |
| [`MANIFEST.md`](MANIFEST.md) | Manifesto do projeto |

## Licença

Apache License 2.0 — ver [LICENSE](LICENSE).
