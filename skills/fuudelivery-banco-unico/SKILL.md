---
name: fuudelivery-banco-unico
description: Regras obrigatórias de banco de dados e segurança para o projeto FuuDelivery (github.com/MarcosPavanBR/fuudelivery). USE ESTA SKILL sempre que for tocar em qualquer coisa relacionada a: banco de dados, schema, migração, tabela nova, coluna nova, MongoDB, Postgres, Supabase, RLS, auditoria, segurança de dados, pagamentos/carteira/wallet, ou qualquer arquivo dentro de sql/, docs/banco-de-dados.md, ou Backend/*/app/models. Mesmo que o pedido pareça pequeno ("só adiciona uma coluna", "corrige esse campo"), consulte esta skill antes de escrever SQL ou mexer em model Go deste projeto — ela existe justamente porque essas mudanças "pequenas" são onde segurança e consistência costumam ser esquecidas.
---

# FuuDelivery — banco único, com segurança e histórico

Este projeto está em processo de consolidação: sair de "dois bancos que não
se falam" (Postgres para auth/pedidos + MongoDB para entrega/chat/pagamentos,
sendo que **pagamento existe duplicado em dois Mongos diferentes**) para
**um único Postgres (Supabase)**. Ver `docs/ARQUITETURA-BANCO-UNICO.md` para
o diagnóstico completo e `docs/banco-de-dados.md` para o dicionário de dados
tabela por tabela.

## Regras obrigatórias, sem exceção

1. **Nunca crie uma tabela nova em MongoDB.** Se o pedido for "salva isso em
   algum lugar" e não especificar o banco, a resposta é Postgres. MongoDB
   está sendo eliminado deste projeto, não expandido.

2. **Toda tabela de negócio nova precisa de três coisas no mesmo script**,
   nesta ordem — copie o padrão de `sql/03_dominio_pagamentos.sql` como
   referência:
   - `CREATE TABLE IF NOT EXISTS ...` com colunas explicadas via
     `COMMENT ON TABLE`/`COMMENT ON COLUMN` quando o nome não for óbvio.
   - Trigger de auditoria (`fn_audit_trigger`, ver `sql/05_audit_log.sql`).
   - RLS habilitado + policy `backend_full_access` restrita à role
     `app_backend` (ver `sql/06_rls_seguranca.sql`).

3. **Nunca copie o padrão de RLS `auth.uid() = user_id` neste projeto.**
   Este projeto não usa Supabase Auth — login é JWT próprio em `auth_api`
   (bcrypt + HS256). `auth.uid()` aqui sempre retorna nulo. Esse é o erro
   mais comum de IA generalista mexendo neste projeto especificamente; se
   você (Claude, ou qualquer IA) estiver prestes a escrever essa linha,
   pare e releia `sql/06_rls_seguranca.sql`.

4. **Coluna sensível (senha, token, dado de pagamento) sempre entra em
   `audit_redacted_columns`** no mesmo script que cria a coluna. Sensível
   inclui: qualquer `password`/`password_hash`, `card_token`,
   `pix_copy_paste`, `qr_code_base64`, e qualquer coisa que dê acesso a
   dinheiro ou identidade.

5. **Toda mudança de estrutura é um script novo e numerado em `sql/`**,
   terminando com `INSERT INTO schema_migrations`. Nunca edite um script já
   aplicado em produção. Ver `docs/CHANGELOG_BANCO.md` para o processo
   completo passo a passo.

6. **`payments`, `wallets` e `wallet_transactions` são o domínio mais
   sensível do projeto.** `wallet_transactions` é INSERT-only — nunca
   UPDATE nem DELETE (estorno é uma nova linha de sinal contrário, não uma
   edição da linha antiga). Qualquer mudança aqui merece revisão dobrada,
   porque este domínio já teve um bug real de sincronização (fila Redis que
   ignora mensagem silenciosamente se não configurada — é por isso que
   payment_api e Backend/Payment foram unificados numa tabela só, para
   eliminar a necessidade dessa sincronização).

7. **Antes de entregar qualquer script SQL para este projeto, teste de
   verdade.** Não descreva o que o script "deveria" fazer — rode contra um
   Postgres real (local ou de homologação) e mostre o resultado. Rode duas
   vezes seguidas para confirmar idempotência. Isso é o padrão que este
   projeto já espera (ver histórico de `sql/00` a `sql/08`, todos testados
   assim).

8. **Depois de aplicar uma migração, rode `sql/08_testes.sql`** (ou peça
   para adicionar um novo bloco de teste nele, seguindo o padrão
   `RAISE NOTICE '[PASS] ...'` / `RAISE WARNING '[FAIL] ...'`).

9. **Documentação anda junto com código, no mesmo commit/entrega**:
   qualquer tabela ou coluna nova entra em `docs/banco-de-dados.md` na hora.
   Uma tabela sem documentação é considerada trabalho incompleto neste
   projeto.

10. **Antes de considerar uma tabela "lixo" e sugerir apagar**, rode
    `sql/07_auditoria_tabelas_orfas.sql` e siga o processo de quarentena
    (renomear com prefixo `zz_deprecated_`, esperar, só depois `DROP`).
    Nunca sugira `DROP TABLE` direto.

## Checklist de segurança que uma IA generalista costuma esquecer
## (releia isto antes de dizer "pronto, terminei")

- [ ] A tabela nova tem RLS habilitado E uma policy de fato (RLS habilitado
      sem policy nenhuma bloqueia geral, o que parece seguro mas quebra a
      aplicação — teste que o `app_backend` consegue mesmo ler/escrever).
- [ ] Nenhuma rota de API do Go devolve coluna sensível em JSON de resposta.
- [ ] A role usada pelo backend (`app_backend`) tem só os privilégios que
      precisa — nunca sugira usar o superusuário do Postgres "por
      simplicidade".
- [ ] Se a mudança envolve dinheiro (payments/wallets/payout), pergunte
      explicitamente sobre reconciliação com o estado anterior antes de
      assumir que pode migrar dado histórico automaticamente.
- [ ] Se a mudança tocar em código Go que hoje usa `mongo-driver`, avise
      claramente que o SQL sozinho não é suficiente — o handler/service Go
      também precisa ser reescrito para GORM/Postgres, e isso é um corte
      (cutover) que merece plano de rollback. Ver "Ordem de corte" em
      `docs/ARQUITETURA-BANCO-UNICO.md`.

## Arquivos de referência deste pacote

- `docs/ARQUITETURA-BANCO-UNICO.md` — diagnóstico completo e plano de corte.
- `docs/banco-de-dados.md` — dicionário de dados, tabela por tabela.
- `docs/CHANGELOG_BANCO.md` — processo passo a passo para registrar mudanças.
- `sql/00` a `sql/08` — os scripts de consolidação, testados e comentados;
  use como padrão de estilo para qualquer script novo.
- `sql/run_all.sh` — runner com confirmação manual antes de alterar schema.
