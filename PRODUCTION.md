# Guia de Deploy em Producao - FuuDelivery

> **📌 Referência central de URLs:** ver `references/URLS.md` — mapa de todos os
> serviços, endpoints de health, CORS e histórico de correções de URLs.

## Status Atual (29 Agosto 2026)

| Servico | URL | Status |
|---------|-----|--------|
| API (Monolito) | fuudelivery-api-8y6l.onrender.com | Online |
| WebRestaurant | fuudelivery-web.onrender.com | Online |
| WebAdmin | fuudelivery-admin-lv7f.onrender.com | Online |
| PostgreSQL (Supabase) | Cloud | Connected |
| Redis | **Externo** (`*.db.redis.io` — não é serviço Render) | Connected |

> Os serviços isolados `fuudelivery-payment` e `fuudelivery-payment-panel` foram
> **removidos (2026-08)**: todas as rotas de pagamento (PIX, cartão, carteira,
> webhook, split) vivem no monolito `fuudelivery-api`. NÃO provisione esses
> serviços de novo — qualquer doc que os mencione está desatualizada.

### Segurança (Atualizado 29/08/2026)

| Medida | Status |
|--------|--------|
| Sessão HttpOnly (cookies) | ✅ Frontends usam `POST /auth/session` com cookies HttpOnly + Secure + SameSite=None |
| CSRF protection | ✅ `X-CSRF-Token` header + cookie, retry automático em 403 |
| localStorage tokens | ❌ Removido — nenhum frontend grava token em localStorage |
| ChargeCard via gateway router | ✅ Fallback Pagar.me → Asaas → AbacatePay |
| IDOR no WebSocket | ✅ `wsCanAccessOrder()` antes de HandleChatWebSocket |
| Rate limiting | ✅ Login 10/min, refresh 30/min, mutações protegidas |
| CORS | ✅ `Allow-Credentials: true`, 2 origens produção |
| CSP headers | ✅ `Content-Security-Policy` + `X-Content-Type-Options: nosniff` |

---

## Deploy Automatico

O deploy e automatizado via GitHub Actions. Push para master dispara:

1. CI workflow (go build, go vet, go test, gofmt, govulncheck, npm audit, testes de integração com testcontainers)
2. Deploy workflow (apenas se CI passar)
3. Render hot-reload automatico

### GitHub Secrets Necessarios

| Secret | Descricao |
|--------|-----------|
| RENDER_API_KEY | API key do Render |
| RENDER_SERVICE_ID_API | ID servico API |
| RENDER_SERVICE_ID_WEB | ID WebRestaurant |
| RENDER_SERVICE_ID_ADMIN | ID WebAdmin |
| EXPO_TOKEN | Token Expo para APKs |

---

## Variaveis de Ambiente

### Monolito (fuudelivery-api) — unico serviço backend

| Variavel | Descricao |
|----------|-----------|
| PORT | 3000 |
| GO_ENV | production |
| JWT_SECRET | Chave JWT (secret) |
| ADMIN_BOOTSTRAP_SECRET | Bootstrap único do primeiro admin via `POST /admin/bootstrap` (secret). Remover após uso |
| ABACATE_PAY_WEBHOOK_SECRET | Webhook secret (secret) |
| REDIS_URL | Redis EXTERNO gerenciado (Redis Cloud — valor real no dashboard, NÃO é auto-linked) |
| DB_CONNECTION_STRING | PostgreSQL Supabase via pooler :6543 (secret) |
| SUPABASE_URL | API do Supabase p/ Storage/REST upload de imagens (secret) |
| SUPABASE_SERVICE_ROLE_KEY | Service role do Supabase (secret). Sem elas o upload responde 503 |
| MONGO_URI | MongoDB Atlas — dual-write legado; opcional após aposentadoria do Atlas (secret) |
| MONGO_DATABASE | fuudelivery |
| PAYMENT_MONGO_DATABASE | fuudelivery_payments |
| APP_URL | https://fuudelivery-web.onrender.com |
| API_BASE_URL | https://fuudelivery-api-8y6l.onrender.com |
| ABACATE_PAY_API_KEY | API key AbacatePay (secret) |
| ALLOWED_ORIGINS | CORS origins (domínios web + admin) |
| METRICS_TOKEN | Protege `GET /metrics` (Bearer ou ?token=). Vazio = público — evite em produção |
| OSRM_BASE_URL | Servidor OSRM próprio p/ cálculo de rotas. Vazio = demo público (só dev) |

### Como configurar no Render Dashboard

1. Acesse **Render → fuudelivery-api → Environment**
2. Adicione cada chave e o valor correspondente:
   - `METRICS_TOKEN`: gere com `openssl rand -hex 32`
   - `OSRM_BASE_URL`: URL da sua instância OSRM (opcional até ter volume)
3. Clique em **Save & Deploy** (variáveis só aplicam no próximo deploy)
4. Se usar o blueprint (`render.yaml`), as chaves já aparecem como `sync: false` —
   preencha os valores no dashboard; eles nunca são commitados no repo

> **Refresh tokens:** a tabela `refresh_tokens` (Postgres) guarda as sessões de
> 30 dias. O monolito limpa tokens expirados/revogados a cada 24h
> (`startRefreshTokenCleanup`). Se trocar `JWT_SECRET`, todas as sessões caem —
> os usuários precisarão logar de novo (esperado).

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
2. Adicionar monitores HTTP (5 min):
   - https://fuudelivery-api-8y6l.onrender.com/health
   - https://fuudelivery-web.onrender.com
   - https://fuudelivery-admin-lv7f.onrender.com
3. Configurar alertas via email/Telegram

> O job "Monitor Production" do GitHub Actions (`monitor.yml`) também verifica
> os health checks — confira se os alertas dele estão ativos.

---

## Troubleshooting Rapido

| Problema | Solucao |
|----------|--------|
| API retorna 503 | Verificar verify-deploy.sh (Postgres/Mongo caídos) |
| Webhook nao chega | Verificar ABACATE_PAY_WEBHOOK_SECRET |
| CORS error | Verificar ALLOWED_ORIGINS |
| Build APK falha | Verificar eas build:list |
| Render sleep | UptimeRobot ou monitor.yml |
| Sessões caindo | Verificar JWT_SECRET (mudou? tokens antigos invalidam) |

---

## Custo Estimado

| Servico | Plano | Custo/mes |
|---------|-------|----------|
| fuudelivery-api | Starter | $7 |
| fuudelivery-web | Static | $0 |
| fuudelivery-admin | Static | $0 |
| Redis externo | conforme provedor | ver conta do provedor |
| **Total Render** | | **~$7/mes** |

> ⚠️ **Redis EXTERNO e fila de pagamentos:** o `REDIS_URL` aponta para um provedor
> externo (não é serviço Render). Verifique na conta do provedor qual plano está
> ativo — planos com eviction (`allkeys-lru`) ou sem persistência podem perder
> mensagens da fila de pagamentos em pico de tráfego. Sem Redis, o monolito usa
> fallback in-memory (eventos não sobrevivem a restart). O render.yaml NÃO
> declara mais um serviço Redis — se migrar para o Redis gerenciado do Render,
> atualize só o valor de REDIS_URL no dashboard.

---

## Checklist Pre-Deploy

- [x] Todos health checks retornam status ok (verificado 29/08)
- [x] CI pipeline passa (go build + go vet + 54+ testes gateway)
- [x] Credenciais nao estao no repositorio (.gitignore + HEAD limpo)
- [x] JWT_SECRET e unico e forte
- [x] ALLOWED_ORIGINS inclui todas URLs frontend (2 origens produção)
- [x] Sessão HttpOnly (cookies, não localStorage)
- [x] CSRF protection (token + cookie)
- [x] ChargeCard usa gateway router com fallback
- [ ] API keys dos gateways configuradas (Pagar.me, Asaas, Mercado Pago)
- [ ] Webhooks registrados nos gateways
- [ ] UptimeRobot monitors criados
- [ ] GitHub Secrets configurados (RENDER_API_KEY, etc.)
- [ ] APKs gerados e testados
- [ ] Plano do Redis revisado (ver risco acima) se houver volume de pagamentos

### O que falta para 100% produção (só você pode fazer)

| # | Ação | Onde |
|---|------|------|
| 1 | Configurar API keys dos gateways | Render Dashboard → Environment |
| 2 | Registrar webhooks nos gateways | Painel de cada gateway |
| 3 | Rodar migrations 14-18 no Supabase | `psql` ou Supabase Dashboard |
| 4 | Confirmar UptimeRobot | uptimerobot.com |
| 5 | Gerar release v1.0.0 com APKs | `eas build` |

---

*Ultima atualizacao: 29 de agosto de 2026*
