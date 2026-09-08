# Exigência mínima de teste (sempre ativa)

- Toda função pública nova tem teste correspondente.
- Toda operação financeira (carteira, webhook de pagamento) tem teste de idempotência (chamar duas vezes, mesmo efeito).
- Antes de reportar uma tarefa como concluída, rode a suíte inteira — não só o teste novo. Ver `skills/verification-loop`.
