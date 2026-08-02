# Guia de Deploy em Producao - FuuDelivery

## Status Atual (Agosto 2026)

| Servico | URL | Status |
|---------|-----|--------|
| API (Monolito) | fuudelivery-api-8y6l.onrender.com | Online |
| Payment Service | fuudelivery-payment.onrender.com | Online |
| WebRestaurant | fuudelivery-web.onrender.com | Online |
| WebAdmin | fuudelivery-admin-lv7f.onrender.com | Online |
| PaymentPanel | fuudelivery-payment-panel.onrender.com | Online |
| Redis | Render Managed | Online |
| MongoDB Atlas | Cloud | Connected |
| PostgreSQL (Supabase) | Cloud | Connected |

---

## Deploy Automatico

O deploy e automatizado via GitHub Actions. Push para master dispara:

1. CI workflow (go build, go vet, go test, gofmt, govulncheck, npm audit)
2. Deploy workflow (apenas se CI passar)
3. Render hot-reload automatico

### GitHub Secrets Necessarios

| Secret | Descricao |
|--------|-----------|
| RENDER_API_KEY | API key do Render |
| RENDER_SERVICE_ID_API | ID servico API |
| RENDER_SERVICE_ID_WEB | ID WebRestaurant |
| RENDER_SERVICE_ID_ADMIN | ID WebAdmin |
| RENDER_SERVICE_ID_PAYMENT | ID Payment Service |
| RENDER_SERVICE_ID_PAYMENT_PANEL | ID PaymentPanel |
| EXPO_TOKEN | Token Expo para APKs |

---

## Variaveis de Ambiente

### Monolito

| Variavel | Descricao |
|----------|-----------|
| PORT | 3000 |
| GO_ENV | production |
| JWT_SECRET | Chave JWT (secret) |
| ABACATE_PAY_WEBHOOK_SECRET | Webhook secret (secret) |
| REDIS_URL | Auto-linked pelo Render |
| DB_CONNECTION_STRING | PostgreSQL Supabase (secret) |
| MONGO_URI | MongoDB Atlas (secret) |
| MONGO_DATABASE | fuudelivery |
| APP_URL | https://fuudelivery-web.onrender.com |
| API_BASE_URL | https://fuudelivery-api.onrender.com |
| ABACATE_PAY_API_KEY | API key AbacatePay (secret) |
| ALLOWED_ORIGINS | CORS origins |

### Payment Service

| Variavel | Descricao |
|----------|-----------|
| PORT | 8084 |
| MONGO_URI | MongoDB Atlas (secret) |
| PAYMENT_MONGO_DATABASE | fuudelivery_payments |
| REDIS_URL | Auto-linked |
| JWT_SECRET | Mesmo JWT do monolito |
| ABACATE_PAY_API_KEY | API key AbacatePay |
| ABACATE_PAY_WEBHOOK_SECRET | Webhook secret |
| ADMIN_PASSWORD | Senha admin painel |

---

## Health Checks

Todos os servicos Go tem GET /health:

```bash
# Verificacao rapida
bash scripts/verify-deploy.sh
```

---

## Monitoramento

### UptimeRobot (Gratuito)

1. Criar conta em uptimerobot.com
2. Adicionar 2 monitores HTTP:
   - https://fuudelivery-api-8y6l.onrender.com/health (5 min)
   - https://fuudelivery-payment.onrender.com/health (5 min)
3. Configurar alertas via email/Telegram

### Keepalive Script

```powershell
powershell -File scripts/keepalive-payment.ps1
```

---

## Troubleshooting Rapido

| Problema | Solucao |
|----------|--------|
| API retorna 503 | Verificar verify-deploy.sh |
| Payment offline | Logs no Render Dashboard |
| Webhook nao chega | Verificar ABACATE_PAY_WEBHOOK_SECRET |
| CORS error | Verificar ALLOWED_ORIGINS |
| Build APK falha | Verificar eas build:list |
| Render sleep | UptimeRobot ou keepalive |

---

## Custo Estimado

| Servico | Plano | Custo/mes |
|---------|-------|----------|
| fuudelivery-api | Starter | $7 |
| fuudelivery-web | Static | $0 |
| fuudelivery-admin | Static | $0 |
| fuudelivery-payment | Starter | $7 |
| fuudelivery-payment-panel | Static | $0 |
| fuudelivery-redis | Free | $0 |
| **Total** | | **~$14/mes** |

---

## Checklist Pre-Deploy

- [ ] Todos health checks retornam status ok
- [ ] CI pipeline passa
- [ ] GitHub Secrets configurados
- [ ] UptimeRobot monitors criados
- [ ] Credenciais nao estao no repositorio
- [ ] JWT_SECRET e unico e forte
- [ ] ABACATE_PAY_WEBHOOK_SECRET corresponde ao dashboard
- [ ] ALLOWED_ORIGINS inclui todas URLs frontend
- [ ] Docker build funciona
- [ ] APKs gerados e testados

---

*Ultima atualizacao: 2 de agosto de 2026*
