# ============================================================
# Payment Service — FuuDelivery (Go 1.23 + Fiber)
# Build: docker build -t fuudelivery-payment -f Backend/Payment/Dockerfile .
# Run:   docker run -p 8084:8084 fuudelivery-payment
# ============================================================
# O módulo Backend/Payment depende do pacote compartilhado pkg/queue
# via `replace github.com/carloshomar/fuudelivery/pkg/queue => ../../pkg/queue`
# (ver go.mod). Por isso o build context DEVE ser a RAIZ do repositório
# (não ./Backend/Payment) — o docker-compose usa `context: .` +
# `dockerfile: Backend/Payment/Dockerfile`.

# ── Estágio 1: build ─────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copia os go.mod/go.sum do Payment + do módulo compartilhado pkg/queue
# (alvo do replace) ANTES de baixar dependências — camada cacheável.
COPY Backend/Payment/go.mod Backend/Payment/go.sum ./Backend/Payment/
COPY pkg/queue/go.mod pkg/queue/go.sum ./pkg/queue/

RUN cd Backend/Payment && go mod download

# Copia o restante do código (inclui o fonte de pkg/queue)
COPY . .

# Build otimizado para produção
RUN cd Backend/Payment && CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/server .

# ── Estágio 2: runtime ───────────────────────────────────────
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -u 1001 appuser

WORKDIR /app
COPY --from=builder --chown=appuser:appuser /app/server .

USER appuser

EXPOSE 8084

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8084/health || exit 1

CMD ["./server"]
