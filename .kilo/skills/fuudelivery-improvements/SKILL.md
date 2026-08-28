# Skill: FuuDelivery — Plano de Correções e Melhorias Restantes

Esta skill documenta os issues identificados na análise de código que **ainda não foram corrigidos** e fornece um plano passo a passo para resolvê-los.

---

## 1. Split Rules — Platform Fee Negativo

**Arquivo:** `Backend/payment_api/app/handlers/webhook.go:470-526`

**Problema:** Quando `deliveryAmount` excede os fundos disponíveis, a lógica de compensação reduz `establishmentAmount` e depois `platformFee`, podendo chegar a valores negativos. Os fundos "perdidos" ficam sem prestação de contas.

**Plano de correção:**

1. Criar uma função `CalculateSplitRules(totalAmount, deliveryAmount float64) (platformFee, establishmentAmount float64, err error)` em um novo arquivo `Backend/payment_api/app/services/split_calculator.go`.
2. Implementar a seguinte lógica:
   - Se `deliveryAmount <= totalAmount`: `platformFee = totalAmount - deliveryAmount` (estabelecimento recebe 0).
   - Se `deliveryAmount > totalAmount`: retornar erro `ErrDeliveryExceedsTotal` — o split deve ser rejeitado ou ajustado manualmente por admin.
3. Remover a compensação "fluida" atual que reduz `platformFee` abaixo de 0.
4. Adicionar teste unitário `split_calculator_test.go` cobrindo:
   - `deliveryAmount < totalAmount`
   - `deliveryAmount == totalAmount`
   - `deliveryAmount > totalAmount` (erro)
5. Atualizar `defaultSplitRules` em `webhook.go` para usar a nova função.

**Passos práticos:**
```bash
# 1. Criar o novo arquivo
touch Backend/payment_api/app/services/split_calculator.go

# 2. Implementar a função
# 3. Adicionar testes
touch Backend/payment_api/app/services/split_calculator_test.go

# 4. Rodar testes
cd Backend/payment_api && go test ./app/services/ -v
```

---

## 2. IdempotencyKey — Não Enviado aos Gateways

**Arquivos:**
- `pkg/gateway/gateway.go:122` (interface)
- `pkg/gateway/pagarme/gateway.go` (adapter)
- `pkg/gateway/asaas/gateway.go` (adapter)
- `pkg/gateway/abacatepay/gateway.go` (adapter)
- `pkg/gateway/mercadopago/gateway.go` (adapter)
- `pkg/gateway/router.go` (router)

**Problema:** `TransactionRequest.IdempotencyKey` existe na interface mas nenhum adapter a envia para os gateways. Combinado com retry-on-POST nos clients HTTP, falhas transitórias de rede criam transações duplicadas.

**Plano de correção:**

1. **Pagar.me:** Adicionar header `X-Idempotency-Key: <uuid>` em `POST /orders` em `pagarme/gateway.go`.
2. **Asaas:** Adicionar header `X-Idempotency-Key` em `POST /payments` em `asaas/gateway.go`.
3. **AbacatePay:** Verificar documentação — se suportar, adicionar header; senão, ignorar.
4. **Mercado Pago:** Adicionar header `X-Idempotency-Key` em `POST /v1/payments`.
5. **Router:** Garantir que `IdempotencyKey` é gerado (UUID v4) no `PaymentRouter.CreateTransaction` antes de chamar o adapter.

**Passos práticos:**
```bash
# 1. Verificar qual client HTTP é usado (provavelmente em pkg/gateway/*/client.go)
# 2. Adicionar header em cada adapter
# 3. Gerar UUID no router se não fornecido
# 4. Testar com retry simulando falha de rede
cd pkg/gateway && go test ./... -v -run Idempotency
```

---

## 3. GitHub Actions — Versões de Actions Inconsistentes

**Arquivos:**
- `.github/workflows/ci.yml` (`checkout@v7`, `setup-go@v7`, `upload-artifact@v7`)
- `.github/workflows/deploy.yml` (várias actions)
- `.github/workflows/release.yml` (`expo-github-action@v9`, `setup-android@v3`)

**Problema:** Várias major versions não correspondem a releases publicadas, podendo quebrar o CI/CD completamente.

**Plano de correção:**

1. Consultar GitHub Marketplace para versões válidas:
   - `actions/checkout@v4`
   - `actions/setup-go@v5`
   - `actions/setup-node@v4`
   - `actions/upload-artifact@v4`
   - `actions/download-artifact@v4`
   - `expo-ai/expo-github-action@v8` (verificar)
   - `actions/setup-java@v4`
   - `action-gh-release@v7` (verificar)

2. Atualizar todos os arquivos YAML para usar as versões corretas.

3. Padronizar: usar `@v4` para actions core, pinar commit SHA para ações de terceiros críticas.

**Passos práticos:**
```bash
# 1. Listar actions usadas
grep -rh "uses:" .github/workflows/ | sort -u

# 2. Verificar versões no GitHub Marketplace
# 3. Atualizar cada arquivo
# 4. Validar com:
bash .github/workflows/ci.yml  # syntax check
# ou usar actionlint
```

---

## 4. CORS — Origens Over-broad em Produção

**Arquivo:** `cmd/fuudelivery/main.go:1484-1490`, `references/URLS.md:62-67`

**Problema:** `isLocalDevOrigin` permite qualquer `localhost` em produção (se `GO_ENV` não estiver como `production`). `*.daytonaproxy01.net` é permitido como wildcard.

**Plano de correção:**

1. Garantir que `isLocalDevOrigin` é **sempre false** quando `GO_ENV=production` (já está implementado, mas adicionar log de warn se `GO_ENV` não está definido).
2. Remover `*.daytonaproxy01.net` dos `defaultOrigins` — deve vir apenas de `ALLOWED_ORIGINS` explícito.
3. Adicionar teste unitário para `isLocalDevOrigin` cobrindo:
   - `GO_ENV=production` + localhost → false
   - `GO_ENV=production` + produção URL → true (se em ALLOWED_ORIGINS)
   - `GO_ENV=development` + localhost → true

**Passos práticos:**
```bash
# 1. Adicionar teste
touch cmd/fuudelivery/cors_test.go

# 2. Implementar teste
# 3. Rodar
cd cmd/fuudelivery && go test -v -run CORS
```

---

## 5. Asaas CaptureTransaction — Body Nil

**Arquivo:** `pkg/gateway/asaas/gateway.go:180`

**Problema:** `g.client.post(path, nil)` envia body nil, impossibilitando captura parcial.

**Plano de correção:**

1. Verificar documentação da API Asaas para captura parcial.
2. Atualizar `CaptureTransaction` para aceitar `amount int64` e enviar no body:
   ```json
   {"value": <amount_em_reais>}
   ```
3. Adicionar teste mockando o client HTTP e verificando que o body contém o valor correto.

**Passos práticos:**
```bash
# 1. Verificar docs Asaas
# 2. Editar pkg/gateway/asaas/gateway.go
# 3. Adicionar teste
cd pkg/gateway/asaas && go test -v -run Capture
```

---

## 6. Redis Eviction Policy — allkeys-lru

**Arquivos:**
- `docker-compose.vps.yml:81`
- `PRODUCTION.md:136-142`
- `render.yaml` (nota sobre Redis externo)

**Problema:** `allkeys-lru` descarta mensagens não-expired da fila de pagamentos sob pressão de memória.

**Plano de correção:**

1. Alterar `docker-compose.vps.yml` para `noeviction`:
   ```yaml
   command: redis-server --maxmemory-policy noeviction
   ```
2. Adicionar alerta/monitoramento de memória Redis no `monitor.yml` ou novo workflow.
3. Documentar no `PRODUCTION.md` que Redis externo deve usar `noeviction` ou `volatile-lru` com TTLs apropriados.
4. Adicionar TTLs nas chaves da fila (`queue:*`) via `EXPIRE` no `pkg/queue/queue.go` (ex: TTL de 24h para mensagens não processadas).

**Passos práticos:**
```bash
# 1. Editar docker-compose.vps.yml
# 2. Editar PRODUCTION.md
# 3. Adicionar TTL no queue.go (Publish com XAdd + EXPIRE)
# 4. Testar localmente
docker compose -f docker-compose.vps.yml up -d redis
redis-cli CONFIG GET maxmemory-policy
```

---

## 7. Frontend — Migração de localStorage para HttpOnly Cookies

**Arquivos:**
- `Frontend/WebRestaurant/src/services/api.js`
- `Frontend/WebAdmin/src/services/api.js`
- Ambos os `AuthContext.js`

**Problema:** JWT e refresh token armazenados em `localStorage`. Qualquer XSS exfiltra todos os tokens.

**Plano de correção (sem breaking changes):**

1. **Backend:** Adicionar endpoint `POST /auth/session` que seta cookies `HttpOnly; Secure; SameSite=Strict` com access token + refresh token.
2. **Backend:** Modificar `ValidateJWT` para aceitar tokens de cookies `access_token` além do header Authorization.
3. **Frontend:** No login, chamar `/auth/session` em vez de receber tokens no body. O backend seta os cookies.
4. **Frontend:** Remover `localStorage.setItem('token', ...)` — ler token dos cookies via `document.cookie` apenas quando necessário para APIs não-padrão.
5. **Frontend:** Implementar `/auth/logout` que limpa os cookies.
6. **Backend:** Adicionar CSRF token em header `X-CSRF-Token` para proteção adicional em mutações.

**Passos práticos:**
```bash
# 1. Backend: criar handler de session
# 2. Backend: atualizar CORS para AllowCredentials + cookies
# 3. Frontend: atualizar api.js para usar credentials: 'include'
# 4. Testar fluxo completo de login/logout
cd Frontend/WebRestaurant && npm test
```

---

## 8. Frontend — Content Security Policy (CSP)

**Arquivos:**
- `cmd/fuudelivery/main.go` (middleware)
- `Frontend/WebRestaurant/index.html`
- `Frontend/WebAdmin/index.html`

**Problema:** Sem CSP headers, XSS pode executar scripts arbitrários.

**Plano de correção:**

1. **Backend Go (para APIs):** Adicionar header `Content-Security-Policy: default-src 'self'` em todas as respostas.
2. **Frontend (Vite):** Adicionar meta tag CSP no `index.html`:
   ```html
   <meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; connect-src 'self' wss: https:;">
   ```
3. Remover `'unsafe-inline'` gradualmente após migrar estilos para Tailwind classes puras.
4. Adicionar `X-Content-Type-Options: nosniff` em todas as respostas.

**Passos práticos:**
```bash
# 1. Adicionar middleware CSP no main.go
# 2. Atualizar index.html dos webs
# 3. Testar no Chrome DevTools → Console → CSP violations
```

---

## 9. Frontend — Proteção contra CSRF

**Arquivos:** Todos os handlers de mutação (POST/PUT/DELETE)

**Problema:** Sem CSRF tokens, qualquer site pode fazer requests autenticados em nome do usuário.

**Plano de correção:**

1. **Backend:** Gerar CSRF token por sessão (cookie `csrf_token`) e validar em todas as mutações.
2. **Frontend:** Ler cookie `csrf_token` e enviar no header `X-CSRF-Token` em todas as requisições mutantes.
3. Usar biblioteca `csrf-csrf` ou implementação simples com `crypto.randomUUID()`.

**Passos práticos:**
```bash
# 1. Backend: middleware CSRF
# 2. Frontend: interceptor axios para adicionar header
# 3. Testar com curl sem header → 403
```

---

## 10. Backend — Testes de Integração para Novos Fixes

**Problema:** Os fixes aplicados (ownership checks, race conditions, etc.) não têm cobertura de teste automatizado.

**Plano de correção:**

1. **Testes de IDOR:** Para cada handler com ownership check, adicionar teste que tenta acessar recurso de outro estabelecimento → espera 403.
2. **Testes de Race Condition:** Para `ApplyCoupon` e `EarnPoints`, adicionar teste com 10 goroutines concorrentes → espera exatamente 1 sucesso (coupon) ou crédito único (loyalty).
3. **Testes de Shutdown:** Para `pkg/queue`, adicionar teste que chama `Close()` enquanto consumers estão ativos → espera shutdown limpo sem goroutine leak.

**Passos práticos:**
```bash
# 1. Adicionar testes em cada módulo
# Exemplo: Backend/orders_api/app/handlers/products_test.go
cd Backend/orders_api && go test ./app/handlers/ -v -run TestOwnership

# 2. Testes de race condition
cd Backend/orders_api && go test -race ./app/handlers/ -run TestCouponRace
```

---

## 11. Backend — Remover HTTP Call Residual

**Arquivo:** `Backend/orders_api/app/handlers/orders.go`

**Problema:** `checkEstablishmentOpen` foi substituído por query direta, mas o código antigo pode ter resíduos ou imports não utilizados (`net/http`, `io`, `net/url` em `orders.go`).

**Plano de correção:**

1. Verificar se há imports não utilizados em `orders.go` após a remoção do HTTP call.
2. Remover `clientHTTPCheck` (provavelmente `http.Client` global) se não for usado em outro lugar.
3. Rodar `go vet ./Backend/orders_api` para detectar imports mortos.

**Passos práticos:**
```bash
cd Backend/orders_api
go vet ./app/handlers/
# Remover imports não utilizados manualmente
```

---

## 12. Seed Script — Remover Impressão de Tokens

**Arquivo:** `scripts/seed-test-data.sh:242-262`

**Problema:** Mesmo após corrigir senhas, o script ainda imprime tokens JWT no stdout. Se executado via CI, os tokens vazam para logs/artifatos.

**Plano de correção:**

1. Remover ou comentar o bloco de impressão de tokens (linhas 242-262).
2. Adicionar flag `--print-tokens` opcional (opt-in) para debugging.
3. Limpar variáveis `ADMIN_TOKEN`, `RESTAURANT_TOKEN`, etc. no final do script (`unset`).

**Passos práticos:**
```bash
# 1. Editar scripts/seed-test-data.sh
# 2. Comentar bloco de tokens ou adicionar condicional
# 3. Testar
bash scripts/seed-test-data.sh FUU_BOOTSTRAP_SECRET=test
```

---

## 13. Frontend — Hardcoded Production API URLs

**Arquivos:**
- `Frontend/AppComida/config/api.ts`
- `Frontend/AppEntrega/config/api.ts`
- `Frontend/AppRestaurante/config/api.ts`

**Problema:** URLs de produção hardcoded nos apps mobile. Mudança requer atualizar 3 arquivos.

**Plano de correção:**

1. Centralizar URL base em uma variável de ambiente `EXPO_PUBLIC_API_URL`.
2. Remover fallback hardcoded dos `config/api.ts`.
3. Atualizar `app.json` de cada app para incluir `extra.apiUrl` com a URL de produção.
4. Documentar que `EXPO_PUBLIC_API_URL` deve ser usada em builds de produção.

**Passos práticos:**
```bash
# 1. Editar config/api.ts para usar Constants.expoConfig?.extra?.apiUrl
# 2. Atualizar app.json de cada app
# 3. Rebuild com EAS
```

---

## 14. Backend — Validar Amount em ChargeCard (Reforço)

**Arquivo:** `Backend/payment_api/app/handlers/card.go`

**Problema:** Mesmo após o fix inicial, `ChargeCard` ainda usa `AbacatePay` diretamente, ignorando o `PaymentRouter` e o circuit breaker.

**Plano de correção (fase 2):**

1. Migrar `ChargeCard` para usar `gateway.Router` ao invés de `services.NewAbacatePayClient()`.
2. O router respeita fallback chain + circuit breaker.
3. Adicionar teste de circuit breaker: simular 5 falhas do AbacatePay → próximo request deve cair para Asaas.

**Passos práticos:**
```bash
# 1. Refatorar ChargeCard para usar router
# 2. Adicionar teste
cd Backend/payment_api && go test -v -run TestChargeCardCircuitBreaker
```

---

## 15. Backend — Adicionar Unique Constraints no Banco

**Problema:** Race conditions de coupon e loyalty foram mitigadas em código, mas falta constraint única no banco como defense-in-depth.

**Plano de correção:**

1. **CouponUsage:** Adicionar unique constraint `UNIQUE(coupon_id, user_phone, order_id)` em `sql/` (nova migration ou atualizar `sql/01_dominio_pedidos.sql`).
2. **LoyaltyTransaction:** Adicionar unique constraint `UNIQUE(order_id, type, user_phone)` onde `type='earn'`.
3. Aplicar migration em produção.

**Passos práticos:**
```bash
# 1. Criar migration
sql/17_unique_constraints.sql

# 2. Aplicar
bash sql/run_all.sh

# 3. Verificar
psql $DB_CONNECTION_STRING -c "\d coupon_usages"
```

---

## Como Usar Esta Skill

Quando o usuário pedir para corrigir um item desta lista:

1. Ler o arquivo `SKILL.md` para contexto.
2. Executar o "Plano de correção" do item específico.
3. Aplicar as alterações nos arquivos indicados.
4. Rodar os comandos de validação sugeridos.
5. Reportar o resultado.

Para priorização, seguir a ordem:
1. Críticos de segurança (item 1, 2, 3)
2. Bugs de negócio (item 5, 6)
3. Infraestrutura/CI (item 3, 6)
4. Frontend (item 7, 8, 9)
5. Testes e melhorias estruturais (item 10, 14, 15)
