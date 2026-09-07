---
description: Diagnostica e corrige um erro de build ou de CI
argument-hint: [comando que falhou, ex: "go build ./..." ou "npm run build"]
---

Reproduza o erro rodando $ARGUMENTS, capture o output completo, e delegue a correção ao agente `build-error-resolver`. Corrija a causa raiz, não o sintoma — se o erro vier de uma divergência de schema/tipo entre módulos, sinalize para o agente `architect` em vez de aplicar um patch local.
