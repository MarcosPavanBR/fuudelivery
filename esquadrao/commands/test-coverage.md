---
description: Analisa cobertura de teste do módulo/app indicado e sugere onde priorizar
argument-hint: [módulo ou app, ex: AppEntrega]
---

Analise a cobertura de teste atual de $ARGUMENTS. Se a cobertura for zero ou muito baixa, use a priorização do skill `tdd-workflow`: fluxo de maior risco financeiro primeiro, depois máquina de estado crítica, depois autenticação, só então telas de exibição pura.

Não gere testes triviais só para inflar percentual de cobertura — cada teste sugerido deve corresponder a um comportamento real que pode quebrar.
