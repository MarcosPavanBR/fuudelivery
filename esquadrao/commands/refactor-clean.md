---
description: Remove código morto e simplifica sem mudar comportamento
argument-hint: [caminho ou módulo]
---

Analise $ARGUMENTS em busca de código morto (funções não referenciadas, imports não usados, branches inalcançáveis) e duplicação óbvia. Aplique só mudanças que não alteram comportamento observável — se uma simplificação exigir mudar comportamento, pare e proponha como plano separado em vez de aplicar direto.

Rode a suíte de testes antes e depois para confirmar que nada mudou de fato.
