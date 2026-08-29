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

1. ✅ **`push_tokens`** (script 01) — baixíssimo risco, não é dado financeiro nem afeta fluxo em tempo real. Bom para validar o processo de corte (trocar o código para escrever em Postgres, rodar os dois em paralelo por alguns dias, depois desligar o Mongo).
2. ✅ **`chat_api`** (script 04) — sem dado financeiro, mas é tempo real (WebSocket). Testa o padrão de corte sob carga de leitura/escrita constante.
3. ✅ **`delivery_api`** (script 02) — código já migrado: escrita/leitura primárias em Postgres, dual-write Mongo best-effort. Motor de despacho usa o read-model GORM.
4. ✅ **Pagamentos** (script 03) — código migrado: todos os handlers (`payment_api`) usam GORM/Postgres como primário com dual-write best-effort no Mongo; ETL one-shot disponível em `cmd/etl-payments` (idempotente, não apaga nada). Teste E2E reescrito para Postgres via testcontainers.

5. ✅ **Recursos de pedidos** — código migrado: TODOS os handlers do `orders_api` usam Postgres primário com dual-write best-effort no Mongo. Detalhes no corte 5 da tabela abaixo.

## Status dos cortes (atualizado em 2026-08-23)

| Corte | Status | Onde |
|---|---|---|
| 1. `push_tokens` | ✅ **Código migrado** — escrita primária em Postgres (`models.PushToken`, upsert por user_id+user_type), dual-write Mongo best-effort; leitura 100% Postgres. Bônus: corrigido caminho de envio de push que consultava Mongo por `user_phone` (campo nunca gravado pela escrita — provável caminho morto); agora resolve phone → client_id → tokens. | `orders_api/app/models/push_token.go`, `handlers/notifications.go`, `handlers/orders.go` |
| 2. `chat_messages` | ✅ **Código migrado** — escrita primária em Postgres (GORM AutoMigrate + tabela sql/04), dual-write Mongo best-effort; leitura e MarkAsRead 100% Postgres. ID agora é BIGSERIAL (era ObjectID). | `chat_api/app/models/{message,database}.go`, `handlers/chat.go` |
| 3. `delivery_solicitations` | ✅ **Código migrado** — Postgres primário, dual-write legado | `delivery_api/app/handlers/solicitations.go`, `dispatch_handler.go` |
| 4. Pagamentos/carteiras | ✅ **Código migrado + ETL pronto** — handlers 100% GORM/Postgres com dual-write best-effort; lazy-ETL "on first touch" para carteiras + ferramenta `cmd/etl-payments` para importar histórico completo (payments, wallets, wallet_ledger → wallet_transactions) antes de desligar o Atlas. Suíte E2E reescrita para Postgres (testcontainers). | `payment_api/app/handlers/*`, `payment_api/app/models/{payment,wallet}.go`, `cmd/etl-payments/` |
| 5. Recursos de pedidos | ✅ **Código migrado** — a tabela `order_documents` guarda o payload completo (JSONB) + colunas tipadas para filtros/índices (`establishment_id`, `user_phone`, `status`, `pickup_code`, agendamento). O ID público continua no formato legado (ObjectID hex) — nenhum cliente precisou mudar. Leitura Postgres-first com **lazy import** do Mongo (pedido antigo é importado no primeiro acesso) e fallback de listagem enquanto o ETL não roda. Teste de integração atualizado: valida persistência em Postgres, dual-write no Mongo e geração de pickup code. | `orders_api/app/models/order_document.go`, `handlers/orders_pg.go`, `handlers/{orders,pickup_code,review,scheduling,reorder}.go` |

**Como desligar o Mongo depois:** remova as chamadas `ConnectMongoDatabase()` e os blocos "dual-write" marcados nos handlers; então remova `MONGO_URI` do Render. Os blocos legados estão todos marcados com comentários "dual-write temporário" no código.

### Runbook do ETL de pagamentos (`cmd/etl-payments`)

Rode UMA vez (idempotente — pode repetir sem duplicar) antes de pausar o Atlas:

```bash
cd cmd/etl-payments && GOWORK=off go build -o etl-payments .
DB_CONNECTION_STRING="postgres://..." \
MONGO_URI="mongodb+srv://..." \
PAYMENT_MONGO_DATABASE="fuudelivery_payments" \
./etl-payments
```

O que faz: importa `payments` (dedup por abacatepay_id ou order_id+amount), cria carteiras que só existem no Mongo (**nunca sobrescreve saldo Postgres**) com lançamento de auditoria, e importa o `wallet_ledger` antigo para `wallet_transactions` (dedup por tupla característica). Não apaga nada em nenhum banco. Confira o resumo impresso ao final contra os totais do Atlas.

Depois de 1 ciclo financeiro de dual-write observado + ETL validado: execute o ETL de pedidos (abaixo) e só então remova o Atlas.

### Runbook do ETL de pedidos

O corte 5 tem lazy import por pedido individual, mas para desligar o Atlas sem lacunas em LISTAGENS (pedidos antigos só aparecem nas listas depois do ETL), rode uma vez:

```bash
cd cmd/etl-orders && GOWORK=off go build -o etl-orders .
DB_CONNECTION_STRING="postgres://..." \
MONGO_URI="mongodb+srv://..." \
MONGO_DATABASE="fuudelivery" \
./etl-orders
```

Idempotente (dedup por `legacy_id`), não apaga nada nos dois lados. Confira os totais impressos contra o Atlas antes de pausá-lo.

### ✅ ETLs executados em produção (23/08/2026)

| Ferramenta | Resultado |
|---|---|
| `etl-payments` | 1 pagamento importado; 0 carteiras a criar (Atlas sem carteiras fora do Postgres); 0 ledger antigo |
| `etl-orders` | 0 documentos na collection `orders` do Atlas (nenhum pedido pré-migração) — nada a importar |

Durante o ETL foi descoberto e **corrigido em produção** um bug de schema: tabelas legadas vazias com `id TEXT` (era Mongo) bloqueavam o `CREATE TABLE IF NOT EXISTS` dos scripts 01–03, e o AutoMigrate do GORM não altera coluna existente — a tabela `payments` tinha `id TEXT`, quebrando TODO insert de pagamento. Reparo aplicado manualmente e registrado como script idempotente `sql/09_reparo_tabelas_legado_texto.sql` para ambientes futuros.

Achados adicionais da auditoria de 23/08:

1. `run_all.sh` rodava o reparo 09 DEPOIS dos scripts 01–03 — ordem trocada (09 agora roda logo após o 00, como ele mesmo exige).
2. As colunas `kind`/`destination` do ledger só existiam via AutoMigrate; versionadas em `sql/10_wallet_ledger_kind.sql` (aplicado em produção).
3. A tabela `schema_migrations` não existia no banco de produção (script 00 nunca rodou lá). Criada manualmente; o changelog de produção começa na versão 10. Recomendação: rodar `sql/run_all.sh --so-testes` e depois avaliar aplicar 00–08 completos em janela planejada (06/RLS merece revisão antes, pois muda privilégios).

### ⏰ Critério com data para desligar o Atlas

"Esperar 1 ciclo financeiro" sem data vira "esperar para sempre". Marcado:

- **Data limite de revisão: 22/09/2026** (30 dias após o ETL de 23/08). Na revisão: re-rodar ambos os ETLs (agora seguros para re-execução — pulam linhas já existentes), comparar totais contra o Atlas e decidir a remoção do `MONGO_URI`.
- Antes de pausar o Atlas, **confirmar que as collections do antigo `Backend/Payment`** (`chargebacks`, `chargeback_evidence`, `payout_requests`, `payment_approval_rules`, `payment_admin_users`) estão vazias — o `cmd/etl-payments` não as migra por não haver caminho primário nelas; se houver dado real, estender o ETL primeiro.

## Por que RLS não pode copiar o padrão `auth.uid()`

Ver comentário detalhado no topo de `sql/06_rls_seguranca.sql`. Resumo: este projeto não usa Supabase Auth (login é JWT próprio), então `auth.uid()` sempre retorna nulo aqui — uma policy baseada nisso pareceria segura mas não protegeria nada. A estratégia usada foi: RLS habilitado + só a role `app_backend` tem acesso + revogação de `anon`/`authenticated`. O controle fino de "usuário só vê o que é dele" continua no middleware do backend Go, que já existe (ver `docs/seguranca.md`).

## Coisas que uma IA (inclusive eu) tende a esquecer, e que já foram tratadas aqui

- RLS copiado de tutorial genérico sem checar se o projeto usa Supabase Auth de verdade.
- Redação de colunas sensíveis (senha, token de cartão) no próprio log de auditoria — sem isso, o audit_log vira um vazamento de dado sensível em vez de proteção.
- Testar os scripts de verdade contra um banco, não só descrever em teoria (todos os 9 scripts deste pacote rodaram duas vezes seguidas sem erro antes de chegar até você).
- Idempotência: pode rodar tudo de novo sem quebrar nada nem duplicar dado.
- Registrar cada mudança de estrutura (`schema_migrations`) e cada mudança de dado (`audit_log`) separadamente — são dois changelogs com propósitos diferentes.
- Role de aplicação com privilégios mínimos em vez do backend usar o superusuário do Postgres.
