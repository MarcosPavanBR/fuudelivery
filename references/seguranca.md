# Segurança — FuuDelivery

## 🔴 Prioridade 0 — Exposição de Credenciais

O arquivo `.fuudelivery-config/CREDENTIALS.md` foi commitado no repositório público junto com `Frontend/WebRestaurant/.env`. Isso expõe:

| Credencial | Serviço | Risco |
|---|---|---|
| MongoDB Atlas password | Banco de dados | Acesso total ao banco de produção |
| Redis password | Fila/pubsub | Manipulação de filas e cache |
| Supabase password | PostgreSQL | Acesso total ao banco relacional |
| AbacatePay API Key | Gateway de pagamento | Transações financeiras |
| AbacatePay Webhook Secret | Webhooks | Falsificação de webhooks |
| JWT Secret | Autenticação | Forjar tokens de qualquer usuário |
| Render API Token | Deploy | Deploy/destroy de serviços |
| Admin password (`123456`) | Login admin | Conta admin comprometida |

### Guia Completo de Rotação de Credenciais

> **IMPORTANTE:** Execute TODOS os passos na ordem abaixo. Não pule nenhum.
> Após rotacionar, atualize as env vars no Render ANTES de fazer push das novas credenciais.

#### Passo 1 — MongoDB Atlas

```bash
# 1. Acesse https://cloud.mongodb.com
# 2. Database Access → Usuário pavanbrtl050_db_user → Edit
# 3. Regenerate Password → copie a nova senha
# 4. Atualize MONGODB_URI no Render (Payment Service e API)
```

Nova connection string:
```
mongodb+srv://pavanbrtl050_db_user:NOVA_SENHA@fuudelivery.hj0pytw.mongodb.net/fuudelivery?retryWrites=true&w=majority&appName=fuudelivery
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
postgresql://postgres.prpfuoqhazfynpsfsrpb:NOVA_SENHA@aws-1-us-east-2.pooler.supabase.com:6543/postgres
```

#### Passo 3 — Redis (Render)

```bash
# 1. Dashboard Render → fuudelivery-redis → Info
# 2. Copie a nova Connection String
# 3. Atualize REDIS_URL no Render (API + Payment)
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

### Limpar Histórico do Git (BFG Repo-Cleaner)

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

- [ ] CREDENTIALS.md removido do histórico do git (BFG)
- [ ] .env removido do histórico do git (BFG)
- [ ] Todas as credenciais rotacionadas (Atlas, Supabase, Redis, AbacatePay, JWT, Render)
- [ ] Senha do admin alterada para forte (16+ caracteres)
- [ ] Rate limiting em login, registro e pagamento (PENDENTE — ver task #4)
- [ ] govulncheck e npm audit no CI (✅ Implementado)
- [ ] Repo verified como público (ou tornar privado)
- [ ] Nenhum `.env` com credenciais de produção commitado

---

## P1 — Rate Limiting (PENDENTE)

### Rotas que precisam de rate limiting

| Rota | Método | Limite sugerido | Justificativa |
|---|---|---|---|
| `/auth/login` | POST | 5 req/min por IP | Brute force |
| `/auth/register` | POST | 3 req/min por IP | Spam de contas |
| `/payments/create` | POST | 10 req/min por user | Fraude |
| `/payments/webhook` | POST | 100 req/min por IP | AbacatePay retry |
| `/wallets/*/credit` | POST | 20 req/min por user | Manipulação de saldo |
| `/wallets/*/debit` | POST | 20 req/min por user | Manipulação de saldo |

### Implementação sugerida (Go + Fiber)

```go
// middleware/ratelimit.go
package middleware

import (
    "time"
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/limiter"
)

// LoginRateLimit limita tentativas de login por IP
func LoginRateLimit() fiber.Handler {
    return limiter.New(limiter.Config{
        Max:        5,
        Expiration: 1 * time.Minute,
        KeyGenerator: func(c *fiber.Ctx) string {
            return c.IP()
        },
        LimitReached: func(c *fiber.Ctx) error {
            return c.Status(429).JSON(fiber.Map{
                "error": "Muitas tentativas de login. Tente novamente em 1 minuto.",
            })
        },
    })
}

// PaymentRateLimit limita operações de pagamento por usuário
func PaymentRateLimit() fiber.Handler {
    return limiter.New(limiter.Config{
        Max:        10,
        Expiration: 1 * time.Minute,
        KeyGenerator: func(c *fiber.Ctx) string {
            userID, _ := c.Locals("user_id").(string)
            return userID
        },
        LimitReached: func(c *fiber.Ctx) error {
            return c.Status(429).JSON(fiber.Map{
                "error": "Limite de operações excedido.",
            })
        },
    })
}
```

### Dependência necessária

```bash
go get github.com/gofiber/fiber/v2/middleware/limiter
```

---

## P1 — Scanning de Vulnerabilidades no CI (✅ Implementado)

### Go: govulncheck (✅ Matrix strategy)

O CI agora usa matrix strategy para rodar govulncheck em paralelo para todos os 7 módulos:
- cmd/fuudelivery
- Backend/Payment
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

*Última atualização: 2026-07-26*
