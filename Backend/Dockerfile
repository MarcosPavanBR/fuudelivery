FROM openresty/openresty:1.27.1.2-0-alpine

RUN apk update && apk add --no-cache curl

COPY jwt-auth.lua /etc/nginx/jwt-auth.lua
COPY nginx.conf /usr/local/openresty/nginx/conf/nginx.conf

ENV JWT_SECRET=""
