---
description: Revisa uma migração de schema Postgres antes de aplicar
argument-hint: [caminho da migração]
---

Revise a migração em $ARGUMENTS usando `skills/postgres-migrations`. Confirme especificamente: tipo consistente com a tabela referenciada, migração reversível (up/down), e se popula default antes de tornar coluna NOT NULL quando a tabela já tem dados.
