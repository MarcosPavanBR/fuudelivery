# Segurança (sempre ativa — os três erros já vistos aqui)

1. Autorização por recurso em todo handler que recebe um ID (REST e WebSocket), não apenas autenticação.
2. Nenhum segredo real em `.env`, doc de configuração, ou backup versionado.
3. Todo webhook de pagamento verifica assinatura antes de processar.

Checklist completo: `skills/security-checklist`. Estes três vêm primeiro em qualquer revisão, sempre.
