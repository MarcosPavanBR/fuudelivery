# ============================================================
# FUUDELIVERY — Multi-stage production Dockerfile
# Build: docker build -t fuudelivery -f Dockerfile .
# Run:   docker run -p 3000:3000 fuudelivery
# ============================================================

# ---- Stage 1: Build ----
FROM golang:1.23-alpine AS builder

RUN apk --no-cache add ca-certificates tzdata git

WORKDIR /app

# Copy go.work and all go.mod files first for dependency caching
COPY go.work .
COPY cmd/fuudelivery/go.mod ./cmd/fuudelivery/
COPY cmd/fuudelivery/go.sum ./cmd/fuudelivery/

# Copy all module go.mod files for dependency resolution
COPY Backend/auth_api/go.mod ./Backend/auth_api/
COPY Backend/auth_api/go.sum ./Backend/auth_api/
COPY Backend/orders_api/go.mod ./Backend/orders_api/
COPY Backend/orders_api/go.sum ./Backend/orders_api/
COPY Backend/delivery_api/go.mod ./Backend/delivery_api/
COPY Backend/delivery_api/go.sum ./Backend/delivery_api/
COPY Backend/payment_api/go.mod ./Backend/payment_api/
COPY Backend/payment_api/go.sum ./Backend/payment_api/
COPY Backend/chat_api/go.mod ./Backend/chat_api/
COPY Backend/chat_api/go.sum ./Backend/chat_api/
COPY Backend/Payment/go.mod ./Backend/Payment/
COPY Backend/Payment/go.sum ./Backend/Payment/

# Download dependencies
RUN cd cmd/fuudelivery && go mod download

# Copy entire source
COPY . .

# Build the monolith with optimizations for production
RUN cd cmd/fuudelivery && CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.commitHash=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
    -o /app/server .

# ---- Stage 2: Runtime ----
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata curl && \
    adduser -D -u 1001 appuser

WORKDIR /app
COPY --from=builder --chown=appuser:appuser /app/server .

USER appuser

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:3000/health || exit 1

CMD ["./server"]
