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

### Ação imediata (passo a passo)

**NÃO fazer push de credenciais novas antes de rotacionar as antigas.**

#### 1. MongoDB Atlas (banco de pagamento)
1. Acesse [cloud.mongodb.com](https://cloud.mongodb.com)
2. Database Access → utilisateur `fuudelivery` → Edit
3. Regenerate password → copiar nova senha
4. Atualizar variável `MONGODB_URI` no Render (Payment Service)

#### 2. Supabase (PostgreSQL)
1. Acesse [supabase.com/dashboard](https://supabase.com/dashboard)
2. Project Settings → Database → Reset password (ou Settings → API → regenerate)
3. Atualizar variável `SUPABASE_URL` e `SUPABASE_KEY` no Render (API Service)

#### 3. Redis (Render Managed Redis)
1. Dashboard Render → `fuudelivery-redis` → Info
2. Cópia da Connection String (muda a cada reconnect, mas a password é fixa)
3. Se o Redis foi provisionado via Render, a password não pode ser rotacionada diretamente — recrie o serviço se necessário

#### 4. AbacatePay
1. Acesse painel do AbacatePay
2. API Keys → Revogar chave antiga → Gerar nova
3. Atualizar `ABACATEPAY_API_KEY` no Render (Payment Service)
4. Atualizar webhook secret e variável `ABACATEPAY_WEBHOOK_SECRET`

#### 5. JWT Secret
1. Gerar novo secret: `openssl rand -hex 32`
2. Atualizar variável `JWT_SECRET` no Render (API + Payment)
3. **ATENÇÃO**: todos os tokens JWT existentes serão invalidados — usuários precisarão fazer login novamente

#### 6. Admin Bootstrap Secret
1. Gerar novo: `openssl rand -hex 16`
2. Atualizar variável `BOOTSTRAP_SECRET` no Render (Payment Service)
3. O código em `main.go` já não reseta mais a senha em cada restart

#### 7. Render API Token
1. Dashboard Render → Account Settings → API Keys
2. Revogar token antigo → Criar novo
3. Atualizar secret `RENDER_API_KEY` no GitHub (`.github/workflows/deploy.yml`)

#### 8. Senha do Admin
1. Acesse o PaymentPanel ou WebAdmin
2. Faça login com credenciais atuais
3. Altere a senha para uma forte (16+ caracteres, mistura de tipos)
4. Atualizar variável `ADMIN_PASSWORD` no Render

### Limpar histórico do git

Mesmo após remover `CREDENTIALS.md` do tracking, o conteúdo permanece no histórico. Usar BFG Repo-Cleaner:

```bash
# 1. Clonar o repo (BFG precisa de clone limpo)
git clone --mirror https://github.com/MarcosPavanBR/fuudelivery.git

# 2. Rodar BFG para remover o arquivo
bfg --delete-files CREDENTIALS.md

# 3. Limpar reflog e fazer push forçado
cd fuudelivery.git
git reflog expire --expire=now --all
git gc --prune=now --aggressive
git push --force
```

**IMPORTANTE**: Após o push forçado, todos os clones locais precisam ser re-clonados ou fazer `git fetch --all && git reset --hard origin/master`.

### Verificar se o repo é público

O repositório `github.com/MarcosPavanBR/fuudelivery` é público. Se containha credenciais de produção, qualquer pessoa pode ter clonado e acessado os dados. Considere:

1. Criar um novo repo privado com o mesmo código (limpo)
2. OU manter o repo público mas com ZERO credenciais em texto plano

---

## P1 — Rate Limiting

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

## P1 — Scanning de Vulnerabilidades no CI

### Go: govulncheck

Adicionar ao `.github/workflows/ci.yml`:

```yaml
- name: Install govulncheck
  run: go install golang.org/x/vuln/cmd/govulncheck@latest

- name: Run govulncheck
  run: govulncheck ./...
  working-directory: cmd/fuudelivery
```

### JavaScript: npm audit

```yaml
- name: Run npm audit (WebRestaurant)
  run: npm audit --audit-level=moderate
  working-directory: Frontend/WebRestaurant

- name: Run npm audit (WebAdmin)
  run: npm audit --audit-level=moderate
  working-directory: Frontend/WebAdmin
```

---

## Checklist de segurança para produção

- [ ] CREDENTIALS.md removido do histórico do git (BFG)
- [ ] Todas as credenciais rotacionadas (Atlas, Supabase, AbacatePay, JWT, Render)
- [ ] Senha do admin alterada para forte
- [ ] Rate limiting em login, registro e pagamento
- [ ] govulncheck e npm audit no CI
- [ ] Repo verified como público (ou tornar privado)
- [ ] Nenhum `.env` com credenciais de produção commitado
