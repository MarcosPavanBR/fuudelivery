# FuuDelivery — guia para o Claude Code

Este repositório usa o plugin **[esquadrao](./esquadrao/)**: agentes especializados,
skills de domínio e comandos `/slash` para planear, construir, revisar e proteger
este projeto. Instale uma vez por ambiente:

```bash
/plugin marketplace add ./esquadrao
/plugin install esquadrao
```

## Stack real deste projeto

- **Backend**: Go (`go.work`), monorepo de módulos — `Backend/auth_api`,
  `Backend/chat_api`, `Backend/delivery_api`, `Backend/orders_api`,
  `Backend/payment_api`, monólito `cmd/fuudelivery` e pacotes partilhados
  (`pkg/gateway`, `pkg/queue`, `pkg/featureflags`, `pkg/health`,
  `pkg/sanitizer`).
- **Banco**: consolidação para **Postgres único (Supabase)** em andamento —
  ver `docs/ARQUITETURA-BANCO-UNICO.md` e `skills/fuudelivery-banco-unico/`
  (skill própria do projeto, complementar à `esquadrao/skills/postgres-migrations`).
  **`Backend/Payment` está arquivado** — histórico, não código ativo; o
  pagamento vivo é `payment_api`. Nunca crie tabela nova em MongoDB.
- **Mobile**: Expo/React Native — `Frontend/AppComida`, `Frontend/AppEntrega`,
  `Frontend/AppRestaurante`.
- **Web**: React + **Vite** — `Frontend/WebAdmin`, `Frontend/WebRestaurant`.
  (A skill `esquadrao/skills/nextjs-patterns` é para o portal de conteúdo
  Next.js separado do usuário, não para estes dois — não se aplica aqui.)
- **Pagamento**: gateway multi-provedor com router central (`pkg/gateway`),
  webhooks assinados, carteira/wallet com idempotência.

## Convenções deste projeto (sempre ativas)

Ver `esquadrao/rules/common/` para o conjunto completo — resumo:

- Autorização por recurso é obrigatória em todo handler novo, REST ou
  WebSocket, sem exceção "só para MVP" — já houve IDOR real aqui.
- Nenhuma credencial real em arquivo de documentação, `.env` commitado ou
  backup de banco — sempre placeholder. Já houve vazamento recorrente
  (`CREDENTIALS.md`, depois `DOCUMENTATION.md`).
- Todo webhook de pagamento verifica assinatura (HMAC) antes de processar.
- Toda operação financeira (carteira, webhook) tem teste de idempotência.

## Como pedir trabalho

- Feature nova ou mudança em mais de um arquivo: comece com `/plan`.
- Código escrito: `code-reviewer` (+ revisor de linguagem) roda automaticamente
  antes de considerar pronto — ou peça `/code-review`.
- Autenticação, pagamento, WebSocket ou dado pessoal: `security-reviewer`
  entra antes do merge — ou peça `/security-scan`.
- Antes de merge: `/quality-gate`. Antes de deploy: `/deploy-check`.
- Migração de schema: `/db-review <arquivo>`.
- Depois de qualquer correção não trivial: `/learn` para capturar o padrão
  numa skill existente em vez de deixar o aprendizado se perder.
