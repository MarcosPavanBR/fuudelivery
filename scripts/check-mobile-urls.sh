#!/usr/bin/env bash
# =============================================================
# check-mobile-urls.sh — Verificação de consistência das URLs dos apps mobile
# =============================================================
# Objetivo: garantir que a URL da API usada pelos apps mobile esteja
# sincronizada com a referência canônica (references/URLS.md).
#
# Problema que previne: a URL de produção vivia em 5 arquivos por app
# (eas.json x2, helpers, api.tsx, LiveTrackingReadonly). Se o sufixo do
# Render mudasse, era fácil atualizar um e esquecer outro. Após a
# centralização, a fonte única é `config/api.ts` — mas este script
# garante que nada divergiu (e que ninguém reintroduziu hardcodes).
#
# Uso: bash scripts/check-mobile-urls.sh
# Exit code: 0 (consistente) | 1 (divergência encontrada — falha no CI)
#
# Integrado ao CI: .github/workflows/ci.yml (job `check-mobile-urls`)
# =============================================================
set -euo pipefail

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
ok()   { echo -e "${GREEN}✔${NC}  $*"; }
fail() { echo -e "${RED}✘${NC}  $*"; }
warn() { echo -e "${YELLOW}⚠${NC}  $*"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
URLS_MD="$ROOT_DIR/references/URLS.md"

echo ""
echo "╔══════════════════════════════════════════════════╗"
echo "║  FuuDelivery — Check Mobile API URL Consistency  ║"
echo "╚══════════════════════════════════════════════════╝"
echo ""

FAILURES=0

# ─── 1. Extrai a URL canônica do references/URLS.md ──────────
# A seção "Apps Mobile" lista a URL na linha do AppComida.
# Formato esperado:
#   | **AppComida** (cliente) | `https://...onrender.com` | `config/api.ts` ...
CANONICAL=$(sed -n 's/.*\*\*AppComida\*\*.*| `\(https:\/\/[^`]*\)`.*/\1/p' "$URLS_MD" | head -1)

if [ -z "$CANONICAL" ]; then
    fail "Não foi possível extrair a URL canônica de references/URLS.md"
    fail "Verifique se a linha do AppComida segue o formato:"
    fail "  | **AppComida** (cliente) | \`https://...\` | \`config/api.ts\` ..."
    exit 1
fi
ok "URL canônica (URLS.md): $CANONICAL"

# ─── 2. Verifica o config/api.ts de cada app ─────────────────
# A fonte única centralizada. O valor de API_URL deve ser idêntico.
check_config() {
    local app_name="$1"
    local config_file="$2"
    local file_url

    if [ ! -f "$config_file" ]; then
        fail "$app_name: $config_file não encontrado"
        FAILURES=$((FAILURES + 1))
        return
    fi

    # Suporta tanto API_URL hardcoded quanto EXPO_PUBLIC_API_URL via env
    file_url=$(sed -n 's/.*API_URL = "\([^"]*\)".*/\1/p; s/.*EXPO_PUBLIC_API_URL.*"\(https:\/\/[^"]*\)".*/\1/p' "$config_file" | head -1)
    if [ -z "$file_url" ]; then
        # Se não encontrar URL hardcoded, verifica se usa EXPO_PUBLIC_API_URL
        if grep -q "EXPO_PUBLIC_API_URL" "$config_file"; then
            ok "$app_name: config/api.ts usa EXPO_PUBLIC_API_URL (URL definida no build)"
        else
            fail "$app_name: API_URL/EXPO_PUBLIC_API_URL não encontrada em $config_file"
            FAILURES=$((FAILURES + 1))
            return
        fi
    else
        if [ "$file_url" = "$CANONICAL" ]; then
            ok "$app_name: config/api.ts OK ($file_url)"
        else
            fail "$app_name: DIVERGÊNCIA — config/api.ts tem '$file_url', URLS.md diz '$CANONICAL'"
            FAILURES=$((FAILURES + 1))
        fi
    fi
}

check_config "AppComida"  "$ROOT_DIR/Frontend/AppComida/config/api.ts"
check_config "AppEntrega" "$ROOT_DIR/Frontend/AppEntrega/config/api.ts"
check_config "AppRestaurante" "$ROOT_DIR/Frontend/AppRestaurante/config/api.ts"

# ─── 3. Varredura de URLs hardcoded divergentes ──────────────
# Qualquer referência a onrender.com nos sources dos apps deve ser a
# canônica (ou derivada dela via getWsUrl). Hardcodes antigos são pegos aqui.
scan_app() {
    local app_name="$1"
    local app_dir="$2"

    if [ ! -d "$app_dir" ]; then
        warn "$app_name: diretório não encontrado ($app_dir) — pulando varredura"
        return
    fi

    # Arquivos-fonte e configs (exclui node_modules, lockfiles e o próprio config central)
    local matches
    matches=$(grep -rInoE 'https?://[a-z0-9.-]*\.onrender\.com' "$app_dir" \
        --include='*.ts' --include='*.tsx' --include='*.json' --include='*.js' --include='*.jsx' \
        --exclude-dir=node_modules --exclude-dir=.expo --exclude-dir=dist --exclude-dir=android \
        --exclude-dir=ios --exclude-dir=.git \
        2>/dev/null | grep -v 'config/api.ts' || true)

    if [ -z "$matches" ]; then
        ok "$app_name: sem URLs hardcoded em fontes (tudo centralizado)"
        return
    fi

    # Compara cada ocorrência com a canônica (por host — ignora wss/http)
    while IFS= read -r line; do
        url=$(echo "$line" | grep -oE 'https?://[a-z0-9.-]*\.onrender\.com')
        file=$(echo "$line" | cut -d: -f1-2)
        if [ "$url" != "$CANONICAL" ]; then
            fail "$app_name: URL divergente em $file → $url (esperado: $CANONICAL)"
            FAILURES=$((FAILURES + 1))
        else
            ok "$app_name: $file usa URL canônica"
        fi
    done <<< "$matches"
}

scan_app "AppComida"  "$ROOT_DIR/Frontend/AppComida"
scan_app "AppEntrega" "$ROOT_DIR/Frontend/AppEntrega"
scan_app "AppRestaurante" "$ROOT_DIR/Frontend/AppRestaurante"

# ─── Resultado ───────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════════"
if [ "$FAILURES" -eq 0 ]; then
    echo -e "${GREEN}Todas as URLs dos apps mobile estão consistentes!${NC}"
else
    echo -e "${RED}$FAILURES divergência(s) encontrada(s) — corrija antes do merge${NC}"
fi
echo "══════════════════════════════════════════════════════"
echo ""

exit $FAILURES
