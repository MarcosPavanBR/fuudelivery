# Hooks

Automação leve disparada em eventos do ciclo de vida da sessão do Claude Code — não substitui os agentes, só garante que certos passos nunca sejam esquecidos.

- `SessionStart`: roda `scripts/session-start.js` (se presente) para carregar contexto do projeto atual (branch, testes falhando na última execução conhecida).
- `PreToolUse` (Bash, comandos `git commit`): roda uma checagem rápida de `grep` por padrões de segredo antes de permitir o commit — ver `skills/credential-hygiene`. Isso é uma rede de segurança adicional, não substitui a revisão do agente `security-reviewer`.
- `Stop`: nenhum hook obrigatório por padrão; adicione um se quiser persistir um resumo de sessão.

Hooks aqui são propositalmente mínimos — a lógica pesada vive nos agentes e skills, não em scripts de hook difíceis de revisar.
