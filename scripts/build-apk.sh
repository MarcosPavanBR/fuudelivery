#!/usr/bin/env bash
# build-apk.sh — FuuDelivery APK Builder
# Validates prerequisites then builds APK(s) via EAS cloud build.
# Usage: ./scripts/build-apk.sh [comida|entrega] [--clean] [--install] [--validate]

set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
info()    { echo -e "${BLUE}ℹ${NC}  $*"; }
success() { echo -e "${GREEN}✔${NC}  $*"; }
warn()    { echo -e "${YELLOW}⚠${NC}  $*"; }
error()   { echo -e "${RED}✘${NC}  $*" >&2; }
step()    { echo -e "\n${BOLD}${CYAN}── $* ──${NC}"; }
die()     { error "$1"; echo -e "\n${RED}Aborted.${NC}"; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
APPCOMIDA_DIR="$PROJECT_ROOT/Frontend/AppComida"
APPENTREGA_DIR="$PROJECT_ROOT/Frontend/AppEntrega"
REQUIRED_PLUGINS=("expo-router")
BANNED_PLUGINS=("withGradleWorkaround" "expo-modules-core" "withGradleWorkaround.js")
REQUIRED_PKGS=("expo-router" "expo" "react" "react-native" "expo-updates")

MODE="build"; TARGET="both"; CLEAN=false
for arg in "$@"; do
  case "$arg" in
    --install)  MODE="install" ;;
    --validate) MODE="validate" ;;
    --clean)    CLEAN=true ;;
    --help|-h) echo "Usage: $0 [comida|entrega] [--clean] [--install] [--validate]"; exit 0 ;;
    comida)     TARGET="comida" ;;
    entrega)    TARGET="entrega" ;;
    *)          die "Unknown: $arg" ;;
  esac
done

validate_node() {
  step "Checking Node.js"
  command -v node &>/dev/null || die "Node.js not installed"
  local v; v=$(node -v | sed 's/v//' | cut -d. -f1)
  [ "$v" -lt 18 ] && die "Node.js v$(node -v) too old, need >= 18"
  success "Node.js $(node -v)"
}

validate_npm() {
  step "Checking npm"
  command -v npm &>/dev/null || die "npm not installed"
  success "npm $(npm -v)"
}

validate_jq() {
  step "Checking jq"
  command -v jq &>/dev/null || die "jq not installed. Install: brew install jq / apt install jq"
  success "jq $(jq --version)"
}

validate_eas() {
  step "Checking EAS CLI"
  if command -v eas &>/dev/null; then EAS_CMD="eas"
  elif command -v npx &>/dev/null; then EAS_CMD="npx eas-cli"; info "Using npx eas-cli"
  else die "eas-cli not found. Install: npm i -g eas-cli"; fi
  success "EAS CLI $($EAS_CMD --version 2>/dev/null | head -1 || echo '?')"
}

validate_login() {
  step "Checking EAS auth"
  [ -n "${EXPO_TOKEN:-}" ] && success "EXPO_TOKEN set" && return 0
  local w; w=$($EAS_CMD whoami 2>/dev/null || echo "")
  [ -n "$w" ] && success "Logged in: $w" && return 0
  die "Not logged in. Run: eas login"
}

validate_nm() {
  local d="$1" n="$2"
  step "Checking node_modules — $n"
  [ ! -d "$d/node_modules" ] && warn "node_modules missing" && return 1
  local c; c=$(find "$d/node_modules" -maxdepth 1 -mindepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')
  [ "$c" -lt 10 ] && warn "Only $c packages" && return 1
  local miss=()
  for p in "${REQUIRED_PKGS[@]}"; do [ ! -d "$d/node_modules/$p" ] && miss+=("$p"); done
  [ ${#miss[@]} -gt 0 ] && warn "Missing: ${miss[*]}" && return 1
  success "node_modules OK ($c packages)"
  return 0
}

validate_app_json() {
  local d="$1" n="$2"
  step "Checking app.json — $n"
  local f="$d/app.json"
  [ ! -f "$f" ] && die "app.json not found"
  local bad=false
  for p in "${BANNED_PLUGINS[@]}"; do
    grep -q "\"$p\"" "$f" 2>/dev/null && error "BANNED: '$p' — WILL fail" && bad=true
  done
  for p in "${REQUIRED_PLUGINS[@]}"; do
    grep -q "\"$p\"" "$f" 2>/dev/null && success "Plugin '$p' OK" || { warn "Missing '$p'"; bad=true; }
  done
  grep -q '"projectId"' "$f" 2>/dev/null && success "EAS project ID OK" || warn "No project ID"
  [ "$bad" = true ] && return 1
  return 0
}

validate_eas_json() {
  local d="$1" n="$2"
  step "Checking eas.json — $n"
  local f="$d/eas.json"
  [ ! -f "$f" ] && die "eas.json not found"
  grep -q '"preview"' "$f" 2>/dev/null && success "Preview profile OK" || warn "No preview profile"
  grep -q '"buildType": "apk"' "$f" 2>/dev/null && success "Build type: APK"
}

validate_versions() {
  local d="$1" n="$2"
  step "Checking SDK versions — $n"
  local p="$d/package.json"
  [ ! -f "$p" ] && return 0
  local e r rn rt
  e=$(node -e "console.log(require('$p').dependencies.expo||'?')" 2>/dev/null)
  r=$(node -e "console.log(require('$p').dependencies.react||'?')" 2>/dev/null)
  rn=$(node -e "console.log(require('$p').dependencies['react-native']||'?')" 2>/dev/null)
  rt=$(node -e "console.log(require('$p').dependencies['expo-router']||'?')" 2>/dev/null)
  info "expo:$e react:$r rn:$rn router:$rt"
}

install_deps() {
  local d="$1" n="$2"
  step "Installing deps — $n"
  cd "$d"
  [ "$CLEAN" = true ] && info "Cleaning..." && rm -rf node_modules package-lock.json
  if ! validate_nm "$d" "$n" 2>/dev/null; then
    info "npm install --legacy-peer-deps..."
    npm install --legacy-peer-deps 2>&1 | tail -5 || die "npm install failed for $n"
    success "Installed"
  else success "Already installed"; fi
}

build_app() {
  local d="$1" n="$2" lbl="$3"
  step "Building APK — $lbl"
  cd "$d"
  info "EAS cloud build (~15-20 min)..."
  local OUT
  if OUT=$($EAS_CMD build --platform android --profile preview --non-interactive --json 2>&1); then
    local bid; bid=$(echo "$OUT" | jq -r '.id' 2>/dev/null || echo "")
    if [ -n "$bid" ] && [ "$bid" != "null" ]; then
      success "Build submitted! ID: $bid"
      info "Monitor: https://expo.dev/accounts/pavanbr/projects/my-app/builds"
    else warn "Submitted but no build ID parsed"; echo "$OUT" | head -20; fi
  else
    error "Failed: $lbl"; echo "$OUT" | tail -20
    echo "$OUT" | grep -q "expo-module-gradle-plugin" && error "Fix: remove banned plugins from app.json"
    echo "$OUT" | grep -q "expo-router" && error "Fix: npm install — expo-router missing"
    echo "$OUT" | grep -q "Not authenticated" && error "Fix: eas login"
    return 1
  fi
}

echo ""
echo -e "${BOLD}╔══════════════════════════════════════╗${NC}"
echo -e "${BOLD}║  🚀 FuuDelivery APK Builder         ║${NC}"
echo -e "${BOLD}╚══════════════════════════════════════╝${NC}"
info "Mode: $MODE | Target: $TARGET | Clean: $CLEAN"

validate_node; validate_npm; validate_jq; validate_eas; validate_login

BC=false; BE=false
if [ "$TARGET" = "both" ] || [ "$TARGET" = "comida" ]; then
  validate_app_json "$APPCOMIDA_DIR" AppComida || true
  validate_eas_json "$APPCOMIDA_DIR" AppComida || true
  validate_versions "$APPCOMIDA_DIR" AppComida || true
  BC=true
fi
if [ "$TARGET" = "both" ] || [ "$TARGET" = "entrega" ]; then
  validate_app_json "$APPENTREGA_DIR" AppEntrega || true
  validate_eas_json "$APPENTREGA_DIR" AppEntrega || true
  validate_versions "$APPENTREGA_DIR" AppEntrega || true
  BE=true
fi

[ "$MODE" = "validate" ] && echo && success "All OK!" && exit 0

[ "$BC" = true ] && install_deps "$APPCOMIDA_DIR" AppComida
[ "$BE" = true ] && install_deps "$APPENTREGA_DIR" AppEntrega
[ "$MODE" = "install" ] && echo && success "Done!" && exit 0

echo ""
echo -e "${BOLD}── Builds ──${NC}"
F=0
[ "$BC" = true ] && ! build_app "$APPCOMIDA_DIR" AppComida "FuuDelivery (Client)" && F=$((F+1))
[ "$BE" = true ] && ! build_app "$APPENTREGA_DIR" AppEntrega "FuuEntrega (Driver)" && F=$((F+1))
echo ""
[ "$F" -eq 0 ] && echo -e "${GREEN}${BOLD}✔ All submitted!${NC}" || echo -e "${YELLOW}${BOLD}⚠ $F failed${NC}"
info "Monitor: https://expo.dev/accounts/pavanbr/projects/my-app/builds"
echo ""
exit $F
