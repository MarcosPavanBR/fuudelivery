# FuuDelivery — Painel do Restaurante (WebRestaurant)

Painel web do restaurante: Kanban de pedidos em tempo real, gestão de
cardápio, carteira/saques e relatórios.

## Stack

- React 19 + Vite 6 + Tailwind CSS 4
- React Router 6 (BrowserRouter), react-toastify, @hello-pangea/dnd
- WebSocket (`react-use-websocket`) com fallback para polling (15s)
- Testes: Vitest + Testing Library

## Desenvolvimento

```bash
npm install
npm run dev        # http://localhost:3000
npm test           # vitest run
npm run build      # gera dist/ (+ 404.html para SPA no Render)
```

### Variáveis de ambiente

O Vite só expõe variáveis com o prefixo `VITE_` (ver `vite.config.js`).
Copie `.env.example` para `.env`:

```
VITE_API_URL=https://fuudelivery-api-8y6l.onrender.com
```

Em dev, aponte para uma API local se estiver rodando o monolito Go
(`VITE_API_URL=http://localhost:3000`). **Sem essa variável o build cai no
fallback de produção** (`src/services/api.js`).

## PWA

O painel é instalável (Adicionar à tela inicial): `manifest.json` +
`public/sw.js` (shell offline; chamadas de API nunca são cacheadas).

## Deploy

Render (static site) via `render.yaml` na raiz do repo — build `npm run
build`, publish `dist/`, rewrite `/* → /index.html`.

## Marca

Paleta e logotipos oficiais em `../../brand/` (fonte única da verdade).
Vermelho primário `#DC2626`.
