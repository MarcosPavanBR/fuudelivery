---
description: Revisão de segurança focada — autenticação, autorização, pagamentos, WebSocket e credenciais
argument-hint: [caminho opcional, padrão: repositório inteiro]
---

Rode o agente `security-reviewer` sobre $ARGUMENTS (ou o repositório inteiro, se nada for especificado), seguindo o checklist em `skills/security-checklist`.

Priorize, nesta ordem: credenciais versionadas, autorização por recurso em WebSocket, verificação de assinatura em webhooks de pagamento. Diga explicitamente o que foi verificado e não encontrou problema, não apenas o que encontrou.
