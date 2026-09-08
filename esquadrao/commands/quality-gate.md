---
description: Portão de qualidade completo antes de merge — build, testes, lint, revisão de código e segurança
argument-hint: (nenhum argumento necessário)
---

Rode em sequência, parando no primeiro bloqueador:
1. Build limpo.
2. Suíte de testes completa.
3. Lint/vet da linguagem relevante.
4. Agente `code-reviewer` (+ revisor específico da linguagem) no diff atual.
5. Agente `security-reviewer` se o diff tocar autenticação, pagamento, WebSocket ou dado pessoal.

Reporte um resumo final: pronto para merge, ou lista de bloqueadores por ordem de prioridade.
