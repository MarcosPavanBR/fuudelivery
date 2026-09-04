# Roadmap de Modernização — Status (FuuDelivery 2.0)


> ⚠️ **`Backend/Payment` (arquivado) foi removido.** Todo o código de pagamento ativo vive em
> `Backend/payment_api` (embutido no monolito `cmd/fuudelivery`). As menções históricas neste
> documento são referências — não edite, não busque e não rode comandos contra o diretório antigo.
Status verificado contra o código em **04/09/2026**. Este documento registra o
que foi concluído em cada fase do roadmap de modernização (`fuudelivery-modernization`).

## Fase 0 — Rebranding ✅

- `Frontend/AppComida/app.json`: `name: FuuDelivery`, `slug: comida`,
  `scheme: fuudelivery`, `package: com.fuudelivery.comida`.
- `Frontend/AppEntrega/app.json`: `name: FuuDelivery Entregas`, `slug: entrega`,
  `scheme: fuuentrega`, `package: com.fuudelivery.entrega`, `bundleIdentifier: com.fuudelivery.entrega`.
- Nenhum resquício de CoopFood/CoopBike em `app.json` dos apps mobile.

## Fase 1 — Fundação ✅ (Expo + MMKV + Vite)

| Item | Status |
|---|---|
| Expo SDK 51 → 57 (AppComida, AppEntrega, AppRestaurante) | ✅ `expo ~57.0.0`, RN 0.86.3, React 19.1.0 |
| New Architecture (`newArchEnabled`) | ✅ `true` nos dois apps |
| MMKV no lugar de AsyncStorage (AppComida) | ✅ `config/storage.ts` + `react-native-mmkv ~4.3.2` |
| Remoção de AsyncStorage não usado (AppEntrega) | ✅ dependência removida do package.json |
| Vite + React 19 + Tailwind 4 (WebRestaurant/WebAdmin) | ✅ migração completa + vitest |
| PaymentPanel (decisão) | ✅ documentada em `references/frontends-web.md` (mantido vanilla) |

## Fase 2 — Confiabilidade de plataforma ✅ (Streams) / ⏳ (OTel)

| Item | Status |
|---|---|
| Fila → Redis Streams + consumer groups + retry + DLQ + reclaim | ✅ `pkg/queue` (XAdd/XReadGroup/XAck/XClaim) |
| `SubscribeFunc` com retry/DLQ no Payment Service | ✅ `Backend/payment_api (monolith)/queue/redis_queue.go` |
| Métricas em formato Prometheus (`GET /metrics`) | ✅ `cmd/fuudelivery/pkg/metrics` + contadores no `pkg/queue` |
| OpenTelemetry (exportação OTLP) | ⏳ SDK não configurado — depende de collector/endpoint (ver nota) |

> **Nota OTel:** os contadores da fila e HTTP estão expostos em `/metrics` (Prometheus
> text, zero dependências). A exportação OTLP exige um collector (ex.: Grafana Cloud,
> New Relic, Jaeger). Quando houver um endpoint, configurar a env `OTEL_EXPORTER_OTLP_ENDPOINT`.
> O `pkg/metrics` foi desenhado para isso.

## Fase 3 — Busca e IA (1º marco ✅, busca vetorial ⏳)

| Item | Status |
|---|---|
| Busca full-text básica (`GET /search?q=`) | ✅ `cmd/fuudelivery/pkg/search` (ILIKE + scoring em Postgres) |
| Testes unitários do scoring | ✅ `score_test.go` |
| Busca vetorial / embeddings | ⏳ construção nova — requer Atlas Vector Search/pgvector |
| "Fuu AI" assistente de pedido (RAG) | ⏳ depende da indexação acima |

## Fase 4 — Dispatch e operação (parcial ✅, KDS ⏳)

| Item | Status |
|---|---|
| OSRM (rotas) | ✅ já integrado em `delivery_api`/`orders_api` |
| Batching (já existente) | ✅ estrutura existente em `matching_engine.go` |
| KDS (Kitchen Display System) no WebRestaurant | ⏳ WebSocket já existe — falta UI |
| "Oportunidades" com oferta (AppEntrega) | ⏳ greenfield |

## Fase 5 — Pagamentos e confiança no app ⏳

| Item | Status |
|---|---|
| Apple Pay / Google Pay | ⏳ requer conta Apple Developer + Google Play Console |
| PIX em loja | ⏳ PIX já existe via AbacatePay (backend) — falta UI no app |
| Biometria (`expo-local-authentication`) | ⏳ dependência nova a instalar |
| Pedidos em grupo | ⏳ greenfield |

## Fase 6 — Ousado (parcial ✅, resto ⏳)

| Item | Status |
|---|---|
| Live tracking cinematográfico | ✅ base instalada (reanimated + react-native-maps) — composição de UI pendente |
| Mesa digital via QR | ✅ backend já tem `qrcode.go` — evoluir para pedido na mesa é extensão |
| Gamificação / reviews com IA / AR / SOS | ⏳ greenfield |

## Resumo

- **Concluído e validado:** Fases 0, 1, 2 (Streams + métricas), 3 (1º marco), e itens
  pontuais das Fases 4 e 6.
- **Pendente (requer infra/contas externas):** OTel export, busca vetorial, KDS,
  Apple/Google Pay, biometria, gamificação/AR.
- **Prontidão de produção:** ver `references/confiabilidade-deploy.md` (checklist de
  deploy) e a skill `fuudelivery-production-readiness` (credenciais, rate limiting,
  testes de pagamento).
