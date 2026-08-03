# Confiabilidade e Deploy — FuuDelivery

## Arquitetura de deploy

```
github.com/MarcosPavanBR/fuudelivery
    │
    ├── Push to master
    │
    ├── GitHub Actions (.github/workflows/ci.yml)
    │   ├── go-modules (matrix: 7 módulos Go)
    │   ├── lint (gofmt)
    │   ├── govulncheck (matrix: 7 módulos Go)
    │   ├── frontend-webrestaurant (test + build)
    │   └── npm-audit (matrix: 3 frontends)
    │
    ├── GitHub Actions (.github/workflows/deploy.yml)
    │   └── JorgeLNJunior/render-deploy@v1
    │       └── RENDER_API_KEY + RENDER_SERVICE_ID
    │
    └── Render.com (Blueprint via render.yaml)
        ├── fuudelivery-api        (Go web service, porta 8080)
        ├── fuudelivery-web        (Static site, React)
        ├── fuudelivery-admin      (Static site, React)
        ├── fuudelivery-payment    (Go web service, porta 8084)
        ├── fuudelivery-payment-panel (Static site, React)
        └── fuudelivery-redis      (Redis managed)
```

## Checklist de deploy (pré-release)

### Antes de cada deploy

- [ ] `git status` limpo (sem mudanças não commitadas)
- [ ] `go build ./...` passa em todos os módulos
- [ ] `go vet ./...` sem warnings
- [ ] `gofmt -l -s .` retorna vazio
- [ ] `go test ./...` passa (todos os módulos)
- [ ] Nenhum `.env` com credenciais de produção commitado
- [ ] CREDENTIALS.md removido do git tracking
- [ ] CI passa no GitHub Actions

### Verificação pós-deploy

Para cada um dos 5 serviços:

```bash
# API
curl -s https://fuudelivery-api-8y6l.onrender.com/health

# Payment
curl -s https://fuudelivery-payment.onrender.com/health

# WebRestaurant (verificar se retorna HTML)
curl -s -o /dev/null -w "%{http_code}" https://fuudelivery-web.onrender.com

# WebAdmin
curl -s -o /dev/null -w "%{http_code}" https://fuudelivery-admin-lv7f.onrender.com

# PaymentPanel
curl -s -o /dev/null -w "%{http_code}" https://fuudelivery-payment-panel.onrender.com
```

### Variáveis de ambiente por serviço

#### fuudelivery-api
| Variável | Fonte |
|---|---|
| `DATABASE_URL` | Supabase |
| `REDIS_URL` | Render Redis |
| `JWT_SECRET` | Gerado localmente |
| `MONGODB_URI` | Atlas |
| `RABBIT_DELIVERY_QUEUE` | Nome da fila |
| `RABBIT_ORDER_QUEUE` | Nome da fila |

#### fuudelivery-payment
| Variável | Fonte |
|---|---|
| `MONGODB_URI` | Atlas (fuudelivery_payments) |
| `JWT_SECRET` | MESMO da API |
| `ADMIN_PASSWORD` | Gerado localmente |
| `ABACATE_PAY_API_KEY` | AbacatePay dashboard |
| `ABACATE_PAY_WEBHOOK_SECRET` | AbacatePay dashboard |
| `BOOTSTRAP_SECRET` | Gerado localmente |
| `PORT` | 8084 |
| `REDIS_URL` | Render Redis |

### Variáveis de ambiente dos Frontends

| Variável | Serviço | Valor |
|---|---|---|
| `REACT_APP_API_URL` | WebRestaurant | https://fuudelivery-api-8y6l.onrender.com |
| `REACT_APP_PAYMENT_API_URL` | WebRestaurant | https://fuudelivery-payment.onrender.com |
| `REACT_APP_API_URL` | WebAdmin | https://fuudelivery-api-8y6l.onrender.com |

## Confiabilidade da fila

### Arquitetura atual

O sistema usa **Redis Streams com consumer groups** (`pkg/queue`) como transport
de mensagens, com fallback para canais Go em memória quando o Redis não está
configurado:

```
Producer (API) ──XAdd──▶ Stream (queue:<nome>) ──XReadGroup──▶ Consumer (Payment)
                              │  ├─ XAck (confirmação explícita)
                              │  ├─ retry: falha deixa a mensagem pendente
                              │  └─ DLQ (queue:<nome>:dlq) após maxRetries
                              │
                              └─ (se Redis indisponível)
                                   Go Channel (in-memory) → Consumer
```

### Garantias (Redis Streams)

- **Entrega at-least-once**: mensagens só são removidas do pending entries list
  (PEL) após `XAck` explícito — sucesso do handler.
- **Persistência**: mensagens ficam no stream mesmo se o consumer reiniciar ou o
  deploy ocorrer no meio do processamento (sem perda de eventos).
- **Retry com limite**: handler que falha deixa a mensagem pendente; o reclaim
  loop (`XPendingExt` + `XClaim`) reprocessa após `reclaimIdle` (30s).
- **Dead-letter queue**: após `maxRetries` (3) tentativas, a mensagem é movida
  para `queue:<nome>:dlq` com o motivo da falha e a original é confirmada.
- **Reclaim pós-crash**: mensagens de consumers que caíram são reivindicadas e
  reprocessadas por outro consumer do mesmo grupo.

### Risco residual: fallback em memória (apenas dev/local)

Quando o Redis cai ou não está configurado (`REDIS_URL` ausente):
1. O producer continua publicando em canais Go em memória
2. O consumer continua consumindo
3. **MAS**: se o consumer reiniciar, os eventos em memória são perdidos
4. Não há persistência, retry nem DLQ no fallback

### Mitigação

- Monitorar se o fallback está ativo (log quando `REDIS_URL` não está configurado)
- Em produção, o Redis do Render tem alta disponibilidade — o fallback é para dev/local
- **NÃO confiar no fallback em memória para dados financeiros**
- Com `REDIS_URL` configurado, mensagens de pagamento **não se perdem** em
  restart/deploy (Streams + consumer groups)

### O que acontece se o Redis cair em produção

1. Pagamentos já processados não são afetados
2. Novos pagamentos continuam sendo recebidos (API não depende de Redis)
3. O producer tenta `XAdd` e o erro é logado; sem Redis, cai para Go channels
4. Com Redis restabelecido, os Streams retomam com as mensagens pendentes
   (reclaim) — sem reprocessamento manual
5. **Ação necessária** (apenas se o fallback em memória foi usado e o consumer
   reiniciou): reprocessar manualmente os pagamentos pendentes

### Monitorando a fila em produção

```bash
# Tamanho do stream (mensagens não processadas no backlog)
redis-cli XLEN queue:payments

# Mensagens pendentes no consumer group (não confirmadas)
redis-cli XPENDING queue:payments fuudelivery-consumers

# Dead-letter queue (mensagens que esgotaram as tentativas)
redis-cli XLEN queue:payments:dlq
redis-cli XRANGE queue:payments:dlq - +
```

> **Alerta:** se `XPENDING` cresce sem parar, há handler falhando repetidamente;
> se `queue:*:dlq` acumula, há mensagens que precisam de investigação manual.

## Rollback

1. Render Dashboard → Serviço → Manual Deploy → Rollback to previous deploy
2. OU: revert do commit + push (triggera deploy automático)
3. **IMPORTANTE**: rollback de banco (Supabase/Atlas) não é automático — ter migration reversa

## Monitoramento sugerido

- **Uptime**: UptimeRobot ou BetterStack para os 5 endpoints `/health`
- **Erros**: Sentry no frontend, logs no Render
- **Métricas**: Render Metrics (CPU, memória, request time)
- **Alertas**: Slack/Discord webhook para erros 5xx

## Domínio personalizado (futuro)

Quando o domínio `fuudelivery.com.br` estiver configurado:

1. Configurar DNS no provedor do domínio:
   - `fuudelivery.com.br` → fuudelivery-web.onrender.com
   - `api.fuudelivery.com.br` → fuudelivery-api-8y6l.onrender.com
   - `admin.fuudelivery.com.br` → fuudelivery-admin-lv7f.onrender.com
   - `payment.fuudelivery.com.br` → fuudelivery-payment.onrender.com
   - `painel.fuudelivery.com.br` → fuudelivery-payment-panel.onrender.com

2. Atualizar CORS no Backend:
   ```go
   AllowOrigins: "https://fuudelivery.com.br,https://api.fuudelivery.com.br,..."
   ```

3. Atualizar env vars dos frontends com novos domínios

4. Configurar SSL automático no Render (gratuito)

---

*Última atualização: 2026-07-31*
