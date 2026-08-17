# Consolidação para banco único (Supabase/Postgres)


> ⚠️ **`Backend/Payment` foi arquivado e removido do repositório.** Todo o código
> de pagamento ativo vive em `payment_api` (embutido no monolito `cmd/fuudelivery`).
> As menções a `Backend/Payment` neste documento são **históricas** — não edite,
> não busque e não rode comandos apontando para esse diretório.
## Diagnóstico atual

O FuuDelivery hoje usa **dois bancos que não conversam entre si**:

| Serviço | Banco hoje | Situação |
|---|---|---|
| `auth_api` | Postgres (Supabase), via `DB_CONNECTION_STRING` | users, establishments, delivery_men, clients, zones, subscriptions, sponsored_listings, business_hours |
| `orders_api` | Postgres (mesmo `DB_CONNECTION_STRING`) **+** Mongo (`push_tokens`) | Domínio de pedidos já está em Postgres, gerido por GORM AutoMigrate. Só os push tokens de notificação ainda estão soltos numa collection Mongo sem schema. |
| `delivery_api` | MongoDB (`MONGO_URI`/`MONGO_DATABASE`) | 100% Mongo — struct `OrderDTO`, cópia achatada do pedido para o motor de despacho. |
| `chat_api` | MongoDB (mesmo Mongo de cima) | 100% Mongo — struct `ChatMessage`. |
| `payment_api` | MongoDB (`PAYMENT_MONGO_DATABASE`) | Cobrança (PIX/cartão), carteira simples. |
| `Backend/Payment` | MongoDB (outro database dentro do mesmo Mongo) | Aprovação manual, risco, chargeback, carteira com histórico. **Duplica** pagamento e carteira do `payment_api`, sincronizado por fila Redis. |

O ponto mais sério: **`payment_api` e `Backend/Payment` são dois bancos de pagamento paralelos**, sincronizados por uma fila Redis que, segundo o próprio código, ignora mensagens silenciosamente se o Redis não estiver configurado — ou seja, existe hoje um caminho real de saldo não creditado sem erro visível.

## O que este pacote entrega

Scripts SQL testados de verdade (rodados duas vezes seguidas contra um Postgres limpo, sem erro) que:

1. Criam uma role de aplicação com privilégios mínimos (`00`).
2. Levam `push_tokens` do Mongo para o Postgres já existente (`01`).
3. Criam `delivery_solicitations`, substituindo a collection Mongo do `delivery_api` (`02`).
4. **Unificam** `payment_api` + `Backend/Payment` numa única tabela `payments` + `wallets` + `wallet_transactions` + tabelas de chargeback/payout/regras/admin (`03`).
5. Criam `chat_messages`, substituindo a collection Mongo do `chat_api` (`04`).
6. Criam auditoria genérica por trigger, com redação automática de colunas sensíveis (`05`).
7. Habilitam RLS em todas as tabelas, revogando acesso das roles públicas do Supabase (`06`).
8. Auditam o banco inteiro contra o código, mostrando o que sobra e o que falta (`07`).
9. Rodam uma bateria de testes automatizados que confirma que tudo funcionou (`08`).

**Isto cobre o BANCO.** Não cobre, e é importante ser honesto sobre isso, a **troca do código Go** que hoje lê e escreve nessas collections Mongo. SQL não reescreve `Backend/delivery_api`, `Backend/chat_api`, `Backend/payment_api` e `Backend/Payment` para usar GORM/Postgres em vez do driver Mongo — isso é um trabalho de código à parte, service por service.

## Ordem de corte recomendada (menor risco primeiro)

Não faça isso tudo de uma vez. Sugestão de ordem, do menos arriscado ao mais:

1. **`push_tokens`** (script 01) — baixíssimo risco, não é dado financeiro nem afeta fluxo em tempo real. Bom para validar o processo de corte (trocar o código para escrever em Postgres, rodar os dois em paralelo por alguns dias, depois desligar o Mongo).
2. **`chat_api`** (script 04) — sem dado financeiro, mas é tempo real (WebSocket). Testa o padrão de corte sob carga de leitura/escrita constante.
3. **`delivery_api`** (script 02) — mais sensível porque afeta o motor de despacho ao vivo. Recomenda-se rodar os dois bancos em paralelo (dual-write) por um tempo, comparando os resultados do matching antes de desligar o Mongo.
4. **Pagamentos** (script 03) — por último e com mais cuidado. Antes de apontar o código para a tabela unificada, é preciso um **script de migração de dados** (ETL) que reconcilie os registros que hoje representam o mesmo pagamento nos dois Mongos (provavelmente casando por `order_id`). Peça esse script como próximo passo, depois de validar o schema em homologação. Sugestão: manter os dois sistemas de pagamento escrevendo em paralelo (feature flag) por pelo menos um ciclo de fechamento financeiro completo antes de confiar 100% na tabela nova.

## Por que RLS não pode copiar o padrão `auth.uid()`

Ver comentário detalhado no topo de `sql/06_rls_seguranca.sql`. Resumo: este projeto não usa Supabase Auth (login é JWT próprio), então `auth.uid()` sempre retorna nulo aqui — uma policy baseada nisso pareceria segura mas não protegeria nada. A estratégia usada foi: RLS habilitado + só a role `app_backend` tem acesso + revogação de `anon`/`authenticated`. O controle fino de "usuário só vê o que é dele" continua no middleware do backend Go, que já existe (ver `docs/seguranca.md`).

## Coisas que uma IA (inclusive eu) tende a esquecer, e que já foram tratadas aqui

- RLS copiado de tutorial genérico sem checar se o projeto usa Supabase Auth de verdade.
- Redação de colunas sensíveis (senha, token de cartão) no próprio log de auditoria — sem isso, o audit_log vira um vazamento de dado sensível em vez de proteção.
- Testar os scripts de verdade contra um banco, não só descrever em teoria (todos os 9 scripts deste pacote rodaram duas vezes seguidas sem erro antes de chegar até você).
- Idempotência: pode rodar tudo de novo sem quebrar nada nem duplicar dado.
- Registrar cada mudança de estrutura (`schema_migrations`) e cada mudança de dado (`audit_log`) separadamente — são dois changelogs com propósitos diferentes.
- Role de aplicação com privilégios mínimos em vez do backend usar o superusuário do Postgres.
