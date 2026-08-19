# ============================================================
# WebRestaurant — Frontend do Restaurante (Vite + React 19)
# ============================================================
# Build estático multi-estágio: node compila com Vite e o
# resultado é servido pelo nginx. Substitui o Dockerfile antigo
# que instalava expo-cli (herança de mobile, era quebrado).

# ── Estágio 1: build ─────────────────────────────────────────
FROM node:20-alpine AS build
WORKDIR /app

# URL da API injetada em tempo de build (REACT_APP_API_URL).
# Ex.: --build-arg REACT_APP_API_URL=https://api.suaempresa.com
ARG REACT_APP_API_URL=http://localhost:3000
ENV REACT_APP_API_URL=$REACT_APP_API_URL

# Instala dependências (usa o .npmrc com legacy-peer-deps)
COPY package.json package-lock.json* .npmrc ./
RUN npm install

# Copia o código e compila
COPY . .
RUN npm run build

# ── Estágio 2: nginx ─────────────────────────────────────────
FROM nginx:alpine
# Copia o bundle estático gerado pelo Vite (dist/)
COPY --from=build /app/dist /usr/share/nginx/html

# SPA fallback: qualquer rota → index.html (mesmo efeito do _redirects)
RUN printf 'server {\n  listen 80;\n  root /usr/share/nginx/html;\n  index index.html;\n  location / {\n    try_files $uri $uri/ /index.html;\n  }\n}\n' > /etc/nginx/conf.d/default.conf

EXPOSE 80

USER nginx
