# Deploy do FuuDelivery em VPS Ubuntu — Guia Passo a Passo

> **Documento vivo** — mantenha este guia em sincronia com o código. Sempre que uma
> versão de stack mudar (Go, Node, Redis, etc.), atualize a [Tabela de Versões](#tabela-de-versoes)
> e **regere o PDF** com um único comando (veja [Atualizar o PDF](#atualizar-o-pdf)).
>
> **Referências relacionadas:** `PRODUCTION.md` (deploy atual no Render),
> `scripts/verify-deploy.sh` (health checks), `docker-compose.vps.yml` (stack VPS),
> `references/URLS.md` (mapa de URLs).

---

## 1. Visão Geral da Arquitetura

O FuuDelivery em VPS roda **6 contêineres** gerenciados pelo Docker Compose.
Todas as portas ficam **presas a `127.0.0.1`** — o **nginx instalado no host** é o
único ponto de entrada pública (portas 80/443), fazendo proxy reverso + SSL (Certbot).

```
                        Internet
                           │
                    ┌──────▼──────┐
                    │  nginx host │  ← porta 80/443 (HTTP/HTTPS + Certbot)
                    │   + Certbot │
                    └──┬───┬───┬──┘
        api.          │   │   │        restaurante.   admin.   painel.
        :3000         │   │   │        :3002           :3003    :3001
   ┌──────▼─────┐ ┌───▼───▼───▼──┐
   │ fuudelivery│ │ 3 frontends   │  (nginx interno, estáticos)
   │    -api    │ │ (web, admin,  │
   │ (monolito) │ │   panel)      │
   └──────┬─────┘ └───────────────┘
          │            payment.
          │            :8084
   ┌──────▼─────┐ ┌───▼──────────┐
   │    redis   │ │ payment-     │
   │  (fila/cache)│ │  service     │
   └────────────┘ └──────────────┘
          │              │
   MongoDB Atlas (nuvem) PostgreSQL Supabase (nuvem)
```

| Serviço | Imagem / build | Porta interna | Porta host |
|---|---|---|---|
| `fuudelivery-api` | `Dockerfile` (raiz, Go) | 3000 | `127.0.0.1:3000` |
| `payment-service` | `Backend/Payment/Dockerfile` (Go) | 8084 | `127.0.0.1:8084` |
| `payment-panel` | `Frontend/PaymentPanel/Dockerfile` (nginx) | 80 | `127.0.0.1:3001` |
| `web-restaurant` | `Frontend/WebRestaurant/Dockerfile` (Vite→nginx) | 80 | `127.0.0.1:3002` |
| `web-admin` | `Frontend/WebAdmin/Dockerfile` (Vite→nginx) | 80 | `127.0.0.1:3003` |
| `redis` | `redis:7-alpine` | 6379 | `127.0.0.1:6379` |

> **MongoDB e PostgreSQL continuam na nuvem** (Atlas/Supabase). Este guia não
> cobre self-hosting de bancos — manter na nuvem é mais seguro e barato em um VPS.

---

## 2. Pré-requisitos

| Item | Requisito |
|---|---|
| VPS | Ubuntu **22.04 ou 24.04 LTS**, mínimo **1 vCPU / 1 GB RAM** (2 GB recomendado), ~10 GB disco |
| Domínio | Um domínio próprio com acesso ao painel de DNS do registrador |
| MongoDB | Cluster **Atlas** (M0 free ou superior) com usuário + connection string |
| PostgreSQL | Projeto **Supabase** com `DB_CONNECTION_STRING` (pooler) |
| AbacatePay | Conta com `ABACATE_PAY_API_KEY` e `ABACATE_PAY_WEBHOOK_SECRET` |
| Acesso | Chave SSH para o VPS + usuário com `sudo` |

---

## 3. Tabela de Versões

> ⚠️ **MANTENHA ESTA TABELA ATUALIZADA.** Ela é a fonte da verdade das versões.
> Quando atualizar qualquer versão abaixo, edite este arquivo **e** o arquivo onde
> a versão é declarada (coluna "Onde declarado"), depois regere o PDF.

| Componente | Versão atual | Onde declarado |
|---|---|---|
| Ubuntu (VPS) | 22.04 / 24.04 LTS | escolha do provedor |
| Docker Engine | última estável (ex.: 27.x) | instalado no VPS |
| Docker Compose v2 | última estável (ex.: 2.x) | plugin do Docker |
| **Go** (build dos serviços) | **1.23** (`golang:1.23-alpine`) | `Dockerfile` raiz, `Backend/*/Dockerfile`, `render.yaml` (`GO_VERSION`) |
| **Node** (build dos frontends) | **20** (`node:20-alpine`) | `Frontend/WebRestaurant/Dockerfile`, `Frontend/WebAdmin/Dockerfile` |
| **Alpine** (runtime Go) | **3.21** | `Dockerfile` raiz (estágio final) |
| **Nginx** (runtime estático) | `nginx:alpine` (latest) | Dockerfiles dos frontends |
| **Redis** | **7** (`redis:7-alpine`) | `docker-compose.vps.yml` |
| Vite | 6 | `Frontend/*/package.json` |
| React | 19 | `Frontend/*/package.json` |
| Expo SDK (apps mobile — fora do VPS) | 54 | `Frontend/AppComida`, `Frontend/AppEntrega` |
| PostgreSQL | nuvem (Supabase) | `.env` → `DB_CONNECTION_STRING` |
| MongoDB | nuvem (Atlas) | `.env` → `MONGO_URI` |

### Atualizar o PDF

O PDF é gerado **a partir deste Markdown** — nunca edite o PDF à mão:

```bash
# No seu computador (com Python 3 + Edge/Chrome instalados):
python scripts/build-deploy-pdf.py
# → regera scripts/deploy-vps.pdf
```

---

## 4. Passo 1 — Instalar Docker + Compose no VPS

```bash
# 1. Atualize o sistema
sudo apt update && sudo apt upgrade -y

# 2. Instale dependências do repositório oficial do Docker
sudo apt install -y ca-certificates curl

# 3. Adicione a chave GPG e o repositório
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
  -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 4. Instale Docker Engine + Compose plugin
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 5. Adicione seu usuário ao grupo docker (evita sudo em todo comando)
sudo usermod -aG docker $USER
newgrp docker

# 6. Confirme
docker --version
docker compose version
```

**Firewall** (UFW): libere apenas SSH, HTTP e HTTPS — os contêineres estão presos
a `127.0.0.1`, então **não** libere portas 3000/8084/3001/3002/3003/6379.

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

---

## 5. Passo 2 — Clonar o Repositório

```bash
# Na home do usuário do VPS
cd ~
git clone https://github.com/MarcosPavanBR/fuudelivery.git
cd fuudelivery
```

> Se o repositório for privado, use um **deploy key** ou um **token**:
> `git clone https://<USER>:<TOKEN>@github.com/MarcosPavanBR/fuudelivery.git`
> (e evite deixar o token no histórico do shell).

---

## 6. Passo 3 — Configurar o `.env`

O `.env` é lido pelo Compose (`env_file: .env`) e injetado em **todos** os serviços.

```bash
cd ~/fuudelivery
cp .env.example .env
nano .env   # preencha com seus valores reais
```

| Variável | Descrição | Exemplo |
|---|---|---|
| `JWT_SECRET` | Chave JWT — **gere uma forte** | `openssl rand -hex 32` |
| `API_BASE_URL` | URL pública da API (para os frontends) | `https://api.suaempresa.com` |
| `APP_URL` | URL pública do painel do restaurante | `https://restaurante.suaempresa.com` |
| `PAYMENT_API_BASE_URL` | URL pública do Payment Service (build do WebAdmin) | `https://payment.suaempresa.com` |
| `DB_CONNECTION_STRING` | PostgreSQL Supabase (pooler) | `postgresql://postgres:...@aws-0-...pooler.supabase.com:6543/postgres?pgbouncer=true` |
| `MONGO_URI` | MongoDB Atlas | `mongodb+srv://user:pass@cluster0.xxxxx.mongodb.net/fuudelivery` |
| `MONGO_DATABASE` | Banco do monolito | `fuudelivery` |
| `PAYMENT_MONGO_DATABASE` | Banco do Payment Service | `fuudelivery_payments` |
| `REDIS_URL` | **Dentro do compose use `redis://redis:6379`** (nome do serviço). Se usar Redis gerenciado, aponte aqui e remova o serviço `redis` do compose | `redis://redis:6379` |
| `ABACATE_PAY_API_KEY` | Chave da AbacatePay | `abc_...` |
| `ABACATE_PAY_WEBHOOK_SECRET` | Secret do webhook (mesmo valor cadastrado na AbacatePay) | `whsec_...` |
| `PIX_KEY` / `MERCHANT_NAME` / `MERCHANT_CITY` | Dados PIX para transferências manuais | — |

> **Segurança:** `.env` contém segredos. **Nunca** faça commit dele. Já está no
> `.gitignore` — confirme: `git check-ignore .env && echo "protegido"`.
> Faça um backup fora do VPS: `scp user@vps:~/fuudelivery/.env ~/backup-fuudelivery.env`.

---

## 7. Passo 4 — Subir a Stack

```bash
cd ~/fuudelivery

# Primeira subida (compila as imagens — pode levar vários minutos)
docker compose -f docker-compose.vps.yml up -d --build

# Ver os contêineres
docker compose -f docker-compose.vps.yml ps

# Ver logs de um serviço específico
docker compose -f docker-compose.vps.yml logs -f fuudelivery-api
```

**Ajuste útil:** para não digitar `-f docker-compose.vps.yml` toda vez:

```bash
echo "COMPOSE_FILE=docker-compose.vps.yml" >> ~/.profile && source ~/.profile
# Agora basta:  docker compose up -d --build
```

---

## 8. Passo 5 — Verificar a Saúde

Todos os serviços Go expõem `GET /health`:

```bash
# Direto nas portas locais do VPS
curl -s http://127.0.0.1:3000/health | python3 -m json.tool   # API (monolito)
curl -s http://127.0.0.1:8084/health | python3 -m json.tool   # Payment Service

# Frontends (devem responder 200)
curl -s -o /dev/null -w "web-restaurant: %{http_code}\n" http://127.0.0.1:3002/
curl -s -o /dev/null -w "web-admin:       %{http_code}\n" http://127.0.0.1:3003/
curl -s -o /dev/null -w "payment-panel:   %{http_code}\n" http://127.0.0.1:3001/

# Redis
redis-cli -h 127.0.0.1 ping   # → PONG
```

O `/health` do monolito detalha dependências (`mongodb`, `postgres`, `redis`, ...):
cada uma deve vir com `"status": "up"`.

---

## 9. Passo 6 — Apontar o Domínio (DNS)

No painel do seu registrador, crie **registros A** apontando para o **IP público do VPS**:

| Tipo | Nome (host) | Valor | Uso |
|---|---|---|---|
| A | `api` | `<IP_DO_VPS>` | Monolito (API) |
| A | `payment` | `<IP_DO_VPS>` | Payment Service |
| A | `restaurante` | `<IP_DO_VPS>` | Painel do Restaurante |
| A | `admin` | `<IP_DO_VPS>` | Painel Administrativo |
| A | `painel` | `<IP_DO_VPS>` | Painel Financeiro (PaymentPanel) |
| A | `@` | `<IP_DO_VPS>` | (opcional) redireciona o domínio raiz |

> Se o VPS tiver IP dinâmico, use um **DDNS** ou um proxy (Cloudflare) apontando
> para o domínio. A propagação de DNS pode levar de minutos a algumas horas.

Confirme antes de continuar:

```bash
dig +short api.suaempresa.com   # deve retornar o IP do VPS
```

---

## 10. Passo 7 — nginx + SSL com Certbot

### 10.1 Instalar nginx e Certbot

```bash
sudo apt install -y nginx certbot python3-certbot-nginx
```

### 10.2 Criar o arquivo de configuração

Crie `/etc/nginx/sites-available/fuudelivery` com os 5 domínios:

```nginx
# ── API (monolito Go) ─────────────────────────────────────────
server {
    listen 80;
    server_name api.suaempresa.com;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # WebSocket (live tracking, chat, pedidos em tempo real)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }
}

# ── Payment Service ────────────────────────────────────────────
server {
    listen 80;
    server_name payment.suaempresa.com;

    location / {
        proxy_pass http://127.0.0.1:8084;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# ── Painel do Restaurante ──────────────────────────────────────
server {
    listen 80;
    server_name restaurante.suaempresa.com;

    location / {
        proxy_pass http://127.0.0.1:3002;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# ── Painel Administrativo ──────────────────────────────────────
server {
    listen 80;
    server_name admin.suaempresa.com;

    location / {
        proxy_pass http://127.0.0.1:3003;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# ── Painel Financeiro (PaymentPanel) ───────────────────────────
server {
    listen 80;
    server_name painel.suaempresa.com;

    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Ative e teste:

```bash
sudo ln -s /etc/nginx/sites-available/fuudelivery /etc/nginx/sites-enabled/
sudo nginx -t          # teste de sintaxe
sudo systemctl reload nginx
```

### 10.3 Emitir o SSL (Certbot)

```bash
sudo certbot --nginx \
  -d api.suaempresa.com \
  -d payment.suaempresa.com \
  -d restaurante.suaempresa.com \
  -d admin.suaempresa.com \
  -d painel.suaempresa.com \
  --redirect   # força HTTPS
```

- O Certbot edita o nginx automaticamente e ativa o redirecionamento HTTP→HTTPS.
- A renovação é automática (systemd timer): `sudo systemctl status certbot.timer`.

**Teste final:**

```bash
curl -s https://api.suaempresa.com/health | python3 -m json.tool
curl -sI https://restaurante.suaempresa.com | head -1   # → HTTP/2 200
```

---

## 11. Passo 8 — Ajustes Finais de Integração

### 11.1 Webhook da AbacatePay

No painel da AbacatePay, o webhook de pagamento deve apontar para o monolito:

```
https://api.suaempresa.com/payments/webhook
```

> O `ABACATE_PAY_WEBHOOK_SECRET` do `.env` deve ser **exatamente o mesmo**
> cadastrado no painel da AbacatePay.

### 11.2 PaymentPanel → URL do Payment Service

O `Frontend/PaymentPanel/index.html` define a URL da API em código (linhas
~148–150). Troque a constante para o domínio do VPS:

```js
const API_URL = window.location.hostname === 'localhost'
  ? 'http://localhost:8084'
  : 'https://payment.suaempresa.com';
```

Depois recompile: `docker compose up -d --build payment-panel`.

### 11.3 CORS (`ALLOWED_ORIGINS`)

O monolito valida origens via `ALLOWED_ORIGINS` (ver `cmd/fuudelivery/main.go`).
Adicione no `.env` os 3 domínios dos painéis (separados por vírgula):

```bash
ALLOWED_ORIGINS=https://restaurante.suaempresa.com,https://admin.suaempresa.com,https://painel.suaempresa.com
```

Aplique: `docker compose up -d fuudelivery-api` (recria só esse contêiner).

> Os apps mobile (AppComida/AppEntrega) não usam CORS (não são browsers) — eles
> chamam `https://api.suaempresa.com` direto. A URL deles fica em
> `Frontend/AppComida/config/api.ts` / `Frontend/AppEntrega/config/api.ts`.

---

## 12. Manutenção e Atualização de Versões

### 12.1 Atualizar o código (deploy de nova versão)

```bash
cd ~/fuudelivery
git pull                          # traz o código novo
docker compose up -d --build      # reconstrói apenas o que mudou
docker compose ps                 # confirma tudo "Up"
bash scripts/verify-deploy.sh     # health checks
```

> **Zero downtime:** como cada serviço tem `restart: unless-stopped` e o Compose
> cria o contêiner novo antes de remover o antigo, a troca é praticamente
> instantânea por serviço.

### 12.2 Atualizar uma versão de stack (ex.: Go 1.23 → 1.24)

1. Altere a tag no `Dockerfile` raiz **e** nos `Backend/*/Dockerfile`
   (ex.: `golang:1.23-alpine` → `golang:1.24-alpine`).
2. Atualize a [Tabela de Versões](#tabela-de-versoes) deste guia.
3. Reconstrua sem cache: `docker compose up -d --build --no-cache fuudelivery-api`.
4. Teste: `curl -s http://127.0.0.1:3000/health`.
5. Regere o PDF: `python scripts/build-deploy-pdf.py` e commit ambos.

### 12.3 Logs e diagnóstico

```bash
docker compose logs -f --tail=100 fuudelivery-api   # seguir logs
docker compose logs fuudelivery-payment             # logs do Payment
docker stats                                         # CPU/RAM dos contêineres
```

### 12.4 Backup

| Dado | Onde vive | Backup |
|---|---|---|
| PostgreSQL | Supabase (nuvem) | Backups automáticos do Supabase |
| MongoDB | Atlas (nuvem) | Backups automáticos do Atlas |
| Redis | VPS (volume `redis-data`) | AOF persistente no volume; para backup: `docker compose stop redis && tar -czf redis-backup.tgz $(docker volume inspect fuudelivery_redis-data -f '{{.Mountpoint}}')` |
| `.env` | VPS | `scp` para fora do VPS |

---

## 13. Troubleshooting

| Sintoma | Causa provável | Solução |
|---|---|---|
| `curl 127.0.0.1:3000/health` não responde | Build falhou / contêiner caiu | `docker compose logs fuudelivery-api`, `docker compose ps` |
| `/health` mostra `mongodb: down` | `MONGO_URI` errado ou IP não autorizado no Atlas | Corrija o `.env`; no Atlas, libere o **IP do VPS** em Network Access |
| `/health` mostra `postgres: down` | `DB_CONNECTION_STRING` errada | Confira pooler/usuário/senha no Supabase |
| `/health` mostra `redis: down` | `REDIS_URL` errado (fora do compose) | Dentro do compose use `redis://redis:6379` |
| 502 Bad Gateway no nginx | Contêiner ainda não subiu ou porta errada | `docker compose ps`; confira `proxy_pass` e a porta mapeada |
| Certbot "no domain found" | DNS ainda não propagado | `dig +short api.suaempresa.com`; aguarde a propagação |
| Certificado não renova | Porta 443 bloqueada | `sudo ufw allow 443/tcp` |
| CORS error nos painéis | `ALLOWED_ORIGINS` incompleto | Adicione os domínios e recrie `fuudelivery-api` |
| Webhook AbacatePay não chega | Secret errado ou URL errada | Confira `ABACATE_PAY_WEBHOOK_SECRET` e `https://api.../payments/webhook` |
| RAM estourada | VPS 1 GB com builds frequentes | Faça builds fora do horário; aumente o swap: `sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile` |

---

## 14. Checklist Final

- [ ] `docker compose ps` — 6 serviços `Up`
- [ ] `curl http://127.0.0.1:3000/health` — `status: ok`, dependências `up`
- [ ] `curl http://127.0.0.1:8084/health` — `status: ok`
- [ ] `https://api.suaempresa.com/health` — 200 via nginx + HTTPS
- [ ] `https://restaurante.suaempresa.com`, `https://admin.suaempresa.com`, `https://painel.suaempresa.com` — 200
- [ ] Webhook AbacatePay apontando para `https://api.suaempresa.com/payments/webhook`
- [ ] `ALLOWED_ORIGINS` com os 3 domínios
- [ ] `JWT_SECRET` forte e único (não o do exemplo)
- [ ] `.env` fora do git (`git check-ignore .env`)
- [ ] UFW: apenas 22, 80, 443
- [ ] Renovação do certificado automática (`sudo systemctl status certbot.timer`)
- [ ] Backup do `.env` fora do VPS
- [ ] Tabela de Versões atualizada + PDF regenerado

---

*Fonte da verdade de versões: Dockerfiles + docker-compose.vps.yml deste repositório.*
*Última atualização: 10 de agosto de 2026.*
