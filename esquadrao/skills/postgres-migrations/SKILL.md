---
name: postgres-migrations
description: Padrões de migração de schema PostgreSQL — tipos consistentes entre módulos, migrações reversíveis, e migração de dados de produção com verificação prévia. Use ao alterar schema ou migrar dados entre módulos/serviços.
---

# Migrações Postgres

## Consistência de tipo
- Antes de criar uma tabela nova que referencia outra (ex: `payments` referenciando `orders`), confirme o tipo real da chave primária da tabela referenciada (`UUID` vs `TEXT`) — divergência de tipo entre módulos migrados em momentos diferentes já causou incidente real de produção aqui (inserção bloqueada em `payments` por incompatibilidade de tipo com `id`).

## Migrações reversíveis
- Toda migração tem `up` e `down` — mesmo que o `down` nunca seja usado em produção, ele documenta a intenção e permite teste local seguro.
- Migração que adiciona coluna `NOT NULL` a uma tabela com dados existentes precisa de um valor default ou de um passo intermediário (adicionar nullable, popular, depois tornar `NOT NULL`).

## Migração de dados entre módulos
- Antes de migrar dados de um módulo antigo para um novo (ex: de um serviço arquivado para o serviço único atual), rode a migração em modo dry-run que reporta quantas linhas seriam afetadas e quantas falhariam, antes de rodar de verdade.
- Nunca delete a tabela/dado antigo até confirmar que o novo caminho está em produção e estável por um período razoável.

## Backups
- Backup de banco (`*.json`, `*.sql`, dump) nunca é commitado no repositório — vai para storage próprio com controle de acesso, e nunca contém hash de senha de usuário sem necessidade explícita e criptografia adicional.
