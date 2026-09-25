# Plano — dinheiro em centavos inteiros (NEG-04)

> Status: **planejado, não iniciado.** Decisão de 2026-09-25: planejar agora e migrar
> depois, em PRs pequenos. Nada neste documento foi aplicado ao código.

## Por que

Todo valor monetário no backend é `float64` (e `float32` no frete da loja). `float`
não representa centavos com exatidão: `0.1 + 0.2 = 0.30000000000000004`. Hoje isso é
contido com arredondamentos espalhados (`roundCents` no split, `roundMoney` na dívida
de repasse, `gateway.ToCents` na borda do gateway), mas cada conta nova é uma chance
de esquecer um deles. Os sintomas possíveis são centavo a mais ou a menos no ledger,
conciliação que não fecha e comparação `==` entre valores que "deviam" ser iguais.

## O que já está a favor

- **As tabelas do domínio de pagamento já são `NUMERIC(12,2)`** (`sql/03`, `23`, `24`,
  `27`): `payments.amount`, `delivery_amount`, `discount_amount`, `wallets.balance`,
  `wallet_transactions.amount`, `establishment_debts.*`, `delivery_region_fees.fee`,
  `delivery_solicitations.total`. O banco guarda o valor exato; o erro nasce só na
  aritmética em Go. **Para essas tabelas não há migração de schema.**
- `pkg/gateway/money.go` já converte reais → centavos (`ToCents`) com arredondamento
  correto na borda dos gateways.

## O que precisa de schema

Tabelas criadas pelo AutoMigrate com `float64` (viram `double precision`) ou `float32`
(viram `real`, com ~7 dígitos de precisão):

| Tabela | Colunas | Modelo |
|---|---|---|
| `products` | `price` | `orders_api/app/models/Product.go` |
| `additionals` | `price` | `orders_api/app/models/Additional.go` |
| `coupons` | `discount_value`, `min_order_value` | `orders_api/app/models/coupon.go` |
| `coupon_usages` | `discount_amount` | idem |
| `deliveries` | `fixed_taxa`, `per_km` (**float32**) | `orders_api/app/models/Delivery.go` |
| `loyalty_points` | `total_spent` | `orders_api/app/models/loyalty.go` |
| `subscriptions`, `sponsored_listings` | `amount` | `auth_api/app/models/` |
| `batches` | `total_amount` | `orders_api/app/models/batch.go` |

## Estratégia

**Tipo único:** um `type Cents int64` num pacote compartilhado (`pkg/money`), com
`FromReais(float64)`, `Reais() float64` (só para exibição e JSON legado), soma, subtração
e percentual com arredondamento definido num lugar só (meio centavo para cima, como o
`roundCents` atual). Porcentagem de split vira `Cents.Percent(bps int64)` com a taxa em
pontos-base (5% = 500), sem `float` no meio.

**Fronteiras não mudam de uma vez.** A API JSON continua em reais (`89.90`) até os apps
migrarem. A conversão acontece na borda (handlers e DTOs). O núcleo (split, carteira,
dívida, total do pedido, cupom) passa a operar em `Cents`.

## Etapas (um PR cada, em ordem)

1. **`pkg/money`** com `Cents`, testes de propriedade (soma, percentual, arredondamento,
   `FromReais(0.1)+FromReais(0.2) == FromReais(0.3)`). Sem uso ainda.
2. **Split** (`payment_api/app/services/split_calculator.go`): `CalculateSplitRules`
   calcula em `Cents` e converte só no `SplitRule.Amount`. O invariante "as partes
   somam o total" passa a ser igualdade exata. Os testes atuais de split são a rede.
3. **Carteira e ledger** (`models/wallet.go`, `AdjustWalletBalance`): ler o `NUMERIC`
   direto para `Cents` (scanner próprio, sem passar por `float64`), comparação de saldo
   em inteiro. Sem migração de schema.
4. **Dívida de repasse e conferência de cobrança** (`establishment_debt.go`,
   `validateChargeAmount`, `order_total.go`): trocar `roundMoney` e as tolerâncias de
   "1 centavo" por igualdade exata.
5. **Total do pedido** (`orders_api/.../orders.go` `computeOrderTotal`, cupom, frete):
   em `Cents`. Aqui entra o schema: migração SQL que converte `products.price`,
   `additionals.price`, `coupons.*`, `deliveries.*` para `NUMERIC(12,2)`
   (`ALTER COLUMN ... TYPE NUMERIC(12,2) USING round(col::numeric, 2)`), idempotente e
   rodada ANTES do deploy do código, com as tags do GORM ajustadas para `numeric` para
   o AutoMigrate não brigar com a coluna.
6. **Gateways** (`pkg/gateway`): os adapters já recebem centavos via `ToCents`; passam a
   receber `Cents` direto e `ToCents` sai.
7. **API e apps** (opcional, por último): campos novos `*_cents` nos JSONs ao lado dos
   atuais; apps migram; campos em reais saem numa versão seguinte.

## Critério de pronto por etapa

- Testes existentes verdes sem afrouxar asserção (nenhum `InDelta` novo).
- Nenhum `float64` monetário no pacote migrado (checagem com `grep`/revisão).
- Para as etapas com schema: migração testada em banco com dados reais anonimizados,
  com contagem de linhas e soma por coluna idênticas antes e depois.

## Riscos

- **Arredondamento diferente do atual** muda valores em centavo em pedidos novos. Travar
  com testes de caracterização (valores de hoje) antes de trocar a implementação.
- **`float32` no frete** (`deliveries`): valores como 7,90 já podem estar gravados como
  7,8999996. A migração da etapa 5 arredonda para 2 casas; conferir a lista de lojas
  afetadas antes.
- **Relatórios** que somam em Go (`reports.go`) precisam migrar junto com a etapa 3.

## Esforço estimado

Etapas 1–4 (núcleo financeiro, sem schema): 3–5 dias. Etapa 5 (pedido + schema): 2–3
dias. Etapas 6–7: 1–2 dias no backend, mais o trabalho nos apps.
