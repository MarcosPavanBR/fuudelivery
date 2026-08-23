# 🔗 FuuDelivery — Referência de URLs (Produção)


> ⚠️ **`Backend/Payment` foi arquivado e removido do repositório.** Todo o código
> de pagamento ativo vive em `payment_api` (embutido no monolito `cmd/fuudelivery`).
> As menções a `Backend/Payment` neste documento são **históricas** — não edite,
> não busque e não rode comandos apontando para esse diretório.
> ⚠️ **Os serviços Render `fuudelivery-payment` e `fuudelivery-payment-panel`
> foram REMOVIDOS (2026-08).** Qualquer URL `fuudelivery-payment*.onrender.com`
> está morta — não use, não provisione, não monitore.
> **Última auditoria:** 22 de agosto de 2026

---

## 📋 Mapa de Serviços (URLs Ativas)

| Serviço | Função | URL | Status |
|---------|--------|-----|--------|
| **API (Monolito)** | Backend principal — auth, pedidos, entregas, chat, PAGAMENTOS, health | `https://fuudelivery-api-8y6l.onrender.com` | ✅ 200 |
| **WebRestaurant** | Painel do restaurante (React) | `https://fuudelivery-web.onrender.com` | ✅ 200 |
| **WebAdmin** | Painel administrativo (React) | `https://fuudelivery-admin-lv7f.onrender.com` | ✅ 200 |
| **Repositório** | Código-fonte | `https://github.com/MarcosPavanBR/fuudelivery` | ✅ |
| ~~Payment Service~~ | ❌ REMOVIDO 2026-08 — rotas absorvidas pelo monolito | ~~fuudelivery-payment.onrender.com~~ | 💀 morto |
| ~~PaymentPanel (deploy)~~ | ❌ REMOVIDO 2026-08 — código arquivado em `legacy/PaymentPanel/`, não é deployado (aba Financeiro do WebAdmin o substitui) | ~~fuudelivery-payment-panel.onrender.com~~ | 💀 morto |

---

## 🩺 Endpoints de Health Check

| Serviço | Endpoint |
|---------|----------|
| API (Monolito) | `https://fuudelivery-api-8y6l.onrender.com/health` |
| WebRestaurant | `https://fuudelivery-web.onrender.com` |
| WebAdmin | `https://fuudelivery-admin-lv7f.onrender.com` |

---

## 📱 Apps Mobile — API URL usada em build

| App | URL da API | Onde está definida |
|-----|-----------|--------------------|
| **AppComida** (cliente) | `https://fuudelivery-api-8y6l.onrender.com` | `config/api.ts` (`API_URL` — fonte única) → consumida por `services/api.tsx`, `helpers/helpers.ts` e `LiveTrackingReadonly.tsx` |
| **AppEntrega** (entregador) | `https://fuudelivery-api-8y6l.onrender.com` | `config/api.ts` (`API_URL` — fonte única) → consumida por `services/api.tsx` e `helpers/helper.tsx` |
| **WebRestaurant** | `https://fuudelivery-api-8y6l.onrender.com` | `src/services/api.js` + env `REACT_APP_API_URL` |
| **WebAdmin** | `https://fuudelivery-api-8y6l.onrender.com` | `src/services/api.js` + env `REACT_APP_API_URL` |
| **WebAdmin** (pagamentos) | `https://fuudelivery-api-8y6l.onrender.com` | `src/services/paymentApi.js` (fallback hardcoded; env `REACT_APP_PAYMENT_API_URL`) |
| **PaymentPanel** | `https://fuudelivery-api-8y6l.onrender.com` | `index.html` (`API_URL`, switch por hostname — corrigido 2026-08-22 para o monolito) |

---

## 🌐 CORS (ALLOWED_ORIGINS)

O monolito (`cmd/fuudelivery/main.go`)
aceita as seguintes origens (a env var `ALLOWED_ORIGINS` **soma** com esses defaults,
nunca os substitui):

```
https://fuudelivery-web.onrender.com
https://fuudelivery-admin-lv7f.onrender.com
```

Além disso, o middleware aceita programaticamente (sem precisar de env):

- **Previews do Freebuff Cloud** — qualquer subdomínio de `https://*.daytonaproxy01.net`
  (formato `<porta>-<workspace-uuid>.daytonaproxy01.net`), para o preview do
  WebRestaurant conseguir chamar a API de produção.
- **Desenvolvimento local** — `localhost` / `127.0.0.1` / `::1` em qualquer porta.

---

## 🛠️ Scripts de Operação

| Script | URL usada |
|--------|-----------|
| `scripts/verify-deploy.sh` | API + WebRestaurant + WebAdmin (Payment Service removido) |
| `scripts/load-test.sh` | API (default; `PAY_URL` obsoleto — serviço removido) |
| ~~`scripts/keepalive-payment.ps1`~~ | ❌ obsoleto — serviço removido; pode ser deletado |
| `scripts/seed-data.sh` | API (argumento; default localhost:3000) |

---

## 📌 Histórico de Correções

### 2026-08-02 — URLs removidas (retornavam 404)

| URL morta | Onde aparecia | Substituída por |
|-----------|---------------|-----------------|
| `fuudelivery-api.onrender.com` | `render.yaml` (API_BASE_URL), `PRODUCTION.md`, `seed-data.sh` | `fuudelivery-api-8y6l.onrender.com` |
| `fuudelivery-admin.onrender.com` | `references/RELEASE-NOTES-v1.0.0.md` | `fuudelivery-admin-lv7f.onrender.com` |
| `fuudelivery-web-lv7f.onrender.com` | `.github/workflows/release.yml` | `fuudelivery-web.onrender.com` |

> **Nota:** as URLs `-8y6l`/`-lv7f` são o sufixo aleatório que o Render atribui a cada
> serviço. Se um serviço for recriado no Render, o sufixo muda — atualizar este arquivo
> e o `render.yaml` em conjunto.
>
> **Apps mobile:** a URL da API de cada app vive em `config/api.ts` (constante `API_URL`),
> que também expõe `getApiUrl()` (override via `EXPO_PUBLIC_API_URL` p/ dev/staging) e
> `getWsUrl()` (derivada para WebSocket). Para trocar a URL de produção dos apps, edite
> **apenas** a constante `API_URL` nos dois `config/api.ts` e o `eas.json` não precisa
> de mudanças (os blocos `env` duplicados foram removidos na centralização).
