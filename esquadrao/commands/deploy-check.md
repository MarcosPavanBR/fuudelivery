---
description: Checklist pré-deploy completo antes de subir para produção
argument-hint: (nenhum argumento necessário)
---

Rode o checklist de `skills/deployment-patterns` antes do deploy: CI verde, `security-reviewer` nas mudanças desde o último deploy (não só na última), variáveis de ambiente conferidas contra `.env.example`, e plano de rollback documentado se houver migração de banco envolvida.
