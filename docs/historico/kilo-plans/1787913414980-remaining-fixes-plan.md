# Plano: Itens Restantes FuuDelivery

**Data:** 2026-08-28  
**Status:** 11/15 itens concluídos. 5 itens restantes abaixo.

---

## 1. Frontend — Migrar localStorage → HttpOnly Cookies

**Arquivos:**
- `Frontend/WebRestaurant/src/services/api.js`
- `Frontend/WebAdmin/src/services/api.js`
- Ambos os `AuthContext.js`

**Problema:** JWT e refresh token armazenados em `localStorage`. Qualquer XSS exfiltra todos os tokens.

**Plano:**
1. Backend: adicionar `POST /auth/session` que seta cookies `HttpOnly; Secure; SameSite=Strict`.
2. Backend: modificar `ValidateJWT` para aceitar cookie `access_token` além do header.
3. Backend: atualizar CORS para `AllowCredentials: true`.
4. Frontend: no login, chamar `/auth/session` em vez de armazenar tokens no body.
5. Frontend: remover `localStorage.setItem('token', ...)` e ler cookies via `document.cookie` apenas para APIs não-padrão.
6. Backend: adicionar `POST /auth/logout` que limpa cookies.
7. Backend: adicionar CSRF token em cookie `csrf_token` + validação em mutações.

**Ordem:** backend primeiro (items 1-3, 6-7), depois frontend (items 4-5).

---

## 2. Frontend — Content Security Policy (CSP)

**Arquivos:**
- `cmd/fuudelivery/main.go` (middleware para APIs)
- `Frontend/WebRestaurant/index.html`
- `Frontend/WebAdmin/index.html`

**Plano:**
1. Backend: adicionar header `Content-Security-Policy: default-src 'self'` em todas as respostas via middleware.
2. Frontend: adicionar meta tag CSP no `index.html`:
   ```html
   <meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; connect-src 'self' wss: https:;">
   ```
3. Adicionar `X-Content-Type-Options: nosniff` em todas as respostas.
4. Remover `'unsafe-inline'` gradualmente após migrar estilos para Tailwind puro.

---

## 3. Frontend — URLs Hardcoded de Produção

**Arquivos:**
- `Frontend/AppComida/config/api.ts`
- `Frontend/AppEntrega/config/api.ts`
- `Frontend/AppRestaurante/config/api.ts`

**Plano:**
1. Alterar `config/api.ts` para usar `Constants.expoConfig?.extra?.apiUrl` como primário.
2. Manter fallback apenas para `EXPO_PUBLIC_API_URL`.
3. Remover URL hardcoded dos 3 arquivos.
4. Documentar que builds de produção devem usar `EXPO_PUBLIC_API_URL`.

---

## 4. Backend — Migrar ChargeCard para PaymentRouter

**Arquivo:** `Backend/payment_api/app/handlers/card.go`

**Problema:** `ChargeCard` usa `services.NewAbacatePayClient()` diretamente, ignorando router e circuit breaker.

**Plano:**
1. Injete `gateway.Router` no handler ou via contexto.
2. Substituir chamada direta ao AbacatePay por `router.CreateTransactionWithFallback(ctx, req)`.
3. O router já gera `IdempotencyKey` automaticamente.
4. Adicionar teste: simular 5 falhas AbacatePay → request cai para Asaas.

---

## 5. Testes de Integração Faltantes

**Arquivos existentes:**
- `Backend/orders_api/app/handlers/ownership_test.go` (estrutura criada)
- `Backend/orders_api/app/handlers/coupon_test.go` (teste de race adicionado)
- `Backend/orders_api/app/handlers/loyalty_test.go` (teste de race adicionado)
- `Backend/payment_api/app/services/split_calculator_test.go` (criado)

**Falta:**
1. Completar `ownership_test.go` com casos HTTP reais via `app.Test()`.
2. Adicionar `cors_test.go` em `cmd/fuudelivery/`.
3. Adicionar `split_calculator_test.go` em `Backend/payment_api/app/services/` (já criado, verificar cobertura).

---

## Prioridade de Execução

1. **HttpOnly cookies (backend)** — remove risco de XSS roubendo tokens.
2. **CSP headers** — defesa em profundidade contra XSS.
3. **CSRF** — proteção de mutações.
4. **Mobile URLs** — DevOps, baixo risco.
5. **ChargeCard router** — arquitetura, mas não bloqueante.

## Validação

```bash
# Backend
cd Backend/payment_api && go test ./...
cd Backend/orders_api && go test ./...
cd cmd/fuudelivery && go test ./...

# Frontend
cd Frontend/WebRestaurant && npm test
cd Frontend/WebAdmin && npm test
```
