import json
import sys


def main() -> int:
    service = sys.argv[1] if len(sys.argv) > 1 else "?"
    try:
        raw = sys.stdin.read()
        routes = json.loads(raw)
    except Exception as e:
        print(f"::error::Resposta invalida do GET /routes ({service}): {e}")
        return 1
    if not isinstance(routes, list):
        print(f"::error::Resposta inesperada do GET /routes ({service}): {routes!r}")
        return 1
    for r in routes:
        # O Render pode devolver o item direto ou aninhado:
        #   [{"type":"rewrite","source":"/*","destination":"/index.html"}]
        #   [{"route":{...},"cursor":"..."}]   <- formato real do GET /routes
        if isinstance(r, dict) and "route" in r and isinstance(r["route"], dict):
            r = r["route"]
        if (
            r.get("type") in ("rewrite", "redirect")
            and r.get("source") == "/*"
            and r.get("destination") == "/index.html"
        ):
            print(f"::notice::Rota SPA (/* -> /index.html) confirmada em {service}")
            return 0
    print(
        f"::error::Regra SPA (/* -> /index.html) NAO aplicada em {service}. "
        f"Rotas atuais: {json.dumps(routes)}"
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
