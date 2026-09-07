---
name: docker-patterns
description: Padrões de Docker/Docker Compose — multi-stage build, redes isoladas, e configuração local que espelha produção. Use ao criar ou revisar Dockerfile/docker-compose.
---

# Padrões Docker

## Multi-stage build
```dockerfile
FROM golang:1.23 AS build
WORKDIR /app
COPY . .
RUN go build -o server ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /app/server /server
ENTRYPOINT ["/server"]
```
Imagem final não tem compilador, ferramentas de build, nem código-fonte além do binário — reduz superfície de ataque e tamanho.

## Docker Compose local
- Serviços com nomes de rede internos consistentes (`db`, `redis`, `api`) usados nas connection strings — nunca `localhost` dentro de um container (aponta para o próprio container, não para o serviço vizinho).
- Volumes nomeados para dados persistentes (Postgres) — evita perder dado ao recriar container.
- `.env` local nunca commitado; `.env.example` com placeholders sempre atualizado quando uma variável nova é adicionada.

## Paridade com produção
- Versão de imagem base (ex: `postgres:16`) igual à usada em produção — divergência de versão é uma causa clássica de "funciona local, quebra em produção".
