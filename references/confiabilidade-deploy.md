# Confiabilidade e Deploy — FuuDelivery

## Arquitetura de deploy

```
github.com/MarcosPavanBR/fuudelivery
    │
    ├── Push to master
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
| `RABBITMQ_URL` | RabbitMQ Cloud (ou vazio) |

#### fuudelivery-payment
| Variável | Fonte |
|---|---|
| `MONGODB_URI` | Atlas (fuudelivery_payments) |
| `JWT_SECRET` | MESMO da API |
| `ADMIN_PASSWORD` | Gerado localmente |
| `ABACATEPAY_API_KEY` | AbacatePay dashboard |
| `ABACATEPAY_WEBHOOK_SECRET` | AbacatePay dashboard |
| `BOOTSTRAP_SECRET` | Gerado localmente |
| `PORT` | 8084 |

## Confiabilidade da fila

### Arquitetura atual

O sistema usa Redis como fila/pubsub com fallback para canais Go em memória:

```
Producer (API) → Redis Channel → Consumer (Payment)
                    ↓ (se Redis indisponível)
              Go Channel (in-memory) → Consumer
```

### Risco: Perda silenciosa de eventos no fallback

Quando o Redis cai:
1. O producer continua publicando em canais Go em memória
2. O consumer continua consumindo
3. **MAS**: se o consumer reiniciar, os eventos em memória são perdidos
4. Não há persistência, não há retry, não há dead letter queue

### Mitigação

- Monitorar se o fallback está ativo (log quando `REDIS_URL` não está configurado)
- Em produção, o Redis do Render tem alta disponibilidade — o fallback é para dev/local
- **NÃO confiar no fallback em memória para dados financeiros**

### O que acontece se o Redis cair em produção

1. Pagamentos já processados não são afetados
2. Novos pagamentos continuam sendo recebidos (API não depende de Redis)
3. A fila de processamento assíncrono para em memória
4. Se o consumer reiniciar, pagamentos pendentes na fila em memória são perdidos
5. **Ação necessária**: reprocessar manualmente os pagamentos pendentes ou usar retry do AbacatePay

## Rollback

1. Render Dashboard → Serviço → Manual Deploy → Rollback to previous deploy
2. OU: revert do commit + push (triggera deploy automático)
3. **IMPORTANTE**: rollback de banco (Supabase/Atlas) não é automático — ter migration reversa

## Monitoramento sugerido

- **Uptime**: UptimeRobot ou BetterStack para os 5 endpoints `/health`
- **Erros**: Sentry no frontend, logs no Render
- **Métricas**: Render Metrics (CPU, memória, request time)
- **Alertas**: Slack/Discord webhook para erros 5xx
