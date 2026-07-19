# Multi-stage build for FUUDELIVERY monolith
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy entire project (replace directives in go.mod handle local deps)
COPY . .

# Build the monolith
RUN cd cmd/fuudelivery && go build -o /app/server .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 3000
CMD ["./server"]
