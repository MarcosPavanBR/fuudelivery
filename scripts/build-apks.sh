#!/bin/bash
# build-apks.sh — Gera APKs do AppComida (cliente), AppEntrega (entregador) e AppRestaurante (restaurante)
#
# Opcoes:
#   ./scripts/build-apks.sh              # EAS Build (nuvem, recomendado) — aguarda e imprime o link do APK
#   ./scripts/build-apks.sh --local      # Build local (requer Android SDK + Java)
#   ./scripts/build-apks.sh --login      # Login no EAS primeiro

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
DIST_DIR="$ROOT_DIR/dist-apks"
ANDROID_HOME="${ANDROID_HOME:-$HOME/AppData/Local/Android/Sdk}"

log()  { echo "[$(date +%H:%M:%S)] $1"; }
ok()   { echo "[OK] $1"; }
err()  { echo "[ERRO] $1"; }

build_eas() {
    local dir="$1" name="$2"
    cd "$ROOT_DIR/$dir"
    if ! npx eas whoami > /dev/null 2>&1; then
        err "Nao logado no EAS. Execute: eas login"; cd "$ROOT_DIR"; return 1
    fi
    log "Gerando APK para $name via EAS Build (cloud)... aguardando conclusao (~15-20 min)"
    local OUT
    OUT=$(npx eas build --platform android --profile preview --non-interactive 2>&1)
    local rc=$?
    cd "$ROOT_DIR"
    if [ $rc -ne 0 ]; then
        err "Build de $name falhou (exit $rc)"
        echo "$OUT" | tail -30
        return 1
    fi
    local url
    url=$(echo "$OUT" | grep -oE 'https://expo\.dev/artifacts/eas/[A-Za-z0-9._-]+\.apk' | tail -1)
    if [ -n "$url" ]; then
        ok "APK de $name pronto: $url"
    else
        ok "Build de $name concluido! Nao achei URL no log — veja o dashboard do Expo."
    fi
}

build_local() {
    local dir="$1" name="$2"
    command -v java > /dev/null 2>&1 || { err "Java nao encontrado"; return 1; }
    [ -d "$ANDROID_HOME" ] || { err "Android SDK nao encontrado em $ANDROID_HOME"; return 1; }
    cd "$ROOT_DIR/$dir"
    log "Prebuild + Build local para $name..."
    npx expo prebuild --platform android --no-install
    cd android && export ANDROID_HOME && ./gradlew assembleRelease --no-daemon
    local rc=$?
    cd "$ROOT_DIR"
    if [ $rc -ne 0 ]; then err "Gradle build falhou para $name"; return 1; fi
    local apk="$ROOT_DIR/$dir/android/app/build/outputs/apk/release/app-release.apk"
    if [ -f "$apk" ]; then
        mkdir -p "$DIST_DIR"
        cp "$apk" "$DIST_DIR/${name}-release.apk"
        ok "APK: $DIST_DIR/${name}-release.apk"
    else
        err "APK nao encontrado em $apk"; return 1
    fi
}

MODE="eas"
case "${1:-}" in
    --local) MODE="local"; shift ;;
    --login) MODE="login"; shift ;;
    --help|-h) echo "Uso: $0 [--local|--login]"; exit 0 ;;
esac

echo "=========================================="
echo "  FuuDelivery — Gerador de APKs"
echo "=========================================="

if [ "$MODE" = "login" ]; then
    npx eas login; ok "Login concluido! Execute $0 para gerar APKs."; exit 0
fi

rm -rf "$DIST_DIR"; mkdir -p "$DIST_DIR"
APPS=("Frontend/AppComida:appcomida" "Frontend/AppEntrega:appentrega" "Frontend/AppRestaurante:apprestaurante")
FAILED=0
for entry in "${APPS[@]}"; do
    IFS=':' read -r dir name <<< "$entry"
    echo ""; log "Build: $name ($dir)"
    if [ "$MODE" = "local" ]; then
        build_local "$dir" "$name" || FAILED=$((FAILED+1))
    else
        build_eas "$dir" "$name" || FAILED=$((FAILED+1))
    fi
done

echo ""; echo "=========================================="
if [ $FAILED -eq 0 ]; then ok "Todos os APKs gerados com sucesso!"
else err "$FAILED build(s) falharam. Verifique os erros acima."; fi
echo "=========================================="
