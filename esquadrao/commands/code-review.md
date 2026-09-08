---
description: Revisão de código do diff atual ou de um caminho específico
argument-hint: [caminho opcional, padrão: diff atual]
---

Revise as mudanças em $ARGUMENTS (ou o diff atual em relação à branch principal, se nenhum caminho for dado) usando o agente `code-reviewer`. Se as mudanças tocarem Go, TypeScript/React Native, ou Python, invoque também o revisor específico da linguagem em paralelo.

Reporte apenas problemas reais, classificados por severidade (bloqueador / importante / nit), com arquivo:linha.
