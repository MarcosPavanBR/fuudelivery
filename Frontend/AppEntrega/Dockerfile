FROM node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm install --legacy-peer-deps

COPY . .

EXPOSE 19002

CMD ["npx", "expo", "start", "--qrcode"]
