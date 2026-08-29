---
title: Relatório Técnico de Auditoria, Debug e Otimização — Fuudelivery
date: 2026-08-30
repository: https://github.com/MarcosPavanBR/fuudelivery
stack_principal:
  backend: Go 1.25+, Fiber, GORM, PostgreSQL, Redis Streams, JWT, multi-gateway de pagamento
  frontend_web: React 19, Vite 6, Tailwind CSS 4, react-router-dom, WebSocket
  mobile: React Native, Expo SDK 54, expo-router, NativeWind, MapLibre
escopo: >
  Simulação de execução, diagnóstico técnico, pontos de debug, segurança,
  performance, resiliência, observabilidade e plano de otimização.
status: Auditoria técnica profunda
---

# Relatório Técnico de Auditoria, Debug e Otimização — Fuudelivery

## 1. Sumário Executivo

O projeto **Fuudelivery** apresenta uma arquitetura moderna e ambiciosa, combinando:

- Backend em **Go** com framework **Fiber**;
- Persistência em **PostgreSQL**, com uso de **GORM**;
- Filas e eventos assíncronos com **Redis Streams**;
- Sistema de pagamentos com múltiplos gateways: **Pagar.me, Asaas, AbacatePay e Mercado Pago**;
- Frontends web em **React 19 + Vite 6 + Tailwind CSS 4**;
- Aplicativos mobile em **React Native + Expo SDK 54**;
- Recursos de chat em tempo real via **WebSocket**;
- Integração com **OSRM** para cálculo de rotas e distâncias;
- Estrutura monolítica modularizada com tendência futura de extração para serviços.

O projeto é funcional como MVP, mas para operar em produção com segurança, alta disponibilidade, integridade financeira e escalabilidade, são necessários ajustes críticos em:

1. Segurança de autenticação e autorização;
2. Validação e idempotência de webhooks de pagamento;
3. Resiliência de filas com Redis Streams;
4. Controle de conexões com PostgreSQL e PgBouncer;
5. Prevenção de vazamento de goroutines e uso incorreto do contexto do Fiber;
6. Observabilidade distribuída;
7. Otimização do motor de despacho de entregadores;
8. Performance dos apps mobile, especialmente mapas e WebSocket;
9. Estratégia de migração de banco de dados sem `AutoMigrate` em produção;
10. Testes de integração e carga mais realistas.

Este relatório descreve:

- Simulação de execução;
- Debugs necessários por camada;
- Riscos de segurança;
- Otimizações técnicas recomendadas;
- Plano de ação priorizado;
- Checklist operacional.

---

## 2. Premissas da Simulação

A análise foi feita considerando dois cenários:

### 2.1 Cenário de desenvolvimento local

- Desenvolvedor clona o repositório;
- Instala dependências Go e Node;
- Sobe PostgreSQL e Redis localmente ou via Docker;
- Configura arquivo `.env`;
- Executa o backend com:

```bash
cd cmd/fuudelivery
go mod tidy
go run main.go
```

- Executa frontends web com:

```bash
cd Frontend/WebRestaurant
npm install --legacy-peer-deps
npm run dev
```

ou:

```bash
cd Frontend/WebAdmin
npm install --legacy-peer-deps
npm run dev
```

- Executa apps mobile com:

```bash
cd Frontend/AppComida
npm install
npx expo start
```

### 2.2 Cenário de produção

- Backend rodando em múltiplas instâncias;
- PostgreSQL gerenciado via Supabase com PgBouncer;
- Redis para cache, rate limit e filas;
- Webhooks de pagamento recebidos em ambiente público;
- Apps mobile e web conectados simultaneamente via WebSocket;
- Pico de pedidos em horários de refeição;
- Necessidade de tolerância a falhas de gateways externos.

---

## 3. Instalação e Bootstrap do Projeto

### 3.1 Clonagem do repositório

```bash
git clone https://github.com/MarcosPavanBR/fuudelivery.git
cd fuudelivery
```

### 3.2 Pré-requisitos mínimos

| Tecnologia | Versão recomendada | Observação |
|---|---:|---|
| Go | 1.25+ | Backend e workers |
| Node.js | 20+ | Frontend web e apps Expo |
| npm | 10+ | Gerenciador de pacotes |
| Docker | Última estável | PostgreSQL, Redis e testes |
| Git | Última estável | Controle de versão |

### 3.3 Configuração de variáveis de ambiente

Copie o arquivo de exemplo:

```bash
cp .env.example .env
```

Variáveis obrigatórias para o backend:

```env
GO_ENV=development
PORT=3000

# Segurança
JWT_SECRET=troque_por_segredo_forte
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=7d

# Banco de dados
DB_CONNECTION_STRING=postgres://usuario:senha@localhost:5432/fuudelivery?sslmode=disable

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
```

Variáveis recomendadas para produção:

```env
GO_ENV=production
PORT=3000

# Segurança
JWT_SECRET=__usar_secret_manager__
JWT_ISSUER=fuudelivery.auth
JWT_AUDIENCE=fuudelivery.clients

# Banco
DB_CONNECTION_STRING=__usar_secret_manager__
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=15m

# Redis
REDIS_ADDR=__redis-endpoint__
REDIS_PASSWORD=__usar_secret_manager__
REDIS_DB=0

# Pagamentos
PAGARME_API_KEY=__secret__
PAGARME_ENCRYPTION_KEY=__secret__
ASAAS_API_KEY=__secret__
ABACATEPAY_API_KEY=__secret__
MERCADOPAGO_ACCESS_TOKEN=__secret__

# Observabilidade
OTEL_EXPORTER_OTLP_ENDPOINT=__collector_endpoint__
LOG_LEVEL=info
```

---

## 4. Simulação de Execução e Debugs Necessários

## 4.1 Backend Go

### 4.1.1 Execução básica

```bash
cd cmd/fuudelivery
go mod tidy
go run main.go
```

### 4.1.2 Falha esperada sem variáveis obrigatórias

Se `JWT_SECRET` ou `DB_CONNECTION_STRING` estiverem ausentes em produção, o processo deve abortar.

Comportamento recomendado:

```text
FATAL missing required environment variable: JWT_SECRET
FATAL missing required environment variable: DB_CONNECTION_STRING
```

Se isso não ocorrer, o sistema está inseguro.

### 4.1.3 Debug recomendado

Implementar ou validar um modo de verificação de configuração:

```bash
go run main.go --check-config
```

Esse comando deveria:

- Validar variáveis obrigatórias;
- Testar conexão com PostgreSQL;
- Testar conexão com Redis;
- Validar permissões mínimas no banco;
- Validar configuração de JWT;
- Validar credenciais básicas dos gateways;
- Não iniciar servidor HTTP.

### 4.1.4 Problemas comuns

| Problema | Causa provável | Correção |
|---|---|---|
| `connection refused` no PostgreSQL | Banco não iniciado | Subir PostgreSQL via Docker ou serviço local |
| `password authentication failed` | Credenciais inválidas | Corrigir `DB_CONNECTION_STRING` |
| `redis: connection refused` | Redis não iniciado | Subir Redis local ou container |
| `panic` no startup | Variável ausente ou configuração inválida | Validar `.env` e fail-fast |
| Porta em uso | Outro processo usando 3000 | Alterar `PORT` ou matar processo |

---

## 4.2 Debugs no Fiber e Runtime Go

### 4.2.1 Risco crítico: uso incorreto do contexto do Fiber

O Fiber é baseado no `fasthttp`, que reutiliza objetos de contexto para reduzir pressão no garbage collector.

Exemplo perigoso:

```go
app.Get("/orders", func(c *fiber.Ctx) error {
    go func() {
        user := c.Locals("user")
        sendNotification(user)
    }()
    return c.SendStatus(fiber.StatusOK)
})
```

Esse padrão pode causar:

- Data race;
- Panic;
- Leitura de dados de outra requisição;
- Corrupção de contexto;
- Vazamento de goroutine.

### 4.2.2 Correção recomendada

Nunca passar `*fiber.Ctx` para goroutines.

Exemplo seguro:

```go
app.Get("/orders", func(c *fiber.Ctx) error {
    ctx := c.UserContext()
    user := c.Locals("user")

    go func(ctx context.Context, user any) {
        sendNotification(ctx, user)
    }(ctx, user)

    return c.SendStatus(fiber.StatusOK)
})
```

### 4.2.3 Debug necessário

Adicionar linting e revisão manual para impedir:

- Goroutines recebendo `*fiber.Ctx`;
- Handlers sem timeout;
- Handlers bloqueantes sem contexto;
- WebSocket sem deadline;
- Operações longas dentro da request HTTP.

### 4.2.4 Recomendações de runtime

Configurar:

- `ReadTimeout`;
- `WriteTimeout`;
- `IdleTimeout`;
- `BodyLimit`;
- `Concurrency`;
- Graceful shutdown com timeout máximo.

Exemplo conceitual:

```go
app := fiber.New(fiber.Config{
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  60 * time.Second,
    BodyLimit:    4 * 1024 * 1024,
})
```

### 4.2.5 Graceful shutdown

O backend deve terminar conexões ativas antes de encerrar.

Recomendação:

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

<-quit

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

_ = app.ShutdownWithContext(ctx)
```

---

## 4.3 Debugs no PostgreSQL e GORM

### 4.3.1 Problema: AutoMigrate em produção

O projeto menciona uso de `AutoMigrate` no startup.

Isso é aceitável apenas em desenvolvimento local.

Em produção, `AutoMigrate` pode causar:

- Locks longos;
- Alterações acidentais de schema;
- Incompatibilidade entre múltiplas instâncias;
- Downtime durante rolling update;
- Dificuldade de auditoria;
- Impossibilidade de rollback controlado.

### 4.3.2 Correção recomendada

Remover `AutoMigrate` do boot principal.

Adotar migrações versionadas com:

- `golang-migrate`;
- `goose`;
- `Atlas`;
- SQL puro versionado.

Exemplo de fluxo:

```bash
migrate -path ./sql -database "$DB_CONNECTION_STRING" up
```

### 4.3.3 Problema: GORM + PgBouncer em modo transaction pooling

O Supabase pode usar PgBouncer na porta 6543.

Se o PgBouncer estiver em modo `transaction`, prepared statements podem causar erros como:

```text
prepared statement "stmt_1" does not exist
```

### 4.3.4 Correção recomendada

Ao usar GORM com PgBouncer:

```go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    PrepareStmt: false,
})
```

Ou usar `pgx` diretamente para caminhos críticos.

### 4.3.5 Connection pool

Configurar explicitamente:

```go
sqlDB, err := db.DB()
if err != nil {
    log.Fatal(err)
}

sqlDB.SetMaxOpenConns(50)
sqlDB.SetMaxIdleConns(10)
sqlDB.SetConnMaxLifetime(15 * time.Minute)
sqlDB.SetConnMaxIdleTime(5 * time.Minute)
```

### 4.3.6 Debug recomendado

Habilitar logs lentos e métricas:

- Query duration;
- Open connections;
- Idle connections;
- Wait time;
- Connection errors;
- Deadlocks;
- Lock waits.

---

## 4.4 Debugs no Redis e Redis Streams

### 4.4.1 Uso atual

O Redis é usado para:

- Filas com Redis Streams;
- Cache;
- Rate limit;
- Sessões temporárias;
- Estado de curto prazo.

### 4.4.2 Problema: mensagens pendentes após crash de worker

Quando um worker consome uma mensagem com `XREADGROUP`, ela fica pendente até receber `XACK`.

Se o worker cair antes do `XACK`, a mensagem fica retida.

### 4.4.3 Correção recomendada

Implementar um processo reaper/claim:

1. Executar periodicamente `XPENDING`;
2. Identificar mensagens pendentes há mais de X segundos;
3. Executar `XCLAIM` para outro consumidor;
4. Reentregar com limite máximo de tentativas;
5. Após N falhas, mover para DLQ.

### 4.4.4 Estrutura recomendada para filas

Cada stream deve possuir:

- Consumer group;
- DLQ;
- Métricas de atraso;
- Métricas de falha;
- Retry com backoff exponencial;
- Idempotency key;
- Timestamp de criação;
- Trace ID.

Exemplo conceitual de payload:

```json
{
  "event_id": "01919c5a-6f6c-7c1e-9a7e-7d5c0f4f8c9a",
  "event_type": "order.created",
  "occurred_at": "2026-08-30T18:00:00Z",
  "trace_id": "trace_abc123",
  "payload": {
    "order_id": 42
  }
}
```

### 4.4.5 Fallback in-memory

O fallback in-memory pode ser útil em desenvolvimento, mas é perigoso em produção.

Riscos:

- Split-brain;
- Mensagens diferentes entre instâncias;
- Perda de eventos em restart;
- Impossibilidade de reprocessamento;
- Falta de visibilidade operacional.

Recomendação:

- Em produção, usar apenas Redis;
- Se Redis cair, aplicar circuit breaker e degradação controlada;
- Não fingir que a fila está funcionando quando não está.

---

## 4.5 Debugs no sistema de pagamentos

### 4.5.1 Risco de duplicate processing

Gateways podem reenviar webhooks por timeout, retry ou instabilidade de rede.

Sem idempotência, o sistema pode:

- Confirmar pagamento duas vezes;
- Duplicar split;
- Atualizar pedido indevidamente;
- Creditar carteira duas vezes;
- Gerar inconsistência financeira.

### 4.5.2 Correção recomendada

Para cada webhook:

1. Validar assinatura HMAC;
2. Validar timestamp;
3. Extrair `external_payment_id`;
4. Tentar adquirir lock distribuído no Redis;
5. Verificar se o evento já foi processado;
6. Persistir evento como recebido;
7. Processar mudança de estado;
8. Confirmar sucesso somente após commit.

### 4.5.3 Idempotência

Criar tabela de eventos recebidos:

```sql
CREATE TABLE payment_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway TEXT NOT NULL,
    external_event_id TEXT NOT NULL,
    payment_id TEXT NOT NULL,
    status TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (gateway, external_event_id)
);
```

### 4.5.4 Circuit breaker distribuído

Se o estado do circuit breaker ficar apenas em memória, cada instância terá comportamento diferente.

Exemplo:

- Instância A abre circuito para Pagar.me;
- Instância B continua chamando Pagar.me;
- Instância C pode alternar comportamento;
- O sistema fica imprevisível.

Recomendação:

- Usar Redis para estado do circuit breaker;
- Usar scripts Lua para operações atômicas;
- Ou delegar egress resilience para service mesh/API gateway.

---

## 4.6 Debugs no WebSocket e chat

### 4.6.1 Risco de conexões órfãs

Clientes mobile frequentemente alternam entre:

- Wi-Fi;
- 4G/5G;
- Modo avião;
- Bloqueio de tela;
- Suspensão de app.

Se o servidor não detectar conexões mortas, haverá:

- Vazamento de memória;
- Goroutines paradas;
- Canais cheios;
- Aumento de latência;
- Consumo desnecessário de CPU.

### 4.6.2 Correção recomendada

Implementar heartbeat:

- Servidor envia ping a cada 20-30 segundos;
- Cliente responde pong;
- Conexão é fechada se não responder;
- Cliente reconecta com backoff exponencial e jitter.

### 4.6.3 Reconexão no frontend

Evitar reconexão imediata em massa.

Exemplo de lógica:

```ts
const delay = Math.min(
  BASE_DELAY * Math.pow(2, attempt) + Math.random() * 1000,
  MAX_DELAY
);
```

### 4.6.4 Backpressure

Evitar enviar eventos demais para clientes lentos.

Recomendações:

- Buffer limitado por conexão;
- Descartar eventos não críticos se buffer cheio;
- Enviar snapshot de estado após reconexão;
- Usar canais com `select` e `default` para não bloquear workers.

---

## 4.7 Debugs no Frontend Web

### 4.7.1 Dependências

Instalar com:

```bash
npm install --legacy-peer-deps
```

Motivo provável:

- React 19 ainda pode gerar conflitos com bibliotecas que declaram peer dependencies antigas;
- Tailwind 4 pode exigir plugins específicos;
- Bibliotecas de WebSocket, DnD e roteamento podem não estar totalmente compatíveis.

### 4.7.2 Problemas comuns

| Problema | Causa | Correção |
|---|---|---|
| `ERESOLVE` | Peer dependencies conflitantes | Usar `--legacy-peer-deps` ou overrides |
| Página em branco | Variável de API ausente | Definir `VITE_API_URL` |
| WebSocket não conecta | CORS ou URL incorreta | Validar origem e endpoint |
| HMR instável | Cache do Vite | Rodar com `--force` |
| Build lento | Dependências não otimizadas | Analisar bundle com `vite-bundle-visualizer` |

### 4.7.3 Variáveis recomendadas

```env
VITE_API_URL=http://localhost:3000
VITE_WS_URL=ws://localhost:3000
VITE_ENV=development
```

### 4.7.4 Otimizações recomendadas

- Code splitting por rota;
- Lazy loading de telas pesadas;
- Virtualização de listas longas;
- Cache de consultas com TanStack Query;
- Debounce para buscas;
- Compressão Brotli/Gzip no servidor;
- Cache de assets estáticos;
- Service worker para PWA quando aplicável.

---

## 4.8 Debugs nos Apps Mobile

### 4.8.1 Execução

```bash
cd Frontend/AppComida
npm install
npx expo start
```

Se houver problemas de cache:

```bash
npx expo start -c
```

### 4.8.2 Problemas comuns

| Problema | Causa | Correção |
|---|---|---|
| App não conecta na API | URL localhost inválida no device | Usar IP da máquina ou túnel |
| Mapa não renderiza | Dependência nativa ausente | Rodar prebuild e validar MapLibre |
| Permissão de localização negada | Configuração ausente | Configurar `AndroidManifest.xml` e `Info.plist` |
| Build nativo falha | Versão incompatível | Rodar `npx expo install --fix` |
| Lentidão no mapa | Muitos marcadores | Usar GeoJSON + clustering |

### 4.8.3 Configuração de API local

No emulador Android, `localhost` pode não apontar para a máquina do desenvolvedor.

Alternativas:

- Usar IP da rede local;
- Usar túnel do Expo;
- Usar `adb reverse` para Android local.

Exemplo:

```bash
adb reverse tcp:3000 tcp:3000
```

### 4.8.4 MapLibre e performance

Evitar:

```tsx
{deliveries.map(delivery => (
  <Marker key={delivery.id} ... />
))}
```

Para grande volume, preferir:

- `ShapeSource`;
- GeoJSON;
- Clusterização;
- Atualização por viewport;
- Renderização nativa.

---

## 5. Segurança da Aplicação

## 5.1 Autenticação

### 5.1.1 JWT HS256 vs RS256

O uso de HS256 implica segredo simétrico compartilhado.

Riscos:

- Qualquer serviço que valide token conhece o segredo;
- Vazamento do segredo permite forjar tokens;
- Rotação de segredo é complexa;
- Dificuldade para auditoria de chaves.

Recomendação:

- Migrar para RS256 ou ES256;
- Publicar chaves públicas via JWKS;
- Usar `kid` para rotação de chaves;
- Armazenar chave privada apenas no serviço de autenticação.

### 5.1.2 Claims recomendadas

```json
{
  "sub": "user_id",
  "iss": "fuudelivery.auth",
  "aud": "fuudelivery.api",
  "exp": 1760000000,
  "iat": 1760000000,
  "jti": "unique_token_id",
  "role": "customer",
  "tenant_id": "restaurant_id"
}
```

### 5.1.3 Refresh tokens

Recomendações:

- Refresh token opaco;
- Armazenado no Redis;
- Rotação a cada uso;
- Revogação por família de tokens;
- Detecção de reuse;
- TTL curto para access token;
- TTL maior, porém limitado, para refresh token.

---

## 5.2 Autorização

### 5.2.1 RBAC/ABAC

O sistema precisa distinguir:

- Cliente;
- Restaurante;
- Entregador;
- Administrador;
- Suporte;
- Financeiro;
- Operações.

Não confiar apenas no papel. Validar propriedade do recurso.

Exemplo:

- Restaurante A não pode acessar pedido do restaurante B;
- Entregador só pode acessar entrega atribuída a ele;
- Cliente só pode acessar seu próprio pedido;
- Admin deve ter trilhas de auditoria.

### 5.2.2 Middleware recomendado

Implementar autorização por recurso:

```go
func RequireOwnerOrAdmin(resource string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        // validar se usuário tem acesso ao resource_id
        return c.Next()
    }
}
```

---

## 5.3 Webhooks

### 5.3.1 Validação obrigatória

Todo webhook deve validar:

- Assinatura HMAC;
- Timestamp;
- Nonce ou evento único;
- Idempotência;
- Origem confiável;
- Payload schema.

### 5.3.2 Proteção contra replay

Rejeitar eventos com timestamp muito antigo:

```go
const maxTimestampDrift = 5 * time.Minute
```

### 5.3.3 Resposta rápida

O endpoint deve responder rapidamente.

Padrão recomendado:

1. Validar assinatura;
2. Salvar evento bruto;
3. Enfileirar processamento;
4. Retornar `200 OK`;
5. Processar assincronamente.

---

## 5.4 Proteção de dados

### 5.4.1 Dados sensíveis

Nunca logar:

- Cartão;
- CPF;
- Telefone completo quando evitável;
- Tokens;
- Chaves de gateway;
- Senhas;
- Refresh tokens;
- Connection strings.

### 5.4.2 Máscara de logs

Exemplo:

```text
phone=+5511******34
email=m***@domain.com
card_last4=4242
```

### 5.4.3 PCI-DSS

Se o sistema não toca dados de cartão diretamente, melhor.

Pagamentos devem preferencialmente ser feitos via:

- Checkout transparente tokenizado;
- Tokenização do gateway;
- SDK oficial do gateway;
- Redirecionamento seguro quando aplicável.

---

## 5.5 Secrets Management

### 5.5.1 Problema

Arquivo `.env` é aceitável em desenvolvimento local.

Em produção, não usar `.env` diretamente se houver alternativa mais segura.

### 5.5.2 Recomendações

- Usar secret manager;
- Injetar secrets via ambiente no container;
- Nunca commitar `.env`;
- Rotacionar chaves periodicamente;
- Separar segredos por ambiente;
- Auditar acesso a segredos.

Ferramentas possíveis:

- AWS Secrets Manager;
- GCP Secret Manager;
- Azure Key Vault;
- HashiCorp Vault;
- Doppler;
- Infisical.

---

## 5.6 Rate Limiting e Abuso

### 5.6.1 Endpoints críticos

Aplicar rate limit mais rígido em:

- Login;
- Registro;
- Recuperação de senha;
- Criação de pedido;
- Pagamento;
- Upload de imagem;
- Webhooks;
- WebSocket upgrade.

### 5.6.2 Estratégia

- Rate limit por IP;
- Rate limit por usuário;
- Rate limit por dispositivo;
- Rate limit por restaurante;
- Rate limit por entregador;
- Burst controlado;
- Penalidade progressiva.

---

## 6. Otimizações de Performance

## 6.1 Backend

### 6.1.1 Consultas N+1

Sintoma:

- Endpoint de pedidos retorna lista;
- Para cada pedido, busca itens, pagamentos, restaurante, entregador.

Correção:

- Usar `Preload` com critério;
- Usar joins manuais para relatórios;
- Criar DTOs de leitura;
- Evitar carregar entidades completas desnecessárias.

### 6.1.2 Cache

Candidatos a cache:

- Cardápio;
- Categorias;
- Restaurantes abertos;
- Configurações públicas;
- Taxas de entrega por zona;
- Preços de produtos;
- Sessões de usuário;
- JWKS público.

Cuidados:

- Invalidation por evento;
- TTL curto para dados mutáveis;
- Cache stampede prevention;
- Singleflight para chaves quentes.

### 6.1.3 Singleflight

Evitar que 1000 requests simultâneas recriem o mesmo cache.

Exemplo conceitual:

```go
result, err, shared := singleflightGroup.Do(key, func() (any, error) {
    return loadFromDatabase(ctx, key)
})
```

### 6.1.4 Compressão

Habilitar compressão HTTP:

- Brotli se disponível;
- Gzip como fallback;
- Apenas para respostas textuais/JSON;
- Cuidado com CPU em alta escala.

---

## 6.2 Banco de Dados

### 6.2.1 Índices recomendados

Tabela `orders`:

```sql
CREATE INDEX idx_orders_status_created_at
ON orders (status, created_at DESC);

CREATE INDEX idx_orders_customer_id_created_at
ON orders (customer_id, created_at DESC);

CREATE INDEX idx_orders_restaurant_id_status
ON orders (restaurant_id, status);
```

Tabela `deliveries`:

```sql
CREATE INDEX idx_deliveries_status_assigned_at
ON deliveries (status, assigned_at DESC);
```

Tabela `payment_webhook_events`:

```sql
CREATE INDEX idx_payment_webhook_events_payment_id
ON payment_webhook_events (payment_id);

CREATE INDEX idx_payment_webhook_events_created_at
ON payment_webhook_events (created_at DESC);
```

### 6.2.2 PostGIS para despacho

Em vez de chamar OSRM para todos os entregadores:

1. Usar PostGIS para filtrar entregadores próximos;
2. Ordenar por distância aproximada;
3. Selecionar top N candidatos;
4. Chamar OSRM apenas para refinamento de rota/ETA.

Exemplo conceitual:

```sql
SELECT id
FROM couriers
WHERE ST_DWithin(
  location,
  ST_MakePoint(:restaurant_lng, :restaurant_lat)::geography,
  :radius_meters
)
ORDER BY location <-> ST_MakePoint(:restaurant_lng, :restaurant_lat)
LIMIT 10;
```

### 6.2.3 Particionamento

Para tabelas de alto volume:

- `orders`;
- `order_documents`;
- `payments`;
- `ledger`;
- `webhook_events`.

Estratégia:

- Particionamento por mês;
- Retenção por particionamento;
- Índices locais;
- Queries sempre com filtro temporal.

---

## 6.3 Redis

### 6.3.1 Cache com TTL adequado

| Dado | TTL sugerido |
|---|---:|
| Cardápio público | 30-120 segundos |
| Configurações | 5-15 minutos |
| Sessão curta | 15 minutos |
| Rate limit | conforme janela |
| Locks | 5-30 segundos |
| Estado de circuit breaker | 10-60 segundos |

### 6.3.2 Locks distribuídos

Para operações críticas:

- Pagamento;
- Split financeiro;
- Atribuição de entrega;
- Transições de pedido;
- Saque/carteira.

Recomendações:

- Lock com TTL curto;
- Token de lock único;
- Renovação apenas se necessário;
- Liberação com script Lua;
- Observabilidade de lock contention.

---

## 6.4 Motor de despacho

### 6.4.1 Problema

Em pico de pedidos, o dispatch engine pode tentar casar pedidos com entregadores de forma síncrona e custosa.

### 6.4.2 Otimização recomendada

Implementar pipeline assíncrono:

1. Pedido confirmado cria evento `dispatch.requested`;
2. Worker calcula zona e restaurante;
3. Busca entregadores elegíveis via PostGIS/índice geoespacial;
4. Aplica filtros de disponibilidade e reputação;
5. Seleciona candidatos;
6. Calcula ETA real via OSRM apenas para finalistas;
7. Faz oferta ao entregador;
8. Monitora timeout e fallback.

### 6.4.3 Métricas

- Tempo de matching;
- Taxa de aceitação;
- Distância média;
- ETA estimado vs real;
- Entregadores ociosos;
- Pedidos sem entregador;
- Cancelamentos por timeout.

---

## 6.5 Frontend Web

### 6.5.1 Bundle

Analisar tamanho:

```bash
npm run build -- --mode production
```

Recomendações:

- Dividir chunks por rota;
- Lazy load de componentes pesados;
- Remover dependências não usadas;
- Tree shaking correto;
- Evitar bibliotecas gigantes por uma função simples.

### 6.5.2 Estado em tempo real

Para WebSocket:

- Separar estado de conexão;
- Reconciliar estado após reconexão;
- Evitar re-render excessivo;
- Usar subscriptions seletivas;
- Debounce para eventos de alta frequência.

### 6.5.3 Kanban

Para quadro de pedidos:

- Virtualização de colunas;
- Drag-and-drop otimista;
- Atualização por eventos;
- Fallback polling leve;
- Indicadores de conexão.

---

## 6.6 Mobile

### 6.6.1 Hermes

Garantir Hermes ativado para melhorar startup e memória.

### 6.6.2 Imagens

- Usar WebP/AVIF;
- Redimensionar no CDN/Storage;
- Lazy loading;
- Cache local;
- Placeholder de baixa resolução.

### 6.6.3 Offline-first

Para apps de entregador:

- Cache de tarefas atuais;
- Fila de eventos offline;
- Sincronização com retry;
- Resolução de conflitos por timestamp ou versão.

### 6.6.4 Localização

- Atualização de posição com throttling;
- Precisão balanceada por contexto;
- Parar tracking quando não houver entrega ativa;
- Enviar apenas deltas relevantes.

---

## 7. Observabilidade

## 7.1 Logs

Logs devem ser estruturados em JSON.

Campos mínimos:

```json
{
  "timestamp": "2026-08-30T18:00:00Z",
  "level": "info",
  "service": "fuudelivery-api",
  "trace_id": "abc123",
  "span_id": "def456",
  "message": "payment processed",
  "payment_id": "pay_123",
  "duration_ms": 231
}
```

Nunca logar segredos.

## 7.2 Métricas

### Backend

- Requests por segundo;
- Latência p50/p95/p99;
- Taxa de erro;
- Goroutines ativas;
- Memória heap;
- GC pause;
- Conexões DB abertas;
- Conexões Redis;
- Mensagens na fila;
- Idade da mensagem mais antiga;
- Taxa de retry;
- Taxa de DLQ.

### Pagamentos

- Autorizações por gateway;
- Taxa de sucesso;
- Taxa de recusa;
- Latência por gateway;
- Circuit breaker aberto;
- Fallbacks acionados;
- Webhooks duplicados;
- Webhooks inválidos.

### Despacho

- Pedidos aguardando entregador;
- Tempo médio de aceite;
- Entregadores ativos;
- Distância média;
- Cancelamentos;
- Recusas.

## 7.3 Tracing distribuído

Implementar OpenTelemetry.

Pontos de instrumentação:

- Middleware Fiber;
- Handlers HTTP;
- Clients HTTP externos;
- GORM;
- Redis;
- WebSocket;
- Workers de fila;
- Gateways de pagamento.

Fluxo desejado:

```text
request -> auth -> orders_api -> payment_api -> gateway -> webhook -> queue -> db
```

## 7.4 Alertas críticos

Alertar quando:

- Erro 5xx > 1% por 5 minutos;
- p95 de checkout > 2s;
- Fila com atraso > 30s;
- DLQ crescendo;
- Circuit breaker aberto;
- Webhooks inválidos aumentando;
- Conexões DB esgotando;
- Memória acima do limite;
- Restart de pods/instâncias;
- Falha de sincronização de pedidos.

---

## 8. Testes

## 8.1 Testes unitários

Cobrir:

- Regras de split;
- Cálculo de taxa de entrega;
- Validação de JWT;
- Estados de pedido;
- Circuit breaker;
- Retry/backoff;
- Formatação de erros;
- Casos de borda de pagamento.

## 8.2 Testes de integração

Com `testcontainers-go`:

- PostgreSQL real;
- Redis real;
- Fluxo de criação de pedido;
- Fluxo de pagamento;
- Webhook idempotente;
- Fila com retry;
- Despacho;
- Chat básico.

## 8.3 Testes de contrato

Para gateways externos:

- Mock de Pagar.me;
- Mock de Asaas;
- Mock de AbacatePay;
- Mock de Mercado Pago;
- Testes de timeout;
- Testes de erro HTTP;
- Testes de assinatura inválida;
- Testes de retry.

## 8.4 Testes de carga

Cenários:

- 100 usuários simultâneos;
- 1.000 pedidos em 10 minutos;
- Pico de WebSocket;
- Falha de gateway principal;
- Redis instável;
- PostgreSQL com latência alta;
- App mobile reconectando em massa.

---

## 9. CI/CD

## 9.1 Pipeline recomendado

Estágios:

1. Checkout;
2. Setup Go;
3. Setup Node;
4. Cache de dependências;
5. Lint Go;
6. Lint TypeScript;
7. Testes unitários;
8. Testes de integração;
9. Build backend;
10. Build web;
11. Build mobile;
12. Scan de segurança;
13. Build Docker;
14. Publicar imagem;
15. Deploy staging;
16. Smoke tests;
17. Deploy produção;
18. Canary/observabilidade.

## 9.2 Cache

Cache por hash:

- `go.sum`;
- `package-lock.json`;
- Docker layers;
- Expo/EAS cache quando aplicável.

## 9.3 Segurança no CI

- Secrets não expostos em logs;
- Dependabot ou Renovate;
- Scan de dependências;
- SAST;
- Container image scan;
- Assinatura de imagens;
- Políticas de aprovação.

---

## 10. Docker e Deploy

## 10.1 Dockerfile

Boas práticas:

- Multi-stage build;
- Imagem final mínima;
- Usuário não root;
- Binário estático;
- Healthcheck;
- Sem segredos na imagem;
- `.dockerignore` adequado.

## 10.2 Health checks

Endpoints recomendados:

```text
GET /health/live
GET /health/ready
GET /metrics
```

`/health/live` deve verificar apenas processo.

`/health/ready` deve verificar:

- PostgreSQL;
- Redis;
- Configurações mínimas;
- Dependências críticas opcionais com degradação.

## 10.3 Graceful shutdown

Em Kubernetes:

- `terminationGracePeriodSeconds` suficiente;
- `preStop` hook se necessário;
- Readiness probe removendo tráfego antes do shutdown;
- Workers finalizando mensagens ou devolvendo para fila.

---

## 11. Plano de Ação Priorizado

## 11.1 P0 — Crítico e imediato

| Ação | Motivo | Impacto |
|---|---|---|
| Validar e endurecer `.env` obrigatório | Evita falhas inseguras | Alto |
| Migrar ou planejar JWT assimétrico | Reduz risco de forja de token | Alto |
| Implementar idempotência em webhooks | Evita duplicidade financeira | Alto |
| Validar assinatura HMAC dos gateways | Evita eventos falsos | Alto |
| Remover AutoMigrate de produção | Evita locks e instabilidade | Alto |
| Implementar Reaper para Redis Streams | Evita mensagens presas | Alto |
| Corrigir uso de contexto Fiber | Evita data race/panic | Alto |
| Configurar connection pool DB/Redis | Evita esgotamento | Alto |
| Adicionar health checks reais | Melhora operação | Médio |
| Implementar logging estruturado | Melhora diagnóstico | Médio |

## 11.2 P1 — Curto prazo

| Ação | Motivo | Impacto |
|---|---|---|
| OpenTelemetry | Visibilidade ponta a ponta | Alto |
| Testes de integração com containers | Reduz regressões | Alto |
| Rate limiting por usuário/IP | Previne abuso | Alto |
| PostGIS para despacho | Reduz custo OSRM | Alto |
| Cache de dados de leitura | Reduz latência | Médio |
| Otimização WebSocket | Melhora estabilidade | Médio |
| Frontend code splitting | Melhora carregamento | Médio |
| Métricas de fila e pagamento | Melhora operação | Médio |

## 11.3 P2 — Médio prazo

| Ação | Motivo | Impacto |
|---|---|---|
| Circuit breaker distribuído | Consistência em múltiplas instâncias | Alto |
| Outbox pattern para eventos | Garantia de entrega | Alto |
| Particionamento de tabelas | Escala de dados | Médio |
| Feature flags | Deploy seguro | Médio |
| Testes de carga contínuos | Capacidade | Médio |
| Observabilidade mobile | Qualidade em campo | Médio |
| Estratégia offline para entregador | Resiliência | Médio |

---

## 12. Arquitetura Alvo Recomendada

## 12.1 Estado atual

Monólito Go modularizado com:

- auth_api;
- orders_api;
- delivery_api;
- payment_api;
- chat_api;
- storage integration.

## 12.2 Recomendação

Não quebrar em microsserviços prematuramente.

Fase 1:

- Monólito modular com fronteiras internas fortes;
- Filas bem definidas;
- Eventos assíncronos;
- Observabilidade;
- Testes de contrato.

Fase 2:

- Extrair workers críticos:
  - payment processor;
  - dispatch engine;
  - webhook processor;
  - notification service.

Fase 3:

- Extrair serviços somente se houver:
  - escala independente;
  - time ownership claro;
  - necessidade de isolamento de falha;
  - compliance específico.

---

## 13. Checklist Final de Produção

### 13.1 Segurança

- [ ] JWT com assinatura assimétrica ou plano de migração;
- [ ] Refresh tokens com rotação e revogação;
- [ ] Webhooks com HMAC;
- [ ] Webhooks com idempotência;
- [ ] Rate limiting ativo;
- [ ] Secrets fora do código;
- [ ] Logs sem dados sensíveis;
- [ ] Autorização por recurso;
- [ ] Upload seguro de arquivos;
- [ ] Scan de dependências.

### 13.2 Banco de dados

- [ ] Migrações versionadas;
- [ ] AutoMigrate desativado em produção;
- [ ] Índices para consultas críticas;
- [ ] Connection pool configurado;
- [ ] PgBouncer compatível com driver;
- [ ] Backup testado;
- [ ] PITR ou estratégia equivalente;
- [ ] Métricas de lock e slow queries.

### 13.3 Filas e eventos

- [ ] Redis Streams com consumer group;
- [ ] ACK correto;
- [ ] DLQ;
- [ ] Retry com backoff;
- [ ] Reaper para pending messages;
- [ ] Eventos com trace_id;
- [ ] Eventos com idempotency key;
- [ ] Métricas de atraso.

### 13.4 Pagamentos

- [ ] Idempotência por evento;
- [ ] Lock distribuído;
- [ ] Circuit breaker;
- [ ] Fallback controlado;
- [ ] Conciliação financeira;
- [ ] Logs de auditoria;
- [ ] Testes de webhook duplicado;
- [ ] Testes de timeout de gateway.

### 13.5 Observabilidade

- [ ] Logs JSON;
- [ ] Trace ID propagado;
- [ ] Métricas RED;
- [ ] Métricas de fila;
- [ ] Métricas de pagamento;
- [ ] Alertas críticos;
- [ ] Dashboards;
- [ ] Runbooks.

### 13.6 Frontend e Mobile

- [ ] Variáveis de ambiente validadas;
- [ ] Reconexão com backoff;
- [ ] Tratamento de offline;
- [ ] Cache de assets;
- [ ] Otimização de imagens;
- [ ] Mapas com clustering;
- [ ] Testes em dispositivos reais;
- [ ] Error tracking.

---

## 14. Conclusão

O Fuudelivery possui uma base técnica forte e um escopo ambicioso. Para atingir nível profissional de produção, as prioridades devem ser:

1. Segurança de autenticação e pagamentos;
2. Consistência de dados financeiros;
3. Resiliência de filas e workers;
4. Observabilidade completa;
5. Performance de despacho e mapas;
6. Testes de integração e carga;
7. Operação previsível com métricas e alertas.

A recomendação final é não iniciar produção real sem resolver os itens P0, especialmente:

- Idempotência de webhooks;
- Validação HMAC;
- Remoção de AutoMigrate;
- Tratamento de mensagens pendentes no Redis Streams;
- Correção de contexto do Fiber em goroutines;
- Configuração segura de JWT;
- Connection pooling correto com PostgreSQL/PgBouncer;
- Observabilidade mínima para diagnóstico.

Com essas correções, o sistema deixa de ser apenas um MVP funcional e passa a ter fundamentos de uma plataforma de delivery com requisitos financeiros, logísticos e operacionais de alto nível.
