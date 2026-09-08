---
description: Sincroniza documentação (README, docs/ARQUITETURA) com o estado real do código após uma mudança
argument-hint: [área ou arquivo de doc a verificar]
---

Delegue ao agente `docs-updater` a verificação de $ARGUMENTS contra o código atual. Confirme especificamente se algum módulo descrito como ativo já foi arquivado, e se alguma checagem de segurança descrita como existente realmente existe no handler correspondente — este projeto já teve os dois tipos de divergência.
