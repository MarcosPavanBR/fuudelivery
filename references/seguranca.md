# Segurança — FuuDelivery


> ⚠️ **`Backend/payment_api (monolith)` foi arquivado e removido do repositório.** Todo o código
> de pagamento ativo vive em `payment_api` (embutido no monolito `cmd/fuudelivery`).
> As menções a `Backend/payment_api (monolith)` neste documento são **históricas** — não edite,
> não busque e não rode comandos apontando para esse diretório.
> **Última atualização:** 2026-09-25 (auditoria 360°: gitleaks no histórico completo, 662 commits)

## 🔴 Prioridade 0 — Exposição de Credenciais

O repositório é **público** (confirmado em 2026-09-25). `.fuudelivery-config/CREDENTIALS.md`
(commit `abdcedd`) e `.fuudelivery-config/DOCUMENTATION.md`/`.html` (commits `4af3087`,
`9a3fc3b`, `87ea64f`, `085480d`) foram removidos do tracking, **mas o conteúdo segue no
histórico do git**. A árvore atual está limpa (gitleaks `dir`: só placeholders).

| Credencial exposta no histórico | Onde | Status |
|---|---|---|
| MongoDB Atlas (string de conexão com senha) | CREDENTIALS.md, DOCUMENTATION.md | ⚠️ Rotação não confirmada |
| Redis (string de conexão com senha; provedor externo, não é o Render) | CREDENTIALS.md | ⚠️ Rotação não confirmada |
| Render API Token (`rnd_…`, 3 ocorrências) | CREDENTIALS.md | ⚠️ Rotação não confirmada — dá leitura de TODAS as env vars de produção e deploy |
| AbacatePay API Key + Webhook Secret | CREDENTIALS.md, DOCUMENTATION.md | ⚠️ Rotação não confirmada |
| Senha do Supabase (PostgreSQL) | (registro anterior deste documento) | ⚠️ Rotação não confirmada |
| JWT_SECRET fallback no Backend/Payment arquivado | `Backend/Payment/config/config.go` (histórico) | ⚠️ Confirmar que produção não usa esse valor |

**O que resolve é revogar.** Revogar e reemitir TUDO acima, nos painéis de cada serviço
(guia abaixo). Verificar também o repositório `fuudelivery-backend`, que é público e não
foi auditado.

**Decisão (2026-09-25): não reescrever o histórico e não tornar o repositório privado.**
- Depois da rotação, os valores no histórico não abrem mais nada.
- Reescrever o histórico (BFG + force-push no `master`) quebra todo clone existente e
  não apaga as cópias que já saíram (forks, clones, caches, commits acessíveis por SHA).
- Tornar privado no plano gratuito limita o GitHub Actions a 2.000 min/mês; este CI roda
  30 jobs + 3 builds de APK por push e estouraria a cota.
A seção "Limpar Histórico do Git" abaixo fica só como referência, caso a decisão mude.

### Guia Completo de Rotação de Credenciais

> **IMPORTANTE:** Execute TODOS os passos na ordem abaixo. Não pule nenhum.
> Após rotacionar, atualize as env vars no Render ANTES de fazer push das novas credenciais.

#### Passo 1 — MongoDB Atlas

```bash
# 1. Acesse https://cloud.mongodb.com
# 2. Database Access → Usuário <USUARIO> → Edit
# 3. Regenerate Password → copie a nova senha
# 4. Atualize MONGODB_URI no Render (Payment Service e API)
```

Nova connection string:
```
mongodb+srv://<USUARIO>:<NOVA_SENHA>@<CLUSTER>.mongodb.net/fuudelivery?retryWrites=true&w=majority&appName=<APP>
```

#### Passo 2 — Supabase (PostgreSQL)

```bash
# 1. Acesse https://supabase.com/dashboard
# 2. Project Settings → Database → Reset password
# 3. Copie a nova senha
# 4. Atualize DB_CONNECTION_STRING no Render (API Service)
```

Nova connection string:
```
postgresql://<USUARIO>:<NOVA_SENHA>@<HOST>.pooler.supabase.com:6543/postgres
```

#### Passo 3 — Redis (provedor EXTERNO, não é serviço Render)

```bash
# 1. Acesse o painel do provedor externo (*.db.redis.io) → Connection String
# 2. Rotacione a senha / gere nova connection string lá
# 3. Atualize REDIS_URL no dashboard Render → fuudelivery-api → Environment
#
# NOTA: não existe serviço "fuudelivery-redis" no Render — o bloco foi removido
# do render.yaml (auditado 2026-08-23). O Redis é externo ao Render.
```

#### Passo 4 — AbacatePay

```bash
# 1. Acesse painel do AbacatePay
# 2. API Keys → Revogar chave antiga → Gerar nova
# 3. Copie a nova API Key
# 4. Atualize ABACATE_PAY_API_KEY no Render (API + Payment)
# 5. Atualize ABACATE_PAY_WEBHOOK_SECRET no Render (API)
```

#### Passo 5 — JWT Secret

```bash
# 1. Gere um novo secret (64 caracteres):
openssl rand -hex 32

# 2. Atualize JWT_SECRET no Render (API + Payment)
# ATENÇÃO: todos os tokens JWT existentes serão invalidados
# Usuários precisarão fazer login novamente
```

#### Passo 6 — Admin Bootstrap Secret

```bash
# 1. Gere um novo secret:
openssl rand -hex 16

# 2. Atualize BOOTSTRAP_SECRET no Render (Payment Service)
```

#### Passo 7 — Render API Token

```bash
# 1. Dashboard Render → Account Settings → API Keys
# 2. Revogar token antigo → Criar novo
# 3. Atualize RENDER_API_KEY no GitHub (.github/workflows/deploy.yml)
```

#### Passo 8 — Senha do Admin

```bash
# 1. Acesse o PaymentPanel ou WebAdmin
# 2. Faça login com credenciais atuais
# 3. Altere a senha para uma forte (16+ caracteres)
# 4. Atualize ADMIN_PASSWORD no Render (Payment Service)
```

### Limpar Histórico do Git (BFG Repo-Cleaner) — opcional, não adotado (ver decisão acima)

Mesmo após remover `CREDENTIALS.md` do tracking, o conteúdo permanece no histórico.

```bash
# 1. Clonar o repo (BFG precisa de clone limpo)
git clone --mirror https://github.com/MarcosPavanBR/fuudelivery.git

# 2. Rodar BFG para remover os arquivos
bfg --delete-files CREDENTIALS.md
bfg --delete-files .env

# 3. Limpar reflog e fazer push forçado
cd fuudelivery.git
git reflog expire --expire=now --all
git gc --prune=now --aggressive
git push --force
```

**IMPORTANTE:** Após o push forçado, todos os clones locais precisam ser re-clonados:
```bash
git fetch --all && git reset --hard origin/master
```

### Verificar se o Repo é Público

O repositório `github.com/MarcosPavanBR/fuudelivery` é público. Considere:

1. **Criar um novo repo privado** com o mesmo código (limpo)
2. **OU** manter público mas com ZERO credenciais em texto plano

---

## Checklist de Segurança para Produção

- [x] Histórico: decisão de NÃO reescrever (2026-09-25) — revogar é o que resolve
- [x] `.env` do histórico sem segredo (verificado 2026-09-25: `Frontend/WebRestaurant/.env` só tinha URLs)
- [ ] Todas as credenciais rotacionadas (Atlas, Supabase, Redis, AbacatePay, JWT, Render)
- [ ] Senha do admin alterada para forte (16+ caracteres)
- [x] Rate limiting em login, registro e pagamento (✅ Implementado — ver seção abaixo)
- [x] govulncheck (inclui `pkg/gateway` e `pkg/secretbox`) e npm audit bloqueando a partir de **alto** no CI
- [x] Visibilidade verificada: **público** (2026-09-25) — decisão: continua público (ver acima)
- [x] Nenhum `.env` com credenciais de produção na árvore atual (gitleaks `dir`, 2026-09-25)

---

## P1 — Rate Limiting (✅ Implementado)

### Implementação atual

Rate limiting está ativo em dois locais:

1. **Monolito** (`cmd/fuudelivery/main.go`): `rateLimitMiddleware` aplicado em:
   - `/users/register` — 3 req/min por IP
   - `/users/login` — 5 req/min por IP
   - `/admin/bootstrap` — 3 req/min por IP
   - `/payments/webhook` — 100 req/min por IP

2. **Payment Service** (`Backend/payment_api (monolith)/middleware/ratelimit.go`): Token bucket para:
   - Login: 5 req/min por IP
   - Pagamento: 10 req/min por user

### Rotas com rate limiting

| Rota | Método | Limite | Local |
|---|---|---|---|
| `/users/login` | POST | 5 req/min por IP | Monolito |
| `/users/register` | POST | 3 req/min por IP | Monolito |
| `/admin/bootstrap` | POST | 3 req/min por IP | Monolito |
| `/payments/webhook` | POST | 100 req/min por IP | Monolito |
| `/payments/create` | POST | 10 req/min por user | Payment Service |

---

## P1 — Scanning de Vulnerabilidades no CI (✅ Implementado)

### Go: govulncheck (✅ Matrix strategy)

O CI agora usa matrix strategy para rodar govulncheck em paralelo para todos os 7 módulos:
- cmd/fuudelivery
- Backend/payment_api (monolith)
- Backend/auth_api
- Backend/payment_api
- Backend/orders_api
- Backend/delivery_api
- Backend/chat_api

### JavaScript: npm audit (✅ Implementado)

```yaml
- name: Run npm audit
  run: npm audit --audit-level=moderate
```

Roda para: WebRestaurant, WebAdmin, PaymentPanel

---

*Última atualização: 2026-07-31*
