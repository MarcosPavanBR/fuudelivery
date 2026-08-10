# 🌐 FuuDelivery — Frontends Web: stack, migração e decisões

> **Atualizado em:** 2026-08-03 (migração webpack 5 → Vite 6 + React 19 + Tailwind 4)
> Este documento é a fonte da verdade sobre os 3 frontends web do projeto:
> WebRestaurant, WebAdmin e PaymentPanel.

---

## 1. Panorama — 3 frontends, 3 papéis

| App | Papel | Stack (depois da migração) | Deploy (Render) |
|-----|-------|----------------------------|-----------------|
| **WebRestaurant** | Painel do restaurante (pedidos, cardápio, carteira) | Vite 6 + React 19 + Tailwind 4 | `fuudelivery-web` (static) |
| **WebAdmin** | Painel administrativo (estabelecimentos, usuários, financeiro) | Vite 6 + React 19 + Tailwind 4 | `fuudelivery-admin` (static) |
| **PaymentPanel** | Painel de aprovação/auditoria de pagamentos (FuuPayment) | **HTML/JS vanilla** (mantido — ver §3) | `fuudelivery-payment-panel` (static) |

---

## 2. Migração webpack 5 → Vite 6 (WebRestaurant e WebAdmin)

Realizada em 2026-08-03. Resultado: build ~10x mais rápido, HMR instantâneo,
configuração muito menor (um `vite.config.js` por app em vez de
webpack.config.js + postcss.config.js + tailwind.config.js + .babelrc).

### O que mudou por app

| Item | Antes (webpack 5) | Depois (Vite 6) |
|------|-------------------|-----------------|
| **Bundler** | `webpack.config.js` + `webpack-cli` | `vite.config.js` (`@vitejs/plugin-react`) |
| **JSX/Babel** | `babel-loader` + `.babelrc` | `@vitejs/plugin-react` (esbuild + Fast Refresh) |
| **CSS/Tailwind** | `postcss-loader` + `tailwind.config.js` + `postcss.config.js` | `@tailwindcss/vite` + `@theme` no `index.css` (Tailwind v4, CSS-first) |
| **HTML entry** | `HtmlWebpackPlugin` com `public/index.html` | `index.html` na **raiz** do app (entry do Vite) |
| **Arquivos estáticos** | `CopyPlugin` (SW firebase, `_redirects`) | `public/` copiado automaticamente + `vite-plugin-static-copy` (só p/ SW do firebase no WebRestaurant) |
| **Env vars** | `process.env.REACT_APP_*` | `import.meta.env.REACT_APP_*` ou `VITE_*` (com `envPrefix: ["VITE_", "REACT_APP_"]`) |
| **Testes** | `react-scripts test` (Jest/CRA) | `vitest run` (WebRestaurant) |
| **Node polyfills** | `resolve.fallback` (buffer, crypto, os, path, stream, vm) | Removidos — nenhum uso real no src |

### Arquivos removidos
- `Frontend/WebRestaurant/webpack.config.js`, `tailwind.config.js`, `postcss.config.js`, `.babelrc`, `public/index.html`
- `Frontend/WebAdmin/webpack.config.js`, `tailwind.config.js`, `public/index.html`

### Arquivos criados/alterados
- `Frontend/WebRestaurant/vite.config.js` (novo), `index.html` (raiz), `src/index.css` (@theme), `src/services/api.js`, `src/services/payment.model.js`, `src/App.test.js` (vitest), `package.json`, `.npmrc` (legacy-peer-deps)
- `Frontend/WebAdmin/vite.config.js` (novo), `index.html` (raiz), `src/index.css` (@theme), `src/services/api.js`, `src/services/paymentApi.js`, `package.json`, `.npmrc`

### Variáveis de ambiente (novo padrão)
- Vite expõe só `import.meta.env.*`. Mantivemos **alias `REACT_APP_*`** no
  `envPrefix` para o render.yaml continuar injetando `REACT_APP_API_URL` no
  build sem mudança. Novas variáveis devem usar `VITE_*`.
- Lista canônica de URLs em `references/URLS.md`.

### Tailwind v4 — tema CSS-first
As cores `fuu-*`, fontes, sombras, raios e animações que viviam no
`tailwind.config.js` foram migradas para o bloco `@theme` do `src/index.css`:

```css
@import "tailwindcss";

@theme {
  --color-fuu-red: #EA1D2C;
  --font-body: "Inter", sans-serif;
  --shadow-card: 0 2px 12px rgba(0,0,0,0.06);
  --animate-fade-in: fadeIn 0.3s ease-in-out;
  /* ... */
}
```
As classes usadas no código (`bg-fuu-red`, `font-display`, `shadow-card`,
`animate-fade-in`, etc.) continuam funcionando sem alteração.

---

## 3. ✅ Decisão documentada — PaymentPanel (2026-08-03)

**Decisão: MANTER VANILLA (HTML/JS puro, sem framework).**

O PaymentPanel é um painel **interno** (aprovação/auditoria de pagamentos do
FuuPayment), de baixo volume de uso, com zero dependências runtime e deploy
estático no Render. Migrar para Vite + React + Tailwind 4 **não traz ganho
funcional** para o usuário interno e adicionaria custo de manutenção.

### Justificativa (tradeoffs considerados)

| Critério | Manter vanilla ✅ | Migrar p/ React+Vite |
|----------|------------------|----------------------|
| **Custo de manutenção** | ~0 (arquivo único, sem deps) | Novo pipeline + componentes React |
| **Risco de regressão** | Nenhum (funciona em prod) | Reescrita = risco de quebrar fluxo de aprovação |
| **Funcionalidade necessária** | Tabelas + modais + login — suficiente | Sem feature nova demandada |
| **Consistência de stack** | 3ª stack no projeto ⚠️ | Alinharia aos outros frontends |

### Quando rever esta decisão
- Se o painel ganhar features complexas (gráficos, fluxo multi-etapas, testes).
- Se a equipe quiser reaproveitar componentes do WebAdmin (ex.: tabela de
  pagamentos) — nesse caso, migrar seria justificado.
- A stack do painel **não bloqueia** nenhuma fase do roadmap: ele é interno e
  estável.

---

## 4. Como rodar / buildar

```bash
# WebRestaurant
cd Frontend/WebRestaurant
npm install          # .npmrc força legacy-peer-deps
npm run dev          # Vite dev server (porta 3000)
npm run build        # gera dist/ (usado pelo Render)
npm test             # vitest run

# WebAdmin
cd Frontend/WebAdmin
npm install
npm run dev
npm run build        # gera build/ (mantido para o render.yaml)

# PaymentPanel
cd Frontend/PaymentPanel
npm run build        # copia index.html para dist/
```

**Notas**
- `.npmrc` com `legacy-peer-deps=true` em cada app React: necessário porque
  `react-modal`/`@hello-pangea/dnd` ainda não declaram React 19 nos peers.
- O `render.yaml` não mudou: `staticPublishPath` continua `dist/` (WebRestaurant)
  e `build/` (WebAdmin); `envVars` continua `REACT_APP_API_URL`.
- CI (`ci.yml`): o job `frontend-webrestaurant` agora roda `npm test` (vitest)
  em vez de `npm test -- --watchAll=false` (react-scripts).
