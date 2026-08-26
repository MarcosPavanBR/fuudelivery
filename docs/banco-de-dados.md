# Dicionário de dados — FuuDelivery (banco único Postgres/Supabase)


> ⚠️ **`Backend/Payment` foi arquivado e removido do repositório.** Todo o código
> de pagamento ativo vive em `payment_api` (embutido no monolito `cmd/fuudelivery`).
> As menções a `Backend/Payment` neste documento são **históricas** — não edite,
> não busque e não rode comandos apontando para esse diretório.
> Gerado a partir do código-fonte real (structs Go com `gorm:"primaryKey"` ou
> `bson:"..."`) + dos scripts de consolidação em `sql/`. Sempre que uma
> tabela ou coluna mudar, atualize este arquivo no mesmo commit — é a regra
> nº 1 da skill `fuudelivery-banco-unico` (ver `skills/`).

Legenda de origem: **PG** = já era Postgres antes desta consolidação (gerido por GORM AutoMigrate em `auth_api`/`orders_api`). **MG→PG** = migrado de uma collection MongoDB por este pacote.

## Domínio: autenticação e contas (`auth_api`) — PG

| Tabela | O que guarda | Colunas-chave |
|---|---|---|
| `users` | Donos de restaurante (login por email) | `email`, `role` (`user`/`admin`), `establishment_id` |
| `clients` | Clientes finais (login por telefone) | `phone` (único), `password` |
| `establishments` | Restaurantes | `owner_id`, `zone_id`, `lat`/`long`, `payment_wallet_id` |
| `delivery_men` | Entregadores | `zone_id`, `status` (available/busy/offline), `current_lat`/`current_lng` |
| `business_hours` | Horário de funcionamento por dia da semana | `establishment_id`, `day_of_week` (únicos juntos) |
| `zones` | Praça/região: split de pagamento, raio de entrega, algoritmo de match | `platform_fee_percentage`, `establishment_percentage`, `radius_km` |
| `subscriptions` | Assinatura do cliente (frete grátis/cashback) | `user_id`, `plan` (basic/premium) |
| `sponsored_listings` | Patrocínio de restaurante na busca | `establishment_id`, `zone_id`, `plan`, `priority` |
| **`password_reset_tokens`** *(novo, script 13)* | Códigos de uso único para reset assistido de senha | `user_type` + `user_id`, `code_hash` (SHA-256, ⚠ sensível — redigido no audit_log), `expires_at`, `used_at` |

## Domínio: pedidos (`orders_api`) — PG (já existia) + MG→PG (push_tokens)

| Tabela | O que guarda | Colunas-chave |
|---|---|---|
| `categories` / `products` / `additionals` | Cardápio | `establishment_id` |
| `category_products` / `additional_products` | Tabelas de junção many-to-many | — |
| `orders` | Pedido (cabeçalho) | `status`, `zone_id`, `batch_id`, `match_radius_km` |
| `order_items` | Itens do pedido | `order_id`, `product_id`, `quantity` |
| `coupons` / `coupon_usages` | Cupons de desconto | `code` (único), `establishment_id` |
| `loyalty_points` / `loyalty_transactions` | Programa de fidelidade | `user_phone` |
| `reviews` | Avaliações de pedido | `order_id` (único), `rating` (1-5) |
| `batches` | Lotes de entrega (courier pegando vários pedidos) | `zone_id`, `courier_id`, `status` |
| **`push_tokens`** *(novo, script 01)* | Token de push notification | `user_id` + `user_type` (único). **Antes:** collection Mongo solta, sem schema. |

## Domínio: entrega (`delivery_api`) — MG→PG

| Tabela | O que guarda | Colunas-chave |
|---|---|---|
| **`delivery_solicitations`** *(novo, script 02)* | Read-model do pedido para o motor de despacho (dados denormalizados de propósito, para o matching não depender de JOIN pesado) | `order_id` (único), `status`, `zone_id`, `delivery_man_id`, `batch_id`, `products` (JSONB). **Antes:** struct `OrderDTO` numa collection Mongo. |

## Domínio: pagamentos (`payment_api` + `Backend/Payment`) — MG→PG, **unificado**

Este é o domínio que existia **duplicado em dois bancos Mongo**. As tabelas abaixo unificam os dois.

| Tabela | O que guarda | Colunas-chave |
|---|---|---|
| **`payments`** | Um único registro por pagamento (cobrança PIX/cartão + fluxo de aprovação/risco) | `order_id`, `status`, `risk_level`, `method`, `card_token` (⚠ sensível) |
| **`wallets`** | Saldo de restaurante/entregador | `user_id` + `user_type` (único), `balance`, `status` |
| **`wallet_transactions`** | Histórico imutável de crédito/débito (nunca UPDATE/DELETE, só INSERT) | `wallet_id`, `type` (credit/debit), `balance_before`/`balance_after` |
| **`chargebacks`** | Disputas/estornos | `payment_id`, `reason`, `status` |
| **`chargeback_evidence`** | Evidências anexadas a uma disputa | `chargeback_id`, `type`, `file_url` |
| **`payout_requests`** | Solicitações de saque via Pix | `user_id`+`user_type`, `pix_key`, `status` |
| **`payment_approval_rules`** | Regras globais de aprovação automática/manual (tabela de **1 linha só**) | `auto_approve_max_amount`, `manual_review_min_risk` |
| **`payment_admin_users`** | Operadores/admins do painel de pagamentos | `email` (único), `role` (admin/operator), `password_hash` (⚠ sensível, bcrypt) |

**Colunas sensíveis nesta seção** (nunca devolver em resposta de API, sempre redigidas no `audit_log`): `payments.card_token`, `payments.pix_copy_paste`, `payments.qr_code_base64`, `payment_admin_users.password_hash`, `password_reset_tokens.code_hash`.

## Domínio: chat (`chat_api`) — MG→PG

| Tabela | O que guarda | Colunas-chave |
|---|---|---|
| **`chat_messages`** *(novo, script 04)* | Mensagens por pedido | `order_id`, `sender_id`+`sender_type`, `read_at` (nulo = não lida) |

## Infraestrutura da própria migração

| Tabela | O que guarda |
|---|---|
| `schema_migrations` | Changelog de **estrutura**: qual script de schema já rodou, quando, por quem |
| `audit_log` | Changelog de **dados**: cada INSERT/UPDATE/DELETE em tabela auditada, com antes/depois em JSON |
| `audit_redacted_columns` | Lista de colunas que o `audit_log` sempre mascara (`[REDACTED]`) |

## Regras que valem para o banco inteiro

- **RLS habilitado em todas as tabelas de negócio** (script `06`). Só a role `app_backend` tem acesso; `anon`/`authenticated` do Supabase são revogadas.
- **Toda tabela de negócio é auditada** (script `05`) — exceto as três de infraestrutura acima (auditar o próprio log não faz sentido).
- **Toda alteração de schema precisa de um novo arquivo numerado** em `sql/` (ex: `09_...sql`) que termina com um `INSERT INTO schema_migrations`. Nunca altere um script já aplicado em produção — crie um novo.
