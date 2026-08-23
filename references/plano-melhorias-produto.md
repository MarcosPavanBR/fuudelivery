# Plano de Melhorias — Backend, Webs e 3 Apps Mobile

> Criado em 2026-08-23, baseado em auditoria do código real.
>
> **✅ DECISÕES TOMADAS (2026-08-23):**
> 1. **Cor oficial = vermelho do brand (`#DC2626`)** — já propagada para
>    WebRestaurant, WebAdmin e AppComida (app.json).
> 2. **PaymentPanel = ARQUIVADO** em `legacy/PaymentPanel/`.
> 3. **1º app a receber identidade nova = AppComida** (cliente final).
>
> **✅ EXECUTADO (2026-08-23 — Fases 0/1 e início da 2):**
> - Fase 0: tokens do brand consolidados em WebRestaurant + WebAdmin (@theme
>   + :root), zero hex antigo (`#EA1D2C`/`#C41420`/`#F7A11E`) restante.
> - Fase 1a: WebRestaurant 100% na paleta do brand (64+ ocorrências trocadas,
>   sombras rgba incluídas).
> - Fase 1b: WebAdmin na paleta do brand + **code splitting** (React.lazy em
>   7 rotas + manualChunks react/icons): chunk principal **418 kB → 107 kB**.
> - Fase 2a (AppComida): accent de personalidade "quente" centralizado como
>   token (`Colors.light.accent` = laranja #F97316) em 5 componentes;
>   ícone adaptativo Android com fundo do brand; `tsc --noEmit` limpo.
> - PaymentPanel arquivado + 6 docs atualizados (README, CONTRIBUTING,
>   URLS, frontends-web, deploy-vps, plano).
>
> **Pendente:** Fase 2b (AppEntrega → NativeWind), 2c (ícones/splash nativos
> dos 3 apps), Fase 3 (canais EAS), Fase 4 (cortes 3/4 do backend).
> Princípio central: **usar o kit de marca existente em `brand/` como fonte única**
> de identidade visual em todas as frentes (tokens, logos e overlay já prontos),
> e evoluir o backend por cortes de risco incremental.

---

## Visão geral das fases

| Fase | Escopo | Esforço | Risco |
|---|---|---|---|
| **0. Fundações** | Design system compartilhado + qualidade backend | 2-3 dias | Baixo |
| **1. Identidade visual web** | WebRestaurant + WebAdmin + PaymentPanel | 2-3 dias | Baixo |
| **2. Apps mobile personalizados** | AppComida + AppEntrega + AppRestaurante | 3-4 dias | Médio |
| **3. Atualização remota (OTA)** | EAS Update + deploy contínuo | 1 dia | Baixo |
| **4. Backend: cortes restantes** | delivery + pagamentos no Postgres | 5-8 dias | Alto |

---

## Fase 0 — Fundações (antes de qualquer tela)

**Objetivo:** uma única fonte de verdade visual para os 6 produtos.

1. **Consolidar tokens do `brand/tokens.ts`** como fonte única:
   - Cores primárias (#DC2626), secundárias (#F59E0B), neutras, tipografia,
     raios de borda, sombras e espaçamentos — já existem em `brand/`, faltam
     apenas consumir de fato em todos os projetos.
   - Hoje: WebRestaurant usa `--color-fuu-red: #EA1D2C` e o brand define
     `#DC2626` — **há divergência de vermelho entre web e brand**. Decidir qual
     é o oficial e propagar (provável: o do brand).
2. **Design system mínimo compartilhado** (`brand/ui`):
   - Componentes base: Button, Input, Card, Badge de status (usar as cores de
     `DELIVERY_STATUS` que já existem no config), EmptyState, Skeleton.
   - Para web: exportar classes/utilitários Tailwind; para mobile: componentes
     NativeWind com os mesmos tokens.
3. **Qualidade backend (paralelo, sem tocar em telas):**
   - Logger estruturado (JSON) substituindo `log.Printf` solto, com request ID.
   - Padronizar respostas de erro (`{error, code}`) em todos os handlers.
   - Sentry (ou similar) para erro tracking — hoje erros só vão para stdout.

**Critério de aceite:** qualquer produto consegue importar os tokens sem
hardcode de cor; CI continua verde.

---

## Fase 1 — Identidade visual web

### 1a. WebRestaurant (painel do restaurante)
- Aplicar overlay `brand/overlay/Frontend/WebRestaurant` (já existe!).
- Substituir vermelhos hardcoded por tokens do @theme.
- Estados de UI consistentes: skeletons nas listas de pedidos, empty states
  ilustrados, toasts padronizados (react-toastify já está lá).
- Kanban (@hello-pangea/dnd): cartões com badge de status colorido por
  `DELIVERY_STATUS`, avatar do cliente, timer desde a criação.
- Dark mode: já existe toggle em AuthContext — garantir contraste AA nos dois
  temas com os tokens.
- Acessibilidade: focus rings visíveis, labels em inputs, navegação por teclado.

### 1b. WebAdmin (painel administrativo)
- Code splitting (mesmo padrão já aplicado ao WebRestaurant: React.lazy +
  manualChunks) — bundle atual: 418 kB num chunk só.
- Sidebar/Layout com identidade FUUDELIVERY (logo do brand, gradiente sutil).
- Aba Financeiro: cards de KPI com sparklines, tabela com filtros salvos.
- Tabela de status com os mesmos badges de cor do config.

### 1c. PaymentPanel (HTML standalone)
- **Decisão primeiro:** manter ou arquivar? O WebAdmin já tem aba Financeiro
  completa. Se mantiver, aplicar o mesmo tema; senão, arquivar em `legacy/`.

**Critério de aceite:** nenhum hex hardcoded fora do arquivo de tokens;
Lighthouse acessibilidade ≥ 90 nas duas webs; bundles < 300 kB iniciais.

---

## Fase 2 — Apps mobile personalizados ("algo nosso")

### Estado atual
| App | Stack visual | Observação |
|---|---|---|
| AppComida (cliente) | NativeWind ✅ | Base boa para liderar o design |
| AppEntrega (entregador) | StyleSheet puro ❌ | Inconsistente com os outros |
| AppRestaurante | NativeWind ✅ | Tem script EAS Update pronto |

### 2a. Identidade personalizada por app (com família visual comum)
- **Família comum:** mesmas cores do brand, mesma tipografia, mesmo header
  (`FuudeliveryHeader.tsx` já existe em `brand/`!), splash animado com o logo.
- **Personalidade por app:**
  - **AppComida:** quente e apetitoso — fotos grandes, cards arredondados,
    accent amarelo nos CTAs, animação no tracking de entrega.
  - **AppEntrega:** foco e legibilidade — alto contraste para uso na rua,
    botões grandes, mapa em destaque, modo escuro nativo (entregadores à noite).
  - **AppRestaurante:** densidade de informação — estilo "cockpit" com métricas
    do dia no topo, igual ao WebRestaurant para consistência entre canais.
- Aplicar os arquivos de `brand/overlay/Frontend/App{Comida,Entrega}` (já
  existem: Colors.ts, HeaderMain, splash etc.).

### 2b. AppEntrega: migrar para NativeWind
- Unifica a stack com os outros 2 apps e permite compartilhar os mesmos tokens.
- Migração gradual: tokens → componentes novos em NativeWind → telas antigas.

### 2c. Ícones e splash
- Adaptive icons Android + iOS com logo do brand (verificar se os assets em
  `brand/logos` têm todas as resoluções exigidas pela Play/App Store).

**Critério de aceite:** zero cor fora dos tokens; screenshots dos 3 apps
parecem da mesma família; AppEntrega em NativeWind.

---

## Fase 3 — Atualização remota (OTA)

Boa notícia: **os 3 apps já têm EAS Update configurado** (`updates` no app.json).

1. **Canais:** definir `production` / `staging` nos perfis do eas.json.
2. **Fluxo:** mudança de JS/CSS → `eas update --branch production` → usuários
   recebem na próxima abertura (**sem revisão das lojas**). É assim que as
   melhorias visuais da Fase 2 chegam rápido.
3. **Limites do OTA:** mudanças nativas (ícones novos, permissões, SDK) exigem
   build novo via EAS Build + atualização nas lojas — planejar 1 release nativa
   após a Fase 2 para consolidar.
4. **Web:** Render já faz auto-deploy no push — nada a fazer além de manter CI verde.
5. **Rollback:** documentar `eas update --rollback` e republish por canal.

**Critério de aceite:** melhoria visual publicada via OTA chega a device real
em < 10 min, com rollback testado.

---

## Fase 4 — Backend: cortes restantes + robustez ✅ EXECUTADA (2026-08-23)

> Cortes 3 e 4 concluídos no código: handlers de delivery e pagamentos usam
> Postgres/GORM como primário com dual-write best-effort; ETL one-shot em
> `cmd/etl-payments`; suíte E2E reescrita para testcontainers-postgres.
> Restante: corte 5 (`pickup_code`/`review`/`scheduling`/`reorder` ainda no
> Mongo) + desligar o Atlas após ciclo financeiro de observação. Ver
> `docs/ARQUITETURA-BANCO-UNICO.md` para status detalhado e runbook do ETL.

Histórico original:

1. ~~**Corte 3 — delivery_solicitations → Postgres** (motor de despacho):
   mapear OrderDTO para a tabela sql/02, dual-write + comparador de matching
   rodando em paralelo antes de desligar Mongo.~~ ✅
2. **Corte 4 — pagamentos/carteiras → Postgres**: escrever o ETL de
   reconciliação (casamento por order_id entre os Mongos), rodar dual-write
   por um ciclo financeiro completo, então cortar.
3. **Desligar o MongoDB** (remover MONGO_URI do Render) quando 3+4 estiverem
   estáveis — economiza o custo do Atlas inteiro.
4. **Dívida técnica em paralelo:** compartilhar container Mongo nos testes de
   integração (corta ~4 min do CI); extrair OSRM/Haversine duplicado para
   `pkg/geospatial`; camada Repository para desacoplar handlers do banco.

**Critério de aceite:** health check sem dependência de Mongo; custo Atlas zerado;
matching idêntico em paralelo por 7 dias antes do corte final.

---

## Ordem recomendada de execução

```
Semana 1: Fase 0 (fundações) + Fase 1 (webs)
Semana 2: Fase 2 (apps) ── publica via OTA (Fase 3) conforme fica pronto
Semana 2-4: Fase 4 (backend) em paralelo, começando pelo ETL de pagamentos
```

## O que eu preciso de você para começar

1. **Cor oficial:** confirmar o vermelho do brand (#DC2626) ou o atual da web (#EA1D2C).
2. **PaymentPanel:** manter (e tematizar) ou arquivar?
3. **Prioridade dos apps:** qual app recebe a identidade nova primeiro?
   Sugestão: AppComida (é o cliente final).
