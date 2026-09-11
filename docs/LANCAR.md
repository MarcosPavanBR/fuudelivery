# LANÇAR — a folha única

Sequência exata para colocar em produção. Sem teoria: o porquê de cada item
está nos documentos linkados.

**Estado do código:** pronto. `master` verde no CI. Nada aqui depende de mais
desenvolvimento.

---

## 1. BLOQUEADOR — rotacionar as chaves do Supabase

É o único item que impede o lançamento. As chaves expostas dão escrita direta
em `payments` e `wallets` **ignorando o RLS** — ou seja, passando por cima de
toda a validação do backend.

Supabase Dashboard → **Project Settings → API**:

1. Em *API Keys*: revogue a `sb_secret_…` exposta e gere outra.
2. Em *JWT Keys*: **Rotate** o JWT secret (invalida `service_role` e `anon`).
3. Render → Environment → atualize `SUPABASE_SERVICE_ROLE_KEY` → **Save**
   (dispara redeploy).

Passo a passo completo: [`runbook-rotacao-credenciais.md`](runbook-rotacao-credenciais.md)

**Impacto de rotacionar:** só o upload de imagens para enquanto a variável não
é atualizada. Os apps mobile não usam chave do Supabase — falam com a API Go.

---

## 2. Subir

O Render faz deploy automático no push. Confirme que subiu o commit certo:

```bash
curl -s https://fuudelivery-api-8y6l.onrender.com/health | head -c 300
```

Como ler a resposta:

| `status` | HTTP | Significado |
|---|---|---|
| `up` | 200 | pronto |
| `degraded` | 200 | Postgres e gateway de pé; só Redis/batches degradados — **vende normalmente** |
| `starting` | 200 | subindo (até 125s). Se persistir, o banco não conectou |
| `down` | 503 | crítico fora do ar |

Cold start do free tier leva 30–60s na primeira chamada. `starting` nos
primeiros minutos é normal.

E os dois painéis:

```bash
curl -s -o /dev/null -w "%{http_code}\n" https://fuudelivery-web.onrender.com
curl -s -o /dev/null -w "%{http_code}\n" https://fuudelivery-admin-lv7f.onrender.com
```

---

## 3. Teste de fumaça do dinheiro

**Faça isto antes de divulgar para clientes reais.** Nenhum teste automatizado
substitui: é o único que prova o caminho PIX → split → carteira ponta a ponta
com dinheiro de verdade.

Faça **um pedido real de valor baixo** pelo AppComida, pague o PIX, e rode no
SQL Editor do Supabase:

```sql
-- 1. o pagamento confirmou E liquidou?
SELECT abacatepay_id, status, confirmed_at, establishment_credited_at, amount
FROM payments ORDER BY created_at DESC LIMIT 1;
-- establishment_credited_at NÃO pode ser NULL

-- 2. o crédito entrou no ledger uma vez só?
SELECT wallet_id, type, amount, reference_id, created_at
FROM wallet_transactions ORDER BY created_at DESC LIMIT 3;
-- exatamente 1 linha 'credit' para este reference_id

-- 3. o saldo do restaurante bateu?
SELECT user_id, user_type, balance FROM wallets WHERE user_type = 'establishment';
```

Se `establishment_credited_at` ficar NULL: o job de reconciliação liquida
sozinho em até 5 minutos. Rode a consulta 1 de novo. Se continuar NULL depois
disso, os logs do Render têm a linha `[RECONCILE]` com o motivo.

---

## 4. Primeiras 24h — nada aqui bloqueia o lançamento

Na ordem em que doem se ficarem sem.

### a) Monitor externo (o mais importante dos três)

O `Monitor Production` do GitHub Actions **não serve de alarme**: o cron pede
30 min, mas a mediana medida entre execuções é de 3h32 (máximo de 13h). Sem um
monitor externo, você descobre uma queda pelo WhatsApp de um cliente.

UptimeRobot ou BetterStack, plano gratuito:

```
URL:        https://fuudelivery-api-8y6l.onrender.com/health
Intervalo:  1–5 min
Alertar se: HTTP != 200
```

⚠️ `starting` responde **200**. Um monitor que só olha o código HTTP não
distingue cold start de banco que nunca conectou. Se der, alerte também quando
o corpo tiver `"status":"starting"` por mais de 5 min seguidos.

### b) `METRICS_TOKEN`

Sem ele o `/metrics` devolve **403** em produção (falha fechada, de propósito).
Não afeta a plataforma — só te impede de ler as métricas.

```bash
openssl rand -hex 32     # cole em METRICS_TOKEN no Render
curl -H "Authorization: Bearer $METRICS_TOKEN" \
     https://fuudelivery-api-8y6l.onrender.com/metrics
```

O que alertar: [`guia-deploy.md`](guia-deploy.md), seção Monitoramento.
Resumo: `reconciliation_healed_total` subindo **não é boa notícia** — significa
que o webhook síncrono falhou e a rede de segurança segurou.

### c) `sql/24` — travas do frete por região

A tabela já é criada sozinha pelo `AutoMigrate`. O que falta sem a migração são
os CHECKs: faixa de CEP invertida, CEP fora de 0–99999999 e frete negativo
passariam a ser aceitos pelo banco.

Cole `sql/24_delivery_region_fees.sql` no SQL Editor (é idempotente) e confira:

```sql
SELECT conname FROM pg_constraint
WHERE conrelid = 'delivery_region_fees'::regclass AND contype = 'c'
ORDER BY conname;
-- esperado, exatamente 2 linhas:
--   delivery_region_fees_faixa_check   -- CEP em 0..99999999 E cep_start <= cep_end
--   delivery_region_fees_fee_check     -- fee >= 0
```

### d) Regiões de frete

**WebAdmin → Regiões.** Enquanto estiver vazio, todo pedido cai na taxa por km
do estabelecimento — nunca em frete grátis. A tela avisa em vermelho.

Desempate: menor prioridade vence; empatou, vence a faixa **mais estreita**.
Isso permite cadastrar a cidade inteira e recortar um bairro caro depois.
Cidade e UF em branco valem para qualquer cidade.

CEP sem cobertura aparece no log do Render:

```
[FRETE] CEP "01310100" sem região cadastrada — caindo na taxa por km do estabelecimento 42
```

---

## Sabido e aceito

- **Cartão desligado.** Só PIX (AbacatePay). Pagar.me, Asaas e Mercado Pago
  estão sem credencial — os adaptadores existem e têm teste, é só configurar
  quando quiser.
- **CEP vem do corpo da requisição.** Dá para mandar CEP barato com endereço
  caro. Detalhes e por que foi aceito: [`seguranca.md`](seguranca.md), seção
  "Riscos aceitos".
- **Pagamento confirmado não aciona o entregador.** O despacho depende de
  alguém chamar `POST /dispatch/trigger` (o app do restaurante). Pedido pago e
  parado há mais de 15 min aparece na métrica `fuudelivery_orders_stuck`.
