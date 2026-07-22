# FuuDelivery — Configuracoes & Credenciais
> ATENCAO: Este arquivo contem credenciais sensíveis. NUNCA committar em repos publicos.

---

## Render.com — Servicos

| Servico | ID | Tipo | URL |
|---------|-----|------|-----|
| fuudelivery-api | srv-d9e55qf41pts73e8q8dg | web_service | https://fuudelivery-api-8y6l.onrender.com |
| fuudelivery-web | srv-d9edpar7uimc73fdotp0 | static_site | https://fuudelivery-web.onrender.com |
| fuudelivery-admin | srv-d9elp2n41pts73f5kvf0 | static_site | https://fuudelivery-admin-lv7f.onrender.com |
| fuudelivery-payment | srv-d9gego3rjlhs739jgrfg | web_service | https://fuudelivery-payment.onrender.com |
| fuudelivery-payment-panel | srv-d9gefarrjlhs739jdl90 | static_site | https://fuudelivery-payment-panel.onrender.com |

**Render API Token:** `rnd_uWc5UfLvn8OFUxcagUtwHSMYxFWN`
**Render Owner ID:** `tea-d9e51in41pts73e8j02g`
**Render Account:** `marcosedesejo.ms@gmail.com`

---

## MongoDB Atlas

**Connection String:**
```
mongodb+srv://pavanbrtl050_db_user:q0RGpDo30LXVI0eS@fuudelivery.hj0pytw.mongodb.net/fuudelivery?retryWrites=true&w=majority&appName=fuudelivery
```
- Database principal: `fuudelivery`
- Database payments: `fuudelivery_payments`
- User: `pavanbrtl050_db_user`
- Password: `q0RGpDo30LXVI0eS`
- Cluster: `fuudelivery.hj0pytw.mongodb.net`

---

## Redis (Render)

**Connection String:**
```
redis://default:lYuQeZTS8MB7YkPIPrxekcnH2iuNs68H@company-fog-request-20441.db.redis.io:12264
```

---

## Supabase (PostgreSQL)

**Connection String:**
```
postgresql://postgres.prpfuoqhazfynpsfsrpb:%40MarcosPavan1301@aws-1-us-east-2.pooler.supabase.com:6543/postgres
```

---

## AbacatePay (Gateway de Pagamentos)

**API Key:** `abc_prod_uCfXQTJqBxJgY5MQLxSKghPL`
**Webhook Secret:** `whsec_fuudelivery_prod_2024`

---

## JWT

**Secret:** `fuudelivery-jwt-secret-change-in-production-1234567890abcdef`

---

## Admin Bootstrap

**Secret:** `fuu-bootstrap-2026`

---

## EnvVars do servico API (Render)

| Variavel | Valor |
|----------|-------|
| PORT | 3000 |
| GO_ENV | production |
| JWT_SECRET | fuudelivery-jwt-secret-change-in-production-1234567890abcdef |
| MONGODB_ATLAS_URI | (usar MONGO_URI acima) |
| MONGO_URI | (ver acima) |
| MONGO_DATABASE | fuudelivery |
| PAYMENT_MONGO_DATABASE | fuudelivery_payments |
| REDIS_URL | (ver acima) |
| DB_CONNECTION_STRING | (ver Supabase acima) |
| ABACATE_PAY_API_KEY | abc_prod_uCfXQTJqBxJgY5MQLxSKghPL |
| ABACATE_PAY_WEBHOOK_SECRET | whsec_fuudelivery_prod_2024 |
| ADMIN_BOOTSTRAP_SECRET | fuu-bootstrap-2026 |
| API_BASE_URL | https://fuudelivery-api-8y6l.onrender.com |
| URL_GET_ESTABLISHMENT_ID | http://localhost:3000/establishments/%d |
| URL_CHECK_ESTABLISHMENT_OPEN | http://localhost:3000/establishments/%d/is-open |

---

## EnvVars do servico Payment (Render)

| Variavel | Valor |
|----------|-------|
| PORT | 8084 |
| MONGO_URI | (usar string do MongoDB acima com database fuudelivery_payments) |
| PAYMENT_MONGO_DATABASE | fuudelivery_payments |
| JWT_SECRET | fuudelivery-jwt-secret-change-in-production-1234567890abcdef |
| RABBIT_CONNECTION | amqp://guest:guest@localhost:5672/ |
| RABBIT_PAYMENT_QUEUE | payment_queue |
| ABACATE_PAY_API_KEY | abc_prod_uCfXQTJqBxJgY5MQLxSKghPL |

---

## EnvVars dos Frontends (Render)

### fuudelivery-web (WebRestaurant)
| Variavel | Valor |
|----------|-------|
| REACT_APP_API_URL | https://fuudelivery-api-8y6l.onrender.com |
| REACT_APP_PAYMENT_API_URL | https://fuudelivery-payment.onrender.com |

### fuudelivery-admin (WebAdmin)
| Variavel | Valor |
|----------|-------|
| REACT_APP_API_URL | https://fuudelivery-api-8y6l.onrender.com |

---

## Credenciais de Login

### Admin
- Email: `admin2@email.com`
- Senha: `123456`
- Role: `admin` (via bootstrap)

---

## URLs de Producao

- **API:** https://fuudelivery-api-8y6l.onrender.com
- **WebRestaurant:** https://fuudelivery-web.onrender.com
- **WebAdmin:** https://fuudelivery-admin-lv7f.onrender.com
- **Payment Service:** https://fuudelivery-payment.onrender.com
- **Payment Panel:** https://fuudelivery-payment-panel.onrender.com

---

## GitHub

- **Repo:** https://github.com/MarcosPavanBR/fuudelivery
- **Branch:** master
- **User:** MarcosPavanBR

---

## Comandos Uteis

```bash
# Deploy manual via Render API
curl -X POST https://api.render.com/v1/services/srv-d9gego3rjlhs739jgrfg/deploys \
  -H "Authorization: Bearer rnd_uWc5UfLvn8OFUxcagUtwHSMYxFWN"

# Verificar status
curl https://api.render.com/v1/services/srv-d9gego3rjlhs739jgrfg/deploys \
  -H "Authorization: Bearer rnd_uWc5UfLvn8OFUxcagUtwHSMYxFWN"

# Verificar env vars
curl https://api.render.com/v1/services/srv-d9gego3rjlhs739jgrfg/env-vars \
  -H "Authorization: Bearer rnd_uWc5UfLvn8OFUxcagUtwHSMYxFWN"
```

---

*Atualizado em: 2026-07-22*
