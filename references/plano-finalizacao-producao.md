# FuuDelivery — Análise Completa e Plano de Finalização para Produção


> ⚠️ **`Backend/Payment` foi arquivado e removido do repositório.** Todo o código
> de pagamento ativo vive em `payment_api` (embutido no monolito `cmd/fuudelivery`).
> As menções a `Backend/Payment` neste documento são **históricas** — não edite,
> não busque e não rode comandos apontando para esse diretório.
> Auditoria feita em 10/08/2026 sobre o estado real do repositório (`master` local em `cc2132c`,
> 213 commits). Nada aqui é suposição — cada item foi verificado no código, no git ou no CI.

---

## 1. Estado atual (verificado)

### 1.1 Git e CI — o problema nº 1 de hoje

| Fato verificado | Detalhe |
|---|---|
| **Local está 5 commits à frente do remoto** | `git rev-list --left-right --count origin/master...HEAD` → `0 5`. Os commits `249a8fb`, `f492a3d`, `e8b2975`, `c343a2c`, `cc2132c` **nunca foram pushados**. |
| **Último run do CI Gate FALHOU em 100% dos jobs** | Run `30833707526` (commit `db29983`, o último pushado). Todos os 19 jobs falharam. |
| Causas identificadas dos failures | (a) `testcontainers-go v0.43.0` exige go ≥ 1.25, CI roda Go 1.23 → `go mod tidy` quebra em **todos** os módulos Go (corrigido **localmente** nos pins do `cc2132c`, validado com build local Go 1.23); (b) gofmt — **hoje limpo** localmente (`gofmt -l -s Backend/ cmd/ pkg/` → vazio); (c) Frontend/NPM-audit falhavam na era webpack/react-scripts — a migração Vite (local) ainda **não foi validada pelo CI**. |
| **Conclusão dura** | O CI **nunca passou verde** neste repositório. "Zero falhas" começa com um push + CI verde como portão obrigatório. |

### 1.2 Backend (Go, 7 módulos + 2 pacotes compartilhados)

- **Monolito `cmd/fuudelivery`** (o que é deployado como `fuudelivery-api`): 100+ rotas em 1 binário, absorve auth/orders/delivery/payment/chat. Auth JWT por env (`JWT_SECRET` obrigatório, checado no startup), CORS com allowlist (`ALLOWED_ORIGINS`), rate limit em login/register/bootstrap/webhook. Health `/health` com ping a Postgres/Mongo (HTTP 503 só quando DBs críticos caem; Redis degrada com 200). Tem `/metrics` e `/search`.
- **Fila `pkg/queue`**: Redis Streams com consumer group, retry (3x), **DLQ**, reclaim pós-crash e fallback em memória. ✓
- **WebSocket fila→cliente**: a ponte acabou de ser ligada (`c343a2c`) — `order_updates`/`delivery_updates`/`payment_updates` agora notificam o cliente conectado em `/ws/:id` com retry/DLQ. **Mas ninguém publica em `order_updates`/`payment_updates` ainda** (o webhook publica em `payments`/`orders`) → a ponte está pronta, faltam os publishers.
- **Webhook AbacatePay**: re-verifica o status do charge na API da AbacatePay (não confia no body) ✓; **não valida HMAC** `x-abacatepay-signature` (defesa em profundidade ausente).
- **Backend/Payment** (deploy separado `fuudelivery-payment`): risk engine (score 0-100, ≥40 aprovação manual), approvals, chargebacks, wallets — 7 arquivos de teste. Rota `GET /` de índice já existe.
- **Microserviços** (`auth_api`, `orders_api`, `delivery_api`, `payment_api`, `chat_api`): são a biblioteca do monolito (importados via `replace` no workspace). Ainda têm `main.go` próprios que **ninguém deploya** (render.yaml só sobe monólito + Payment). Código vivo, caminho de deploy morto.
- **Inconsistência de module path**: o monólito importa os microserviços como `github.com/carloshomar/vercardapio/...` (fork original), enquanto `pkg/*` usa `github.com/carloshomar/fuudelivery/...`. Funciona por causa dos `replace`, mas é resíduo de rebranding.

### 1.3 Frontends web

- **WebRestaurant / WebAdmin**: migrados para Vite + React 19 + Tailwind 4 (local, commit não pushado). Dockerfiles Vite→nginx prontos. **WebAdmin tem ZERO testes**; WebRestaurant tem 1 (`App.test.js`).
- **PaymentPanel**: estático puro (sem framework, build via `node scripts/build.js`). **Não tem `package-lock.json`** — o job `npm-audit` do CI aponta `cache-dependency-path: Frontend/PaymentPanel/package-lock.json` para um arquivo inexistente (risco real de falha do job).

### 1.4 Apps mobile

- **AppComida / AppEntrega**: Expo SDK 54 + New Architecture + React 19 + RN 0.81 (local, não pushado). Camada de storage: `config/storage.ts` (MMKV + fallback web), `tokenStorage.ts` (JWT em SecureStore), `legacyMigration.ts`, cache de cardápio com TTL. `config/api.ts` é a **fonte única** da URL (com `getApiUrl()`/`getWsUrl()`), consumida por api.tsx, helpers e LiveTracking. Interceptor 401 → logout → redirect login.

### 1.5 Infra / deploy

- Dockerfiles multi-stage Go 1.23 (monólito + Payment) e Vite→nginx (WebRestaurant/WebAdmin), `.dockerignore`, `docker-compose.payment.yml` (contexto corrigido), `docker-compose.vps.yml` (stack completa presa a 127.0.0.1), guia `scripts/deploy-vps.md` + PDF.
- `render.yaml`: 5 serviços (fuudelivery-api, fuudelivery-web, fuudelivery-admin, fuudelivery-payment, fuudelivery-payment-panel) + redis free. URLs atualizadas para o sufixo vivo `-8y6l`.

### 1.6 Segurança (verificado)

- ✅ Nenhuma chave hardcoded detectada (`git grep` por padrões de secret → vazio).
- ✅ Sem segredos no histórico do git (o `Frontend/WebRestaurant/.env` versionado só contém URLs públicas).
- ⚠️ `.env`/`.env.production` **commitados** (`Frontend/WebRestaurant/.env`, `AppComida/.env.production`, `AppEntrega/.env.production`) — só URLs, mas higiene ruim; `.env.production` não está no `.gitignore`.
- ⚠️ Rate limit só em login/register/bootstrap/webhook — **endpoints de dinheiro** (`/payments/pix/generate`, `/payments/card/charge`, `/wallet/topup`, `/wallet/deduct`) têm JWT mas **sem rate limit** (risco de abuso/custo).
- ⚠️ Webhook sem HMAC (ver 1.2).
- ✅ `JWT_SECRET`/`ABACATE_PAY_API_KEY`/`ADMIN_PASSWORD`/credenciais DB são `sync: false` no render.yaml (preenchidas manualmente no painel).

### 1.7 Testes (quantidade ≠ cobertura, mas é o que temos)

- Go: 37 arquivos `_test.go` (queue, health, Payment, auth, chat, delivery, orders, payment_api, monólito). O CI roda `go test` em todos os 7 módulos.
- Frontend: só WebRestaurant tem teste de UI; **WebAdmin e PaymentPanel não têm testes**.
- **Não há teste E2E do fluxo de dinheiro no monolito** (pedido → PIX → webhook → split → carteira/WebSocket).

---

## 2. Plano de finalização (priorizado)

### FASE 0 — Portão de entrada: CI verde (hoje, 1-2 h)

> Sem isso, nada mais importa — o repositório nunca teve CI verde.

1. **Push dos 5 commits locais** para `origin/master`.
2. **Corrigir os jobs restantes do CI até tudo passar**:
   - `npm-audit` do PaymentPanel: remover da matrix (projeto sem dependências) **ou** gerar `package-lock.json` (`npm install --package-lock-only`).
   - Se `npm audit` ainda falhar em WebRestaurant/WebAdmin pós-Vite: decidir por upgrade de dependência **ou** `--audit-level=high` com justificativa documentada (nunca suprimir em silêncio).
   - Confirmar `go mod tidy` + `go test` nos 7 módulos (fix do testcontainers já está no `cc2132c`, mas só o CI prova).
3. **Rodar `bash scripts/verify-deploy.sh`** contra os 5 serviços de produção — registrar o estado atual antes de qualquer mudança (baseline).
4. **Regra para o resto do projeto**: nenhum merge/push com CI vermelho.

### FASE 1 — Segurança (1-2 dias)

1. **HMAC `x-abacatepay-signature`** no `HandlePaymentWebhook` (defesa em profundidade além da re-verificação via API). Usar `ABACATE_PAY_WEBHOOK_SECRET` já existente no render.yaml.
2. **Rate limit nos endpoints de dinheiro**: `/payments/pix/generate`, `/payments/card/tokenize`, `/payments/card/charge`, `/payments/process`, `/payments/split`, `/wallet/topup`, `/wallet/deduct` — mesmo `rateLimitMiddleware` já usado no login (ex.: 10-20/min por IP).
3. **Tirar `.env` do git**: `git rm --cached Frontend/WebRestaurant/.env Frontend/AppComida/.env.production Frontend/AppEntrega/.env.production` + adicionar `.env.production`/`.env` no `.gitignore`. As URLs já estão centralizadas em `config/api.ts` (mobile) e env do Render (web).
4. **Rotacionar segredos antes do launch** (mesmo sem evidência de vazamento, é o momento barato de fazer): `JWT_SECRET`, `ABACATE_PAY_API_KEY`, `ADMIN_PASSWORD` (Payment), credenciais DB/Mongo, webhook secret.
5. **Checklist de auditoria**: `npm audit` verde, `govulncheck` verde (já no CI), revisar `ALLOWED_ORIGINS` (só domínios próprios + localhost dev).

### FASE 2 — Fechar o fluxo de dinheiro de ponta a ponta (2-3 dias)

1. **Ligar os publishers**: webhook de pagamento publica em `payment_updates` + `order_updates` (alinhando com a ponte WebSocket que já consome esses canais) — sem isso, o app nunca recebe "pagamento confirmado" em tempo real.
2. **Teste E2E do fluxo real (headless)**: criar pedido → gerar PIX (mock AbacatePay) → simular webhook `billing.paid` → verificar split no Mongo (5%/85%/taxa real) → status `CONFIRMED` → notificação WebSocket. Um script de integração que **asserta cada etapa** (o mesmo padrão do `integration_test.go` existente).
3. **Validar split com valores reais**: o `defaultSplitRules` usa `DeliveryAmount` real — adicionar teste unitário do cálculo (incluindo pedido menor que a taxa de entrega → proteção).
4. **Smoke test mobile**: apontar AppComida para staging e percorrer o fluxo de checkout em device físico/emulador antes do launch.

### FASE 3 — Limpeza e dívida técnica (1-2 dias, paralelizável)

1. **Remover/arquivar `Frontend/docker-compose.yml`** (stale da era Expo: `EXPO_USERNAME`/`EXPO_PASSWORD`, IP local `192.168.100.142`, referência ao fluxo antigo).
2. **Decidir sobre os `main.go` dos 5 microserviços**: deletar (deploy absorvido pelo monólito) ou marcar como bibliotecas. Recomendado: manter como está por enquanto (baixo risco, funciona) e documentar — ou deletar depois do CI verde.
3. **Unificar module paths** `vercardapio` → `fuudelivery` (rebranding de módulo): mexe em todos os imports/replaces — fazer em commit dedicado com `go mod tidy` em todos os módulos, ou adiar e só documentar. Risco médio, ganho de identidade.
4. **WebAdmin sem testes**: adicionar smoke test mínimo (renderiza, login com mock) — o CI só testa WebRestaurant hoje.
5. **Adicionar teste no fluxo `payment_updates`** (o teste `main_queue_test.go` já cobre a ponte; falta o publisher).

### FASE 4 — Preparação do launch (1 dia)

1. **Rollback/backup**: snapshot dos serviços Render (ou deploy anterior) + plano de rollback documentado no `scripts/deploy-vps.md` (já tem guia; adicionar seção de rollback explícita).
2. **Monitoramento**: UptimeRobot nos 5 `/health` (job "Monitor Production" já existe no GitHub Actions — verificar se cobre todos os serviços e alertas), logs do Render, métricas `/metrics`.
3. **Domínio + SSL**: seguir o guia VPS (nginx + Certbot) — ou custom domains no Render se ficar no Render.
4. **Checklist final de produção** (14 itens, no `PRODUCTION.md`): rodar e marcar item a item.
5. **Release v1.0.0**: criar a GitHub Release com os APKs (AppComida + AppEntrega) e notas (o script de build EAS e o doc de release notes já existem).

---

## 3. Riscos que "zero falhas" não pode eliminar (honestidade técnica)

- **CI nunca verde**: o maior risco real é assumir que está tudo certo porque "compila local". O portão da Fase 0 existe exatamente para isso.
- **Fila em memória (fallback)**: se o Redis cair, eventos não sobrevivem a restart — aceitável como degradação, mas o painel de health deve deixar isso visível (já retorna "degraded").
- **Vite/Expo 54 não validados por CI**: a migração é grande (webpack→Vite, Expo 51→54). Sem push + CI + build EAS local, não há prova.
- **Dependências de terceiros (AbacatePay/Asaas/Supabase)**: mudanças de API do gateway quebram pagamento — mitigar com testes de contrato no webhook e monitoramento de webhook failures.
- **Testes existem mas cobrem caminhos felizes**: os fluxos de erro (webhook com assinatura errada, charge duplicado, split com valores limite) precisam de testes explícitos.

---

## 4. Ordem de execução recomendada (resumo)

1. **Hoje**: push + CI verde (Fase 0) → só depois qualquer outra coisa.
2. **Semana 1**: Fase 1 (segurança) + Fase 2 (fluxo de dinheiro E2E).
3. **Semana 1-2**: Fase 3 (limpeza) em paralelo.
4. **Launch**: Fase 4 (monitoramento, domínio, release).

**Regra de ouro: cada mudança nova entra com CI verde e, quando tocar em dinheiro ou auth, com teste que asserta o comportamento.**
