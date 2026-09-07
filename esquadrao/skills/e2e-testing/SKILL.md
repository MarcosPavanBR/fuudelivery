---
name: e2e-testing
description: Padrões de teste end-to-end com Playwright (web) e Detox/Maestro (mobile) — Page Object Model, dados de teste isolados, e priorização de fluxo crítico. Use ao escrever testes E2E para checkout, aceite de pedido, ou rastreamento.
---

# Testes End-to-End

## Page Object Model (web)
- Cada página/tela tem uma classe própria com os seletores e ações — o teste em si lê como uma história ("faz login", "adiciona item ao carrinho", "confirma pagamento"), nunca com seletores CSS soltos misturados na lógica do teste.

## Dados de teste
- Todo teste E2E roda contra um ambiente com dados próprios (seed isolado), nunca contra produção ou um ambiente compartilhado onde outro teste pode interferir.
- Dados de teste são limpos ou recriados a cada execução — testes não podem depender da ordem de execução de outros testes.

## Mobile (Detox/Maestro)
- `testID` estável em cada elemento interativo relevante, nunca depender de texto visível como seletor (texto muda com i18n e com copy).

## Priorização
1. Checkout completo (maior risco financeiro).
2. Aceite/recusa de pedido pelo restaurante.
3. Rastreamento de entrega em tempo real.
4. Login/sessão.

Não tente cobrir 100% dos fluxos com E2E — são caros de manter. Fluxos de exibição pura (ver cardápio, ver histórico) ficam melhor cobertos por teste de componente/unidade.
