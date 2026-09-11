#!/usr/bin/env bash
# verify-deploy_e2e_test.sh — roda o verify-deploy.sh inteiro contra um servidor
# local, sem tocar em produção.
#
# O caso que importa é o primeiro: um /health que responde "starting" nas
# primeiras chamadas e só depois "up" — exatamente o cold start do Render. O
# script ANTIGO cravava "API is down" já na primeira resposta; o novo espera.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

PASS=0; FAIL=0
PORT=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
STATE_DIR=$(mktemp -d)
trap 'kill %1 2>/dev/null; rm -rf "$STATE_DIR"' EXIT

# Fixtures em arquivo (e não em env var) para o servidor abaixo lê-las como
# dado puro. As chaves vão em ordem alfabética porque o handler devolve um
# fiber.Map e o encoding/json ordena as chaves de um map do Go.
cat > "$STATE_DIR/up.json" <<'JSON'
{"checks":{"batches":{"name":"batches","status":"up"},"payment_gateways":{"name":"payment_gateways","status":"up"},"postgres":{"name":"postgres","status":"up"},"redis":{"name":"redis","status":"up"},"redis_geo":{"name":"redis_geo","status":"up"}},"service":"fuudelivery","status":"up","version":"1.0.0"}
JSON
cat > "$STATE_DIR/degraded.json" <<'JSON'
{"checks":{"batches":{"name":"batches","status":"degraded"},"payment_gateways":{"name":"payment_gateways","status":"up"},"postgres":{"name":"postgres","status":"up"},"redis":{"name":"redis","status":"up"},"redis_geo":{"name":"redis_geo","status":"degraded"}},"service":"fuudelivery","status":"degraded","version":"1.0.0"}
JSON
cat > "$STATE_DIR/down.json" <<'JSON'
{"checks":{"batches":{"name":"batches","status":"down"},"payment_gateways":{"name":"payment_gateways","status":"down","error":"no gateway configured"},"postgres":{"name":"postgres","status":"down","error":"connection refused"},"redis":{"name":"redis","status":"down"},"redis_geo":{"name":"redis_geo","status":"down"}},"service":"fuudelivery","status":"down","version":"1.0.0"}
JSON
cat > "$STATE_DIR/starting.json" <<'JSON'
{"message":"database initializing, please retry","service":"fuudelivery","status":"starting","version":"1.0.0"}
JSON

echo up > "$STATE_DIR/mode"

STATE_DIR="$STATE_DIR" python3 - "$PORT" <<'PY' &
import http.server, os, sys
port = int(sys.argv[1]); state = os.environ["STATE_DIR"]

def fixture(name):
    with open(os.path.join(state, name + ".json")) as fh:
        return fh.read().strip()

class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_GET(self):
        if self.path == "/health":
            mode_p = os.path.join(state, "mode")
            mode = open(mode_p).read().strip() if os.path.exists(mode_p) else "up"
            if mode == "starting_then_up":
                # As 2 primeiras chamadas respondem "starting", como o monólito
                # antes de models.DB ser atribuído; depois vira "up".
                cnt_p = os.path.join(state, "count")
                n = int(open(cnt_p).read()) if os.path.exists(cnt_p) else 0
                with open(cnt_p, "w") as fh:
                    fh.write(str(n + 1))
                body, code = (fixture("starting"), 200) if n < 2 else (fixture("up"), 200)
            elif mode == "starting_forever":
                body, code = fixture("starting"), 200
            elif mode == "degraded":
                body, code = fixture("degraded"), 200
            elif mode == "down":
                body, code = fixture("down"), 503
            else:
                body, code = fixture("up"), 200
            self.send_response(code)
        elif self.path == "/payments/all":
            body = '{"error":"unauthorized"}'
            self.send_response(401)
        else:
            body = "<html>ok</html>"
            self.send_response(200)
        b = body.encode()
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)

http.server.HTTPServer(("127.0.0.1", port), H).serve_forever()
PY

for _ in $(seq 50); do
    curl -sf -o /dev/null "http://127.0.0.1:$PORT/health" 2>/dev/null && break
    sleep 0.2
done

run_case() { # run_case <modo> <descrição> <exit-esperado> <regex-esperada>
    local mode="$1" desc="$2" want_exit="$3" want_re="$4"
    echo "$mode" > "$STATE_DIR/mode"; rm -f "$STATE_DIR/count"
    local out rc bad=0
    out=$(NO_PROXY='127.0.0.1' no_proxy='127.0.0.1' \
          API_URL="http://127.0.0.1:$PORT/health" \
          PAYMENTS_URL="http://127.0.0.1:$PORT/payments/all" \
          WEBRESTAURANT_SITE="WebRestaurant:http://127.0.0.1:$PORT/r" \
          WEBADMIN_SITE="WebAdmin:http://127.0.0.1:$PORT/a" \
          API_BUDGET=6 API_INTERVAL=1 TIMEOUT=5 RETRIES=1 RETRY_INTERVAL=1 \
          bash scripts/verify-deploy.sh 2>&1)
    rc=$?
    [ "$rc" = "$want_exit" ] || { bad=1; echo "  ✘ $desc: exit esperado $want_exit, obtido $rc"; }
    echo "$out" | grep -qE "$want_re" || { bad=1; echo "  ✘ $desc: não casou /$want_re/"; echo "$out" | sed 's/^/      /'; }
    if [ "$bad" -eq 0 ]; then PASS=$((PASS + 1)); else FAIL=$((FAIL + 1)); fi
}

# O caso que estava quebrado em produção: cold start responde "starting" antes de "up".
run_case starting_then_up "cold start (starting -> up) passa" 0 "API is healthy"
run_case up               "API de pé passa"                   0 "API is healthy"
# degraded é 200 de propósito: avisa, não derruba o monitor.
run_case degraded         "degradada avisa, não falha"        0 "API degradada"
# queda real continua sendo queda.
run_case down             "queda real falha"                  1 "API is down"
# "starting" que nunca sai disso é falha real — o serviço subiu, o banco não.
run_case starting_forever "starting eterno falha"             1 "presa em 'starting'"
# o componente crítico aparece no relatório (payment_gateways não era mostrado).
run_case down             "componentes críticos aparecem"     1 "payment_gateways: down"

echo ""
echo "════════════════════════════════════════════"
if [ "$FAIL" -eq 0 ]; then echo "OK — $PASS casos"; else echo "FALHOU — $FAIL de $((PASS + FAIL)) casos"; fi
echo "════════════════════════════════════════════"
[ "$FAIL" -eq 0 ] && exit 0
exit 1
