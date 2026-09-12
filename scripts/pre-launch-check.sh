#!/usr/bin/env bash
# pre-launch-check.sh — o comando único que responde "posso colocar em produção?"
#
# POR QUE ISTO EXISTE
#
# O que decide o lançamento mora em três lugares e nenhum comando lia os três:
#
#   repositório  build, vet, testes            — o CI olha, mas o CI não vê o banco
#   Render       API de pé, painéis, /metrics  — scripts/verify-deploy.sh já cobre
#   Supabase     travas do schema              — NINGUÉM olhava
#
# A terceira é a que importa. GORM AutoMigrate cria tabela e coluna, mas NÃO
# cria UNIQUE, índice parcial nem CHECK — exatamente as travas que sustentam a
# idempotência do dinheiro. O backend sobe verde com o banco sem elas e só
# descobre no primeiro webhook reenviado, creditando duas vezes.
#
# Tudo aqui é SOMENTE LEITURA: só SELECT, curl e go build. Nada neste script
# escreve em produção.
#
# USO
#   export DB_CONNECTION_STRING="postgresql://..."   # a mesma do backend
#   export METRICS_TOKEN="..."                       # opcional
#   ./scripts/pre-launch-check.sh
#
#   --so-banco   pula repositório e HTTP (útil logo depois de rodar run_all.sh)
#   --so-repo    só a seção A, sem rede
#
# SAÍDA: 0 = PRONTO ou PRONTO COM RESSALVAS · 1 = BLOQUEADO
set -uo pipefail

cd "$(dirname "$0")/.." || exit 1

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; GRAY='\033[0;90m'; BOLD='\033[1m'; NC='\033[0m'

BLOQUEIOS=0
RESSALVAS=0

ok()      { echo -e "  ${GREEN}✔${NC}  $*"; }
bloqueia(){ echo -e "  ${RED}✘${NC}  $*"; BLOQUEIOS=$((BLOQUEIOS + 1)); }
ressalva(){ echo -e "  ${YELLOW}⚠${NC}  $*"; RESSALVAS=$((RESSALVAS + 1)); }
nota()    { echo -e "  ${GRAY}·${NC}  $*"; }
secao()   { echo ""; echo -e "${BOLD}$*${NC}"; }

MODO="tudo"
case "${1:-}" in
    --so-banco) MODO="banco" ;;
    --so-repo)  MODO="repo" ;;
    "") ;;
    *) echo "uso: $0 [--so-banco|--so-repo]" >&2; exit 2 ;;
esac

echo ""
echo "╔══════════════════════════════════════════════════════╗"
echo "║  FuuDelivery — posso colocar em produção?            ║"
echo "╚══════════════════════════════════════════════════════╝"

# ═══ SEÇÃO A — repositório (offline) ══════════════════════════════════════
#
# Falha aqui significa que o que está no disco não é o que você pensa que
# subiu. É a checagem mais barata e a primeira a rodar.
if [ "$MODO" = "tudo" ] || [ "$MODO" = "repo" ]; then
    secao "A · Repositório"

    if ! command -v go >/dev/null 2>&1; then
        ressalva "go não está instalado — seção A não verificada"
    else
        # Os módulos vêm do go.work, não de uma lista fixa: uma lista fixa
        # envelhece calada quando alguém adiciona um módulo, e o script
        # continuaria dando verde sem nunca ter compilado o módulo novo.
        modulos=$(awk '/^use *\(/{f=1;next} /^\)/{f=0} f{gsub(/[ \t]/,"");if($0!="")print}' go.work)
        total=$(echo "$modulos" | grep -c . || true)

        falhas_build=""
        for m in $modulos; do
            if ! (cd "$m" && go build ./... >/dev/null 2>&1); then
                falhas_build="$falhas_build $m"
            fi
        done
        if [ -z "$falhas_build" ]; then
            ok "build: $total módulos do go.work compilam"
        else
            bloqueia "build falhou em:$falhas_build"
        fi

        falhas_vet=""
        for m in $modulos; do
            if ! (cd "$m" && go vet ./... >/dev/null 2>&1); then
                falhas_vet="$falhas_vet $m"
            fi
        done
        if [ -z "$falhas_vet" ]; then
            ok "vet: limpo nos $total módulos"
        else
            bloqueia "go vet reclamou em:$falhas_vet"
        fi

        desalinhados=$(gofmt -l Backend cmd pkg 2>/dev/null | grep -v '/vendor/' || true)
        if [ -z "$desalinhados" ]; then
            ok "gofmt: nada fora de formato"
        else
            ressalva "gofmt: $(echo "$desalinhados" | wc -l) arquivo(s) fora de formato"
        fi
    fi

    # git: o commit que está no disco é o que foi empurrado?
    if git rev-parse --git-dir >/dev/null 2>&1; then
        sujo=$(git status --porcelain | wc -l)
        if [ "$sujo" -eq 0 ]; then
            ok "git: árvore limpa em $(git rev-parse --short HEAD)"
        else
            ressalva "git: $sujo arquivo(s) sem commit — o Render não vai ver essas mudanças"
        fi
    fi
fi

# ═══ SEÇÃO B — Render (HTTP) ══════════════════════════════════════════════
if [ "$MODO" = "tudo" ]; then
    secao "B · Render (API e painéis)"

    # Delegado, não reimplementado: verify-deploy.sh já espera pela CONDIÇÃO no
    # /health (e não pela falha do curl), que foi o bug que deixou o monitor
    # vermelho em 82 execuções seguidas sem nenhuma queda real.
    if bash scripts/verify-deploy.sh >/tmp/pre-launch-http.$$ 2>&1; then
        ok "verify-deploy.sh: API e os dois painéis respondendo"
    else
        bloqueia "verify-deploy.sh falhou — saída abaixo:"
        sed 's/^/      /' /tmp/pre-launch-http.$$ | tail -20
    fi
    rm -f /tmp/pre-launch-http.$$

    # /metrics tem que estar FECHADO. Um 200 sem token significa que o
    # fail-closed não está valendo — ou seja, GO_ENV não é "production" lá, e
    # aí todas as outras proteções que dependem disso (os ValidateWebhook dos
    # adapters de pagamento) também não estão valendo.
    API_BASE="${API_BASE:-https://fuudelivery-api-8y6l.onrender.com}"
    codigo=$(curl -s -o /dev/null -w "%{http_code}" --max-time 30 "$API_BASE/metrics" 2>/dev/null || echo "000")
    case "$codigo" in
        403) ok "/metrics fechado (403 sem token) — GO_ENV=production está valendo" ;;
        200) bloqueia "/metrics respondeu 200 SEM token — GO_ENV não é 'production' em produção;
      isso derruba junto o fail-closed dos webhooks de pagamento" ;;
        000) ressalva "/metrics: sem resposta (timeout) — não verificado" ;;
        *)   ressalva "/metrics devolveu $codigo (esperado 403)" ;;
    esac

    if [ -n "${METRICS_TOKEN:-}" ]; then
        com=$(curl -s -o /dev/null -w "%{http_code}" --max-time 30 \
              -H "Authorization: Bearer $METRICS_TOKEN" "$API_BASE/metrics" 2>/dev/null || echo "000")
        if [ "$com" = "200" ]; then
            ok "/metrics abre com o token — METRICS_TOKEN configurado no Render"
        else
            ressalva "/metrics com o token devolveu $com — o token daqui não é o do Render"
        fi
    else
        nota "METRICS_TOKEN não definido aqui — não dá para provar que o Render tem um"
    fi
fi

# ═══ SEÇÃO C — Supabase (o que ninguém checava) ═══════════════════════════
BANCO_OLHADO="nao"

if [ "$MODO" = "repo" ]; then
    : # --so-repo pediu explicitamente para não tocar em rede nem em banco
else
secao "C · Banco (travas do schema)"

if [ -z "${DB_CONNECTION_STRING:-}" ]; then
    # "Não olhei" NUNCA pode se parecer com "está ok". Foi assim que o monitor
    # ficou vermelho 82 vezes sem ninguém perceber.
    ressalva "NÃO VERIFICADO — DB_CONNECTION_STRING não está definida."
    echo -e "      ${GRAY}Sem isto, as travas que impedem crédito duplicado não foram olhadas.${NC}"
    echo -e "      ${GRAY}export DB_CONNECTION_STRING=\"postgresql://...\"  (a mesma do backend)${NC}"
elif ! command -v psql >/dev/null 2>&1; then
    ressalva "NÃO VERIFICADO — psql não está instalado nesta máquina."
else
    q() { psql "$DB_CONNECTION_STRING" -tAc "$1" 2>/dev/null | tr -d '[:space:]'; }

    if [ "$(q 'SELECT 1')" != "1" ]; then
        bloqueia "não consegui conectar no banco com DB_CONNECTION_STRING"
    else
        # ── travas do dinheiro: cada uma é BLOQUEIO ───────────────────────
        #
        # Estas são as que o AutoMigrate não cria. Sem elas o backend sobe
        # normalmente e a duplicação só aparece com dinheiro real em jogo.
        tem_indice() { [ "$(q "SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname='$1'")" = "1" ]; }

        BANCO_OLHADO="sim"

        if tem_indice uq_wallet_txns_credit_ref; then
            ok "crédito idempotente (uq_wallet_txns_credit_ref)"
        else
            bloqueia "FALTA uq_wallet_txns_credit_ref — webhook reenviado CREDITA DUAS VEZES.
      Corrigir: psql \"\$DB_CONNECTION_STRING\" -f sql/11_idempotencia_financeira.sql"
        fi

        # Débito: os dois nomes são MUTUAMENTE EXCLUSIVOS de propósito.
        # sql/20 apaga o índice global (uq_wallet_txns_debit_ref) e o substitui
        # pelo por-carteira, porque duas carteiras podem legitimamente
        # compartilhar a mesma reference_id de débito. Exigir os dois daria um
        # BLOQUEADO falso num banco em dia — que é o pior defeito possível num
        # verificador.
        if tem_indice uq_wallet_txns_debit_ref_wallet; then
            ok "débito idempotente por carteira (uq_wallet_txns_debit_ref_wallet)"
        elif tem_indice uq_wallet_txns_debit_ref; then
            ressalva "débito protegido pelo índice GLOBAL antigo (uq_wallet_txns_debit_ref).
      Funciona, mas o sql/20 ainda não rodou: aplique sql/20_debit_idempotency_por_wallet.sql"
        else
            bloqueia "FALTA índice de idempotência de débito — saque pode ser processado duas vezes.
      Corrigir: aplique sql/18 e sql/20"
        fi

        if tem_indice uq_payments_abacatepay_id; then
            ok "cobrança única por id do gateway (uq_payments_abacatepay_id)"
        else
            bloqueia "FALTA uq_payments_abacatepay_id — o mesmo pagamento pode virar duas linhas.
      Corrigir: aplique sql/11_idempotencia_financeira.sql"
        fi

        # ── travas do frete por região ────────────────────────────────────
        if [ "$(q "SELECT count(*) FROM pg_class WHERE relname='delivery_region_fees' AND relkind='r'")" != "1" ]; then
            ressalva "tabela delivery_region_fees não existe — o AutoMigrate ainda não rodou nesta base"
        else
            checks=$(q "SELECT count(*) FROM pg_constraint WHERE conrelid='delivery_region_fees'::regclass AND contype='c'")
            if [ "${checks:-0}" -ge 2 ]; then
                ok "travas do frete por região ($checks CHECKs)"
            else
                ressalva "delivery_region_fees sem os 2 CHECKs (tem ${checks:-0}) — faixa de CEP invertida
      e frete negativo passariam. Corrigir: aplique sql/24_delivery_region_fees.sql"
            fi

            regioes=$(q "SELECT count(*) FROM delivery_region_fees")
            if [ "${regioes:-0}" -gt 0 ]; then
                ok "$regioes região(ões) de frete cadastrada(s)"
            else
                ressalva "nenhuma região cadastrada — todo pedido cai na taxa por km do estabelecimento.
      Cadastrar em WebAdmin → Regiões (não bloqueia o lançamento)"
            fi
        fi

        # ── estado operacional do dinheiro ────────────────────────────────
        #
        # Não é sobre schema: é sobre dinheiro parado AGORA. A reconciliação
        # roda a cada 5 min; o que sobrar depois de 10 min é caso real.
        pendurado=$(q "SELECT count(*) FROM payments
                       WHERE status='CONFIRMED'
                         AND establishment_credited_at IS NULL
                         AND confirmed_at < now() - interval '10 minutes'")
        if [ "${pendurado:-0}" -eq 0 ]; then
            ok "nenhum pagamento confirmado sem liquidar"
        else
            bloqueia "$pendurado pagamento(s) CONFIRMED sem crédito no restaurante há mais de 10 min.
      A reconciliação não está dando conta. Logs do Render, linha [RECONCILE]"
        fi

        # Mesma consulta de contarPedidosParados() em reconciliation.go.
        # Falha em silêncio quando orders/batches não existem nesta base.
        parados=$(q "SELECT count(*) FROM payments p
                     LEFT JOIN orders o ON o.id::text = p.order_id
                     LEFT JOIN batches b ON b.id = o.batch_id
                     WHERE p.status='CONFIRMED'
                       AND p.confirmed_at < now() - interval '15 minutes'
                       AND p.confirmed_at > now() - interval '7 days'
                       AND (o.id IS NULL OR o.batch_id IS NULL OR b.courier_id IS NULL)")
        if [ -z "$parados" ]; then
            nota "pedidos parados: não consultável nesta base (orders/batches ausentes)"
        elif [ "$parados" -eq 0 ]; then
            ok "nenhum pedido pago parado sem entregador"
        else
            ressalva "$parados pedido(s) pago(s) há mais de 15 min sem entregador.
      O despacho depende de POST /dispatch/trigger pelo app do restaurante"
        fi

        # ── informativo: até onde a suíte de migrações chegou ─────────────
        #
        # Deliberadamente NÃO é bloqueio. Um banco migrado antes de d770f4b
        # tem 22 linhas em vez de 24 (sql/09 e sql/19 não se registravam), e
        # reprovar por causa disso seria reprovar um banco perfeito. Quem
        # decide são as travas acima; isto aqui só situa.
        aplicadas=$(q "SELECT count(*) FROM schema_migrations")
        if [ -n "$aplicadas" ]; then
            ultima=$(q "SELECT max(version) FROM schema_migrations")
            nota "schema_migrations: $aplicadas registro(s), última '$ultima'"
        else
            ressalva "schema_migrations não existe — a suíte sql/ nunca rodou nesta base.
      As travas acima vieram de outro lugar (ou não vieram)"
        fi
    fi
fi
fi

# ═══ VEREDITO ═════════════════════════════════════════════════════════════
secao "Veredito"

if [ "$BLOQUEIOS" -gt 0 ]; then
    echo -e "  ${RED}${BOLD}BLOQUEADO${NC} — $BLOQUEIOS item(ns) impedem o lançamento"
    [ "$RESSALVAS" -gt 0 ] && echo -e "  ${GRAY}(e $RESSALVAS ressalva(s))${NC}"
    veredito=1
elif [ "$RESSALVAS" -gt 0 ]; then
    echo -e "  ${YELLOW}${BOLD}PRONTO COM RESSALVAS${NC} — $RESSALVAS item(ns) para olhar, nenhum bloqueia"
    veredito=0
elif [ "$BANCO_OLHADO" != "sim" ]; then
    # PRONTO é uma afirmação sobre o dinheiro, e o dinheiro está no banco. Sem
    # a seção C ter rodado de verdade, o mais que este script pode dizer é "o
    # que eu olhei está de pé" — nunca "pode lançar". A garantia é estrutural
    # de propósito: depender de alguém ter lembrado de somar uma ressalva em
    # cada caminho que pula o banco é como se perde uma invariante dessas.
    echo -e "  ${YELLOW}${BOLD}PARCIAL${NC} — o que foi verificado está de pé, mas o BANCO não foi olhado"
    echo -e "  ${GRAY}As travas que impedem crédito duplicado não foram conferidas.${NC}"
    echo -e "  ${GRAY}Rode de novo com DB_CONNECTION_STRING definida.${NC}"
    veredito=0
else
    echo -e "  ${GREEN}${BOLD}PRONTO${NC} — tudo que este script consegue verificar está de pé"
    veredito=0
fi

# O que nenhum script consegue ver. Sai SEMPRE, inclusive no PRONTO: uma chave
# rotacionada é indistinguível de uma não rotacionada para quem olha de fora, e
# omitir isto aqui faria o verde parecer mais completo do que é.
echo ""
echo -e "  ${GRAY}Não verificável daqui — confira você:${NC}"
echo -e "  ${GRAY}· as chaves do Supabase foram rotacionadas? (docs/runbook-rotacao-credenciais.md)${NC}"
echo -e "  ${GRAY}· o teste de fumaça do dinheiro foi feito? (docs/LANCAR.md §3)${NC}"
echo -e "  ${GRAY}· existe monitor externo no /health? (docs/LANCAR.md §4a)${NC}"
echo ""

exit $veredito
