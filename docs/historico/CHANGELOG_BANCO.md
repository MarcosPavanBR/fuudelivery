# Como registrar uma mudança no banco do FuuDelivery

Este projeto tem **dois changelogs automáticos** (não precisa criar um terceiro
arquivo manual toda vez):

| O que mudou | Onde fica registrado | Automático? |
|---|---|---|
| Estrutura (nova tabela, nova coluna, novo índice) | tabela `schema_migrations` | Sim, desde que o script novo termine com `INSERT INTO schema_migrations (...)` |
| Dado (uma linha foi criada/alterada/apagada) | tabela `audit_log` | Sim, via trigger, para toda tabela auditada |

## Processo para uma mudança de ESTRUTURA nova (ex: nova coluna)

1. Crie um arquivo novo em `sql/`, com o próximo número da sequência (ex:
   se o último foi `08_testes.sql`, o seu é `09_nome_da_mudanca.sql`).
   **Nunca edite um script já aplicado em produção** — mesmo que pareça só
   um ajuste pequeno, crie um script novo. Isso é o que torna o histórico
   confiável.
2. Escreva o SQL usando `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS` sempre
   que possível, para o script ser seguro de rodar mais de uma vez
   (idempotente).
3. Se a tabela nova/alterada é uma tabela de negócio (não é infraestrutura
   de migração), lembre de:
   - Anexar o trigger de auditoria (copie o padrão do bloco `DO $$ ... $$`
     em `sql/05_audit_log.sql`).
   - Habilitar RLS e a policy `backend_full_access` (copie o padrão de
     `sql/06_rls_seguranca.sql`).
   - Se a tabela tem coluna sensível (senha, token, dado de cartão),
     adicionar em `audit_redacted_columns`.
4. Termine o script com:
   ```sql
   INSERT INTO schema_migrations (version, description)
   VALUES ('09_nome_da_mudanca', 'descrição curta do que mudou')
   ON CONFLICT (version) DO NOTHING;
   ```
5. Teste localmente (ou em homologação) antes de aplicar em produção —
   rode o script duas vezes seguidas para confirmar que é idempotente.
6. Atualize `docs/banco-de-dados.md` no MESMO commit, descrevendo a tabela
   ou coluna nova.
7. Rode `sql/08_testes.sql` depois de aplicar, para confirmar que nada
   quebrou.

## Consultando o histórico

```sql
-- O que já foi aplicado, em ordem:
SELECT version, description, applied_by, applied_at
  FROM schema_migrations ORDER BY applied_at;

-- Tudo que aconteceu com um pagamento específico:
SELECT operation, before_data, after_data, changed_by, changed_at
  FROM audit_log
 WHERE table_name = 'payments' AND row_pk = '123'
 ORDER BY changed_at;

-- Quem mexeu em quê nas últimas 24h:
SELECT table_name, operation, changed_by, changed_at
  FROM audit_log
 WHERE changed_at > now() - interval '24 hours'
 ORDER BY changed_at DESC;
```
