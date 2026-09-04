#!/bin/bash
set -euo pipefail

# ============================================================
# FUUDELIVERY - PRODUCTION READINESS AUDIT SYSTEM
# ============================================================
# Uso: bash scripts/audit.sh   (a partir da raiz do repo)
# Gera audit-reports/MASTER_REPORT_<timestamp>.md e logs por fase.
# Audita um clone fresco do repo (./fuudelivery) — segredos locais
# do working tree nao entram na auditoria. audit-reports/ e ignorado
# pelo .gitignore.
# ============================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

REPORT_DIR="./audit-reports"
mkdir -p "$REPORT_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
MASTER_REPORT="$REPORT_DIR/MASTER_REPORT_$TIMESTAMP.md"

log() { echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"; }
success() { echo -e "${GREEN}[$(date '+%H:%M:%S')] ✅ $1${NC}"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] ⚠️  $1${NC}"; }
error() { echo -e "${RED}[$(date '+%H:%M:%S')] ❌ $1${NC}"; }

echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║     FUUDELIVERY - PRODUCTION READINESS AUDIT SYSTEM         ║${NC}"
echo -e "${CYAN}║     Versão 2.0 | $(date '+%Y-%m-%d %H:%M:%S')                           ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Inicializar relatório master
cat > "$MASTER_REPORT" << EOF
# 🏥 FUUDELIVERY - RELATÓRIO DE AUDITORIA DE PRODUÇÃO

**Data:** $(date '+%Y-%m-%d %H:%M:%S')
**Repositório:** https://github.com/MarcosPavanBR/fuudelivery
**Auditor:** Sistema Automatizado v2.0

---

EOF

# ============================================================
# FASE 1: AQUISIÇÃO DO CÓDIGO
# ============================================================
log "FASE 1: AQUISIÇÃO DO CÓDIGO"
echo "## 📦 FASE 1: AQUISIÇÃO DO CÓDIGO" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

REPO_DIR="fuudelivery"
REPO_URL="https://github.com/MarcosPavanBR/fuudelivery.git"

if [ -d "$REPO_DIR" ]; then
    warn "Repositório já existe. Removendo para clone limpo..."
    rm -rf "$REPO_DIR"
fi

log "Clonando repositório..."
if git clone --depth=50 "$REPO_URL" "$REPO_DIR" 2>&1; then
    success "Repositório clonado com sucesso"
    echo "- ✅ Repositório clonado" >> "$MASTER_REPORT"
else
    error "Falha ao clonar repositório"
    echo "- ❌ FALHA ao clonar repositório" >> "$MASTER_REPORT"
    exit 1
fi

cd "$REPO_DIR"

# Capturar informações do git
GIT_BRANCH=$(git branch --show-current 2>/dev/null || echo "unknown")
GIT_COMMIT=$(git log -1 --format="%H" 2>/dev/null || echo "unknown")
GIT_COMMIT_SHORT=$(git log -1 --format="%h" 2>/dev/null || echo "unknown")
GIT_DATE=$(git log -1 --format="%ci" 2>/dev/null || echo "unknown")
GIT_AUTHOR=$(git log -1 --format="%an" 2>/dev/null || echo "unknown")

echo "- **Branch:** \`$GIT_BRANCH\`" >> "$MASTER_REPORT"
echo "- **Commit:** \`$GIT_COMMIT_SHORT\` ($GIT_COMMIT)" >> "$MASTER_REPORT"
echo "- **Data do commit:** $GIT_DATE" >> "$MASTER_REPORT"
echo "- **Autor:** $GIT_AUTHOR" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

success "Informações do Git capturadas"
echo ""

# ============================================================
# FASE 2: VERIFICAÇÃO DE PRÉ-REQUISITOS
# ============================================================
log "FASE 2: VERIFICAÇÃO DE PRÉ-REQUISITOS"
echo "## 🔍 FASE 2: PRÉ-REQUISITOS DO SISTEMA" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"
echo "| Ferramenta | Status | Versão |" >> "$MASTER_REPORT"
echo "|------------|--------|--------|" >> "$MASTER_REPORT"

check_tool() {
    local tool=$1
    local min_version=$2
    local required=$3

    if command -v "$tool" &> /dev/null; then
        local version=$("$tool" --version 2>/dev/null | head -1 || echo "installed")
        echo "| $tool | ✅ Instalado | $version |" >> "$MASTER_REPORT"
        success "$tool: $version"
        return 0
    else
        if [ "$required" = "required" ]; then
            echo "| $tool | ❌ NÃO INSTALADO | - |" >> "$MASTER_REPORT"
            error "$tool não instalado (OBRIGATÓRIO)"
        else
            echo "| $tool | ⚠️  Não instalado | - |" >> "$MASTER_REPORT"
            warn "$tool não instalado (opcional)"
        fi
        return 1
    fi
}

check_tool "go" "1.25" "required" || true
check_tool "node" "20" "required" || true
check_tool "npm" "10" "required" || true
check_tool "git" "2.0" "required" || true
check_tool "docker" "20" "optional" || true
check_tool "docker-compose" "2.0" "optional" || true

echo "" >> "$MASTER_REPORT"
echo ""
# ============================================================
# FASE 3: ANÁLISE ESTRUTURAL DO PROJETO
# ============================================================
log "FASE 3: ANÁLISE ESTRUTURAL"
echo "## 📂 FASE 3: ANÁLISE ESTRUTURAL" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

# Contar arquivos por tipo
GO_FILES=$(find . -name "*.go" -not -path "./vendor/*" 2>/dev/null | wc -l)
TS_FILES=$(find . -name "*.ts" -o -name "*.tsx" -not -path "./node_modules/*" 2>/dev/null | wc -l)
JS_FILES=$(find . -name "*.js" -o -name "*.jsx" -not -path "./node_modules/*" 2>/dev/null | wc -l)
SQL_FILES=$(find . -name "*.sql" 2>/dev/null | wc -l)
TEST_FILES=$(find . -name "*_test.go" 2>/dev/null | wc -l)

echo "### Contagem de Arquivos" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"
echo "| Tipo | Quantidade |" >> "$MASTER_REPORT"
echo "|------|------------|" >> "$MASTER_REPORT"
echo "| Go (.go) | $GO_FILES |" >> "$MASTER_REPORT"
echo "| TypeScript (.ts/.tsx) | $TS_FILES |" >> "$MASTER_REPORT"
echo "| JavaScript (.js/.jsx) | $JS_FILES |" >> "$MASTER_REPORT"
echo "| SQL (.sql) | $SQL_FILES |" >> "$MASTER_REPORT"
echo "| Testes Go (*_test.go) | $TEST_FILES |" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

success "Análise estrutural: $GO_FILES Go, $TS_FILES TS, $TEST_FILES testes"

# Verificar arquivos críticos
echo "### Arquivos Críticos" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

CRITICAL_FILES=(
    "go.work"
    "go.mod"
    "Dockerfile"
    "docker-compose.vps.yml"
    ".env.example"
    "README.md"
    "MANIFEST.md"
    "PRODUCTION.md"
    "CONTRIBUTING.md"
    "SECURITY.md"
    "render.yaml"
    "Procfile"
)

for file in "${CRITICAL_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "- ✅ \`$file\`" >> "$MASTER_REPORT"
    else
        echo "- ❌ \`$file\` **NÃO ENCONTRADO**" >> "$MASTER_REPORT"
        warn "Arquivo crítico faltando: $file"
    fi
done

echo "" >> "$MASTER_REPORT"
echo ""

# ============================================================
# FASE 4: BACKEND GO - INSTALAÇÃO E ANÁLISE
# ============================================================
log "FASE 4: BACKEND GO"
echo "## ⚙️ FASE 4: BACKEND GO" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

# Verificar go.work
if [ -f "go.work" ]; then
    success "go.work encontrado"
    echo "### go.work" >> "$MASTER_REPORT"
    echo '```' >> "$MASTER_REPORT"
    cat go.work >> "$MASTER_REPORT"
    echo '```' >> "$MASTER_REPORT"
    echo "" >> "$MASTER_REPORT"
else
    warn "go.work não encontrado"
fi

# Verificar go.mod principal
if [ -f "go.mod" ]; then
    GO_VERSION=$(grep "^go " go.mod | awk '{print $2}')
    MODULE_NAME=$(grep "^module " go.mod | awk '{print $2}')
    echo "- **Módulo:** \`$MODULE_NAME\`" >> "$MASTER_REPORT"
    echo "- **Versão Go:** $GO_VERSION" >> "$MASTER_REPORT"
fi

# Instalar dependências
log "Instalando dependências Go..."
echo "### Instalação de Dependências" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

if go mod tidy 2>&1 | tee -a "$REPORT_DIR/go_mod_tidy.log"; then
    success "go mod tidy executado"
    echo "- ✅ \`go mod tidy\` executado com sucesso" >> "$MASTER_REPORT"
else
    error "go mod tidy falhou"
    echo "- ❌ \`go mod tidy\` FALHOU" >> "$MASTER_REPORT"
fi

if go mod download 2>&1; then
    success "Dependências Go baixadas"
    echo "- ✅ Dependências baixadas" >> "$MASTER_REPORT"
else
    error "Falha ao baixar dependências"
    echo "- ❌ Falha ao baixar dependências" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"

# Análise de dependências
echo "### Dependências Principais" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"
echo "| Dependência | Versão |" >> "$MASTER_REPORT"
echo "|-------------|--------|" >> "$MASTER_REPORT"

grep -E "github.com/gofiber|gorm.io|redis|jwt|websocket|google/uuid" go.mod 2>/dev/null | while read -r line; do
    DEP=$(echo "$line" | awk '{print $1}')
    VER=$(echo "$line" | awk '{print $2}')
    echo "| \`$DEP\` | $VER |" >> "$MASTER_REPORT"
done

echo "" >> "$MASTER_REPORT"

# ============================================================
# FASE 5: ANÁLISE DE CÓDIGO GO (VET + LINT)
# ============================================================
log "FASE 5: ANÁLISE ESTÁTICA DE CÓDIGO"
echo "## 🔬 FASE 5: ANÁLISE ESTÁTICA" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

# go vet
log "Executando go vet..."
if go vet ./... 2>&1 | tee -a "$REPORT_DIR/go_vet.log"; then
    VET_ISSUES=$(wc -l < "$REPORT_DIR/go_vet.log" 2>/dev/null || echo "0")
    if [ "$VET_ISSUES" -eq 0 ]; then
        success "go vet: nenhum problema encontrado"
        echo "- ✅ \`go vet\`: 0 problemas" >> "$MASTER_REPORT"
    else
        warn "go vet: $VET_ISSUES problemas encontrados"
        echo "- ⚠️  \`go vet\`: $VET_ISSUES problemas" >> "$MASTER_REPORT"
    fi
else
    error "go vet encontrou problemas"
    echo "- ❌ \`go vet\`: problemas encontrados" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"

# Verificar erros de compilação
log "Verificando compilação..."
echo "### Compilação" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

if go build ./... 2>&1 | tee -a "$REPORT_DIR/go_build.log"; then
    success "Código compila sem erros"
    echo "- ✅ Compilação: SUCESSO" >> "$MASTER_REPORT"
else
    error "Erros de compilação encontrados"
    echo "- ❌ Compilação: FALHOU" >> "$MASTER_REPORT"
    echo "" >> "$MASTER_REPORT"
    echo '```' >> "$MASTER_REPORT"
    cat "$REPORT_DIR/go_build.log" >> "$MASTER_REPORT"
    echo '```' >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"
echo ""
# ============================================================
# FASE 6: TESTES GO
# ============================================================
log "FASE 6: TESTES GO"
echo "## 🧪 FASE 6: TESTES GO" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

if [ "$TEST_FILES" -gt 0 ]; then
    log "Executando testes Go ($TEST_FILES arquivos de teste)..."

    # Executar testes com coverage
    if go test ./... -v -coverprofile="$REPORT_DIR/coverage.out" -timeout 300s 2>&1 | tee -a "$REPORT_DIR/go_test.log"; then
        TEST_PASSED=$(grep -c "^--- PASS" "$REPORT_DIR/go_test.log" 2>/dev/null || echo "0")
        TEST_FAILED=$(grep -c "^--- FAIL" "$REPORT_DIR/go_test.log" 2>/dev/null || echo "0")

        success "Testes executados: $TEST_PASSED passaram, $TEST_FAILED falharam"
        echo "- **Testes aprovados:** $TEST_PASSED" >> "$MASTER_REPORT"
        echo "- **Testes reprovados:** $TEST_FAILED" >> "$MASTER_REPORT"

        # Gerar relatório de coverage
        if [ -f "$REPORT_DIR/coverage.out" ]; then
            COVERAGE=$(go tool cover -func="$REPORT_DIR/coverage.out" 2>/dev/null | tail -1 | awk '{print $NF}' || echo "N/A")
            echo "- **Cobertura total:** $COVERAGE" >> "$MASTER_REPORT"

            # Gerar HTML de coverage
            go tool cover -html="$REPORT_DIR/coverage.out" -o "$REPORT_DIR/coverage.html" 2>/dev/null || true
            success "Relatório de coverage gerado"
        fi
    else
        error "Testes falharam"
        echo "- ❌ Testes: FALHA" >> "$MASTER_REPORT"
    fi
else
    warn "Nenhum arquivo de teste Go encontrado"
    echo "- ⚠️  **Nenhum teste Go encontrado**" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"
echo ""

# ============================================================
# FASE 7: FRONTEND - INSTALAÇÃO E BUILD
# ============================================================
log "FASE 7: FRONTEND"
echo "## 🎨 FASE 7: FRONTEND" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

FRONTEND_DIRS=("Frontend/WebRestaurant" "Frontend/WebAdmin")

for dir in "${FRONTEND_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        log "Processando $(basename $dir)..."
        echo "### $(basename $dir)" >> "$MASTER_REPORT"
        echo "" >> "$MASTER_REPORT"

        cd "$dir"

        # Verificar package.json
        if [ -f "package.json" ]; then
            REACT_VER=$(grep '"react"' package.json 2>/dev/null | awk -F'"' '{print $4}' || echo "N/A")
            VITE_VER=$(grep '"vite"' package.json 2>/dev/null | awk -F'"' '{print $4}' || echo "N/A")
            echo "- **React:** $REACT_VER" >> "$MASTER_REPORT"
            echo "- **Vite:** $VITE_VER" >> "$MASTER_REPORT"
        fi

        # Instalar dependências
        rm -rf node_modules package-lock.json 2>/dev/null || true

        if npm install --legacy-peer-deps --silent 2>&1 | tee -a "$REPORT_DIR/npm_install_$(basename $dir).log"; then
            success "$(basename $dir): dependências instaladas"
            echo "- ✅ Dependências instaladas" >> "$MASTER_REPORT"
        else
            error "$(basename $dir): falha na instalação"
            echo "- ❌ Falha na instalação" >> "$MASTER_REPORT"
        fi

        # Tentar build
        if npm run build 2>&1 | tee -a "$REPORT_DIR/build_$(basename $dir).log"; then
            success "$(basename $dir): build bem-sucedido"
            echo "- ✅ Build: SUCESSO" >> "$MASTER_REPORT"
        else
            warn "$(basename $dir): build falhou (pode precisar de variáveis de ambiente)"
            echo "- ⚠️  Build: FALHOU (verificar variáveis de ambiente)" >> "$MASTER_REPORT"
        fi

        echo "" >> "$MASTER_REPORT"
        cd ../..
    fi
done

echo ""
# ============================================================
# FASE 8: ANÁLISE DE SEGURANÇA
# ============================================================
log "FASE 8: ANÁLISE DE SEGURANÇA"
echo "## 🔒 FASE 8: ANÁLISE DE SEGURANÇA" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

# Verificar segredos hardcoded
log "Verificando segredos hardcoded..."
SECRETS_FOUND=0

SECRET_PATTERNS=(
    "password.*=.*['\"][^'\"]+['\"]"
    "secret.*=.*['\"][^'\"]+['\"]"
    "api_key.*=.*['\"][^'\"]+['\"]"
    "token.*=.*['\"][^'\"]+['\"]"
    "postgres://[^:]+:[^@]+@"
    "mongodb://[^:]+:[^@]+@"
)

echo "### Verificação de Segredos Hardcoded" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

for pattern in "${SECRET_PATTERNS[@]}"; do
    MATCHES=$(grep -riE "$pattern" --include="*.go" --include="*.ts" --include="*.tsx" --include="*.js" --include="*.jsx" . 2>/dev/null | grep -v node_modules | grep -v ".env.example" | grep -v "_test.go" | wc -l)
    if [ "$MATCHES" -gt 0 ]; then
        SECRETS_FOUND=$((SECRETS_FOUND + MATCHES))
        echo "- ⚠️  Padrão \`$pattern\`: $MATCHES ocorrências" >> "$MASTER_REPORT"
    fi
done

if [ "$SECRETS_FOUND" -eq 0 ]; then
    success "Nenhum segredo hardcoded encontrado"
    echo "- ✅ Nenhum segredo hardcoded detectado" >> "$MASTER_REPORT"
else
    warn "$SECRETS_FOUND possíveis segredos hardcoded encontrados"
    echo "- ⚠️  **$SECRETS_FOUND possíveis segredos hardcoded**" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"

# Verificar .env
echo "### Arquivo .env" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

if [ -f ".env" ]; then
    echo "- ⚠️  Arquivo \`.env\` existe (não deve ser commitado)" >> "$MASTER_REPORT"
    warn ".env existe localmente"
else
    echo "- ✅ Arquivo \`.env\` não existe (correto)" >> "$MASTER_REPORT"
fi

if [ -f ".env.example" ]; then
    ENV_VARS=$(grep -c "^[A-Z_]*=" .env.example 2>/dev/null || echo "0")
    echo "- ✅ \`.env.example\` existe com $ENV_VARS variáveis" >> "$MASTER_REPORT"
else
    echo "- ❌ \`.env.example\` NÃO existe" >> "$MASTER_REPORT"
fi

if [ -f ".gitignore" ]; then
    if grep -q ".env" .gitignore; then
        echo "- ✅ \`.env\` está no \`.gitignore\`" >> "$MASTER_REPORT"
    else
        echo "- ⚠️  \`.env\` NÃO está no \`.gitignore\`" >> "$MASTER_REPORT"
    fi
fi

echo "" >> "$MASTER_REPORT"

# Verificar CORS e segurança HTTP
echo "### Segurança HTTP" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

CORS_FILES=$(grep -rl "cors" --include="*.go" . 2>/dev/null | wc -l)
RATE_LIMIT_FILES=$(grep -rl "rate.*limit\|limiter" --include="*.go" . 2>/dev/null | wc -l)
HTTPS_FILES=$(grep -rl "https\|tls\|ssl" --include="*.go" . 2>/dev/null | wc -l)

echo "- Menções a CORS: $CORS_FILES arquivos" >> "$MASTER_REPORT"
echo "- Menções a Rate Limit: $RATE_LIMIT_FILES arquivos" >> "$MASTER_REPORT"
echo "- Menções a HTTPS/TLS: $HTTPS_FILES arquivos" >> "$MASTER_REPORT"

echo "" >> "$MASTER_REPORT"
echo ""

# ============================================================
# FASE 9: DOCKER E DEPLOY
# ============================================================
log "FASE 9: DOCKER E DEPLOY"
echo "## 🐳 FASE 9: DOCKER E DEPLOY" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

if [ -f "Dockerfile" ]; then
    success "Dockerfile encontrado"
    echo "### Dockerfile" >> "$MASTER_REPORT"
    echo "" >> "$MASTER_REPORT"

    # Analisar Dockerfile
    BASE_IMAGE=$(grep "^FROM" Dockerfile | head -1)
    echo "- **Imagem base:** \`$BASE_IMAGE\`" >> "$MASTER_REPORT"

    # Verificar multi-stage build
    FROM_COUNT=$(grep -c "^FROM" Dockerfile)
    if [ "$FROM_COUNT" -gt 1 ]; then
        echo "- ✅ Multi-stage build ($FROM_COUNT estágios)" >> "$MASTER_REPORT"
    else
        echo "- ⚠️  Build de único estágio" >> "$MASTER_REPORT"
    fi

    # Verificar usuário não-root
    if grep -q "USER" Dockerfile; then
        echo "- ✅ Usuário não-root configurado" >> "$MASTER_REPORT"
    else
        echo "- ⚠️  Nenhum usuário não-root configurado" >> "$MASTER_REPORT"
    fi

    # Verificar healthcheck
    if grep -q "HEALTHCHECK" Dockerfile; then
        echo "- ✅ Healthcheck configurado" >> "$MASTER_REPORT"
    else
        echo "- ⚠️  Nenhum healthcheck configurado" >> "$MASTER_REPORT"
    fi
else
    error "Dockerfile não encontrado"
    echo "- ❌ **Dockerfile NÃO encontrado**" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"

# Verificar docker-compose
for compose_file in "docker-compose.yml" "docker-compose.vps.yml" "docker-compose.yaml"; do
    if [ -f "$compose_file" ]; then
        success "$compose_file encontrado"
        SERVICES=$(grep -c "^  [a-z]" "$compose_file" 2>/dev/null || echo "0")
        echo "- ✅ \`$compose_file\` encontrado ($SERVICES serviços)" >> "$MASTER_REPORT"
    fi
done

echo "" >> "$MASTER_REPORT"

# Verificar CI/CD
echo "### CI/CD" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

if [ -d ".github/workflows" ]; then
    WORKFLOW_COUNT=$(ls -1 .github/workflows/*.yml .github/workflows/*.yaml 2>/dev/null | wc -l)
    echo "- ✅ GitHub Actions: $WORKFLOW_COUNT workflows" >> "$MASTER_REPORT"

    for workflow in .github/workflows/*.yml .github/workflows/*.yaml; do
        if [ -f "$workflow" ]; then
            echo "  - \`$(basename $workflow)\`" >> "$MASTER_REPORT"
        fi
    done
else
    echo "- ⚠️  Nenhum workflow CI/CD encontrado" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"
echo ""
# ============================================================
# FASE 10: MÉTRICAS E DOCUMENTAÇÃO
# ============================================================
log "FASE 10: MÉTRICAS E DOCUMENTAÇÃO"
echo "## 📊 FASE 10: MÉTRICAS E DOCUMENTAÇÃO" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

# Tamanho do código
TOTAL_LINES=$(find . -name "*.go" -o -name "*.ts" -o -name "*.tsx" -not -path "./node_modules/*" -not -path "./vendor/*" 2>/dev/null | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
GO_LINES=$(find . -name "*.go" -not -path "./vendor/*" 2>/dev/null | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')

echo "### Métricas de Código" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"
echo "| Métrica | Valor |" >> "$MASTER_REPORT"
echo "|---------|-------|" >> "$MASTER_REPORT"
echo "| Linhas totais (Go + TS) | ${TOTAL_LINES:-0} |" >> "$MASTER_REPORT"
echo "| Linhas Go | ${GO_LINES:-0} |" >> "$MASTER_REPORT"
echo "| Arquivos Go | $GO_FILES |" >> "$MASTER_REPORT"
echo "| Arquivos de teste | $TEST_FILES |" >> "$MASTER_REPORT"

if [ "$TEST_FILES" -gt 0 ] && [ "$GO_FILES" -gt 0 ]; then
    TEST_RATIO=$((TEST_FILES * 100 / GO_FILES))
    echo "| Ratio teste/código | ${TEST_RATIO}% |" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"

# Documentação
echo "### Documentação" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

DOC_FILES=("README.md" "MANIFEST.md" "PRODUCTION.md" "CONTRIBUTING.md" "SECURITY.md" "CHANGELOG.md")
for doc in "${DOC_FILES[@]}"; do
    if [ -f "$doc" ]; then
        DOC_SIZE=$(wc -c < "$doc")
        echo "- ✅ \`$doc\` (${DOC_SIZE} bytes)" >> "$MASTER_REPORT"
    else
        echo "- ❌ \`$doc\`" >> "$MASTER_REPORT"
    fi
done

echo "" >> "$MASTER_REPORT"
echo ""

# ============================================================
# FASE 11: VERIFICAÇÃO DE PADRÕES DE PRODUÇÃO
# ============================================================
log "FASE 11: PADRÕES DE PRODUÇÃO"
echo "## 🏭 FASE 11: PADRÕES DE PRODUÇÃO" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

echo "### Checklist de Produção" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"
echo "| Item | Status | Observação |" >> "$MASTER_REPORT"
echo "|------|--------|------------|" >> "$MASTER_REPORT"

# Graceful shutdown
if grep -rq "signal.Notify\|SIGTERM\|graceful" --include="*.go" . 2>/dev/null; then
    echo "| Graceful Shutdown | ✅ | Implementado |" >> "$MASTER_REPORT"
else
    echo "| Graceful Shutdown | ❌ | Não encontrado |" >> "$MASTER_REPORT"
fi

# Health checks
if grep -rq "health\|/health\|healthcheck" --include="*.go" . 2>/dev/null; then
    echo "| Health Checks | ✅ | Implementado |" >> "$MASTER_REPORT"
else
    echo "| Health Checks | ❌ | Não encontrado |" >> "$MASTER_REPORT"
fi

# Logging estruturado
if grep -rq "slog\|zap\|logrus" --include="*.go" . 2>/dev/null; then
    echo "| Logging Estruturado | ✅ | Implementado |" >> "$MASTER_REPORT"
else
    echo "| Logging Estruturado | ❌ | Não encontrado |" >> "$MASTER_REPORT"
fi

# Metrics
if grep -rq "prometheus\|metrics\|opentelemetry" --include="*.go" . 2>/dev/null; then
    echo "| Métricas | ✅ | Implementado |" >> "$MASTER_REPORT"
else
    echo "| Métricas | ❌ | Não encontrado |" >> "$MASTER_REPORT"
fi

# JWT
if grep -rq "jwt\|JWT" --include="*.go" . 2>/dev/null; then
    echo "| Autenticação JWT | ✅ | Implementado |" >> "$MASTER_REPORT"
else
    echo "| Autenticação JWT | ❌ | Não encontrado |" >> "$MASTER_REPORT"
fi

# WebSocket
if grep -rq "websocket\|WebSocket" --include="*.go" . 2>/dev/null; then
    echo "| WebSocket | ✅ | Implementado |" >> "$MASTER_REPORT"
else
    echo "| WebSocket | ❌ | Não encontrado |" >> "$MASTER_REPORT"
fi

# Redis
if grep -rq "redis" --include="*.go" . 2>/dev/null; then
    echo "| Redis | ✅ | Implementado |" >> "$MASTER_REPORT"
else
    echo "| Redis | ❌ | Não encontrado |" >> "$MASTER_REPORT"
fi

# PostgreSQL
if grep -rq "postgres\|gorm" --include="*.go" . 2>/dev/null; then
    echo "| PostgreSQL | ✅ | Implementado |" >> "$MASTER_REPORT"
else
    echo "| PostgreSQL | ❌ | Não encontrado |" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"
echo ""

# ============================================================
# FASE 12: RELATÓRIO FINAL
# ============================================================
log "FASE 12: GERANDO RELATÓRIO FINAL"

# Adicionar resumo executivo
cat >> "$MASTER_REPORT" << EOF

---

## 📋 RESUMO EXECUTIVO

### Pontuação de Prontidão para Produção

EOF

# Calcular pontuação
SCORE=0
MAX_SCORE=100

# Verificações básicas
[ -f "Dockerfile" ] && SCORE=$((SCORE + 10))
[ -f ".env.example" ] && SCORE=$((SCORE + 5))
[ -f "README.md" ] && SCORE=$((SCORE + 5))
[ -d ".github/workflows" ] && SCORE=$((SCORE + 10))
[ "$TEST_FILES" -gt 0 ] && SCORE=$((SCORE + 15))
[ "$GO_FILES" -gt 0 ] && SCORE=$((SCORE + 10))
grep -rq "signal.Notify\|SIGTERM" --include="*.go" . 2>/dev/null && SCORE=$((SCORE + 10))
grep -rq "health" --include="*.go" . 2>/dev/null && SCORE=$((SCORE + 10))
grep -rq "slog\|zap" --include="*.go" . 2>/dev/null && SCORE=$((SCORE + 5))
grep -rq "prometheus\|metrics" --include="*.go" . 2>/dev/null && SCORE=$((SCORE + 5))
[ -f "PRODUCTION.md" ] && SCORE=$((SCORE + 5))
[ -f "SECURITY.md" ] && SCORE=$((SCORE + 5))

PERCENT=$((SCORE * 100 / MAX_SCORE))

echo "**Pontuação: $SCORE/$MAX_SCORE ($PERCENT%)**" >> "$MASTER_REPORT"
echo "" >> "$MASTER_REPORT"

if [ $PERCENT -ge 80 ]; then
    echo "🟢 **STATUS: PRONTO PARA PRODUÇÃO**" >> "$MASTER_REPORT"
elif [ $PERCENT -ge 60 ]; then
    echo "🟡 **STATUS: QUASE PRONTO - Ajustes necessários**" >> "$MASTER_REPORT"
elif [ $PERCENT -ge 40 ]; then
    echo "🟠 **STATUS: EM DESENVOLVIMENTO - Trabalho significativo necessário**" >> "$MASTER_REPORT"
else
    echo "🔴 **STATUS: NÃO PRONTO - Fase inicial**" >> "$MASTER_REPORT"
fi

echo "" >> "$MASTER_REPORT"

# Próximos passos
cat >> "$MASTER_REPORT" << EOF

### Próximos Passos Recomendados

1. **Revisar os logs detalhados** na pasta \`$REPORT_DIR/\`
2. **Configurar variáveis de ambiente** no arquivo \`.env\`
3. **Subir infraestrutura local:**
   \`\`\`bash
   docker run -d --name postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:15
   docker run -d --name redis -p 6379:6379 redis:7
   \`\`\`
4. **Executar o backend:**
   \`\`\`bash
   cd cmd/fuudelivery && go run main.go
   \`\`\`

---

*Relatório gerado automaticamente em $(date '+%Y-%m-%d %H:%M:%S')*
EOF

success "Relatório master salvo em: $MASTER_REPORT"
echo ""

# Exibir resumo no terminal
echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                    AUDITORIA CONCLUÍDA                      ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}📊 Pontuação de Produção: $SCORE/$MAX_SCORE ($PERCENT%)${NC}"
echo ""
echo -e "${BLUE}📁 Arquivos gerados:${NC}"
ls -la "$REPORT_DIR/"
echo ""
echo -e "${YELLOW}📄 Relatório principal: $MASTER_REPORT${NC}"
echo ""
echo -e "${GREEN}✅ Envie o conteúdo do relatório para análise detalhada.${NC}"
