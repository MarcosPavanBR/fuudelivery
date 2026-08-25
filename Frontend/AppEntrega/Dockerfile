# FuuDelivery AppEntrega — container de desenvolvimento Expo
FROM node:20-alpine

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

# Dependências primeiro (cache de camadas)
COPY package*.json /usr/src/app/
RUN npm ci --legacy-peer-deps || npm install --legacy-peer-deps

COPY app.json /usr/src/app/

EXPOSE 19000 19001 19002

# Login NÃO é necessário para `expo start` — credenciais via CLI expunham
# a senha no process list. Para EAS builds use EXPO_TOKEN (env) no CI.
CMD ["npx", "expo", "start"]
