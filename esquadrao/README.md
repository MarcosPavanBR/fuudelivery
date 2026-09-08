# Esquadrão

Uma equipa de engenharia para o Claude Code: agentes especializados, skills de domínio e comandos, organizados para planear, construir, revisar e proteger um projeto real — não uma demo genérica. O catálogo abaixo foi desenhado a partir de um caso concreto (plataforma de delivery multi-app + portal de conteúdo Next.js), não copiado de outro projeto.

`github.com/affaan-m/ECC` mostrou o formato: agentes + skills + comandos como uma unidade instalável no Claude Code. Este plugin segue o mesmo formato, com catálogo e conteúdo próprios, e conscientemente menor — 13 agentes e 18 skills reais em vez de uma centena de arquivos genéricos. O aviso do ECC vale igual aqui: mais skills instaladas não significa resultado melhor. Comece com um único pacote de regras (`rules/`) e vá ativando skills conforme a tarefa pedir.

## O que tem aqui

```
esquadrao/
├── .claude-plugin/
│   ├── plugin.json          # Manifesto do plugin
│   └── marketplace.json     # Catálogo para `/plugin marketplace add`
├── agents/                  # 13 subagentes especializados
├── skills/                  # 18 skills de domínio (SKILL.md)
├── commands/                # 14 comandos /slash
├── rules/                   # Sempre carregado — coding-style, git, testing, security
├── hooks/                   # PreToolUse: checagem leve de segredo antes de commit
├── scripts/lib/             # Utilitários Node.js compartilhados
├── tests/run-all.js         # Smoke test estrutural do próprio plugin
├── contexts/                # dev / review / research
├── examples/CLAUDE.md       # Exemplo de CLAUDE.md de projeto usando este plugin
└── mcp-configs/             # Exemplo de configuração de MCP servers
```

### Agentes (13)

| Agente | Quando usar |
|---|---|
| `planner` | Antes de qualquer feature/bug não trivial — gera plano revisável |
| `architect` | Decisões que afetam mais de um serviço/app |
| `code-reviewer` | Depois de qualquer código escrito |
| `security-reviewer` | Auth, pagamento, WebSocket, dado pessoal, pré-deploy |
| `tdd-guide` | Lógica nova, especialmente financeira |
| `build-error-resolver` | Build/CI quebrado |
| `go-reviewer` | Backend Go |
| `typescript-reviewer` | React/React Native/Next.js |
| `python-reviewer` | Scripts e serviços Python |
| `react-native-reviewer` | Apps Expo especificamente |
| `payments-reviewer` | Gateway, webhook, carteira, estorno |
| `e2e-runner` | Testes de ponta a ponta em fluxos críticos |
| `docs-updater` | Sincronizar doc com código real |

### Skills (18)

`coding-standards` · `security-checklist` · `tdd-workflow` · `verification-loop` · `go-patterns` · `go-testing` · `react-native-patterns` · `nextjs-patterns` · `nodejs-api-patterns` · `payment-gateway-integration` · `postgres-migrations` · `websocket-security` · `deployment-patterns` · `e2e-testing` · `seo-content-architecture` · `lgpd-compliance` · `docker-patterns` · `credential-hygiene`

### Comandos (14)

`/plan` `/code-review` `/security-scan` `/build-fix` `/quality-gate` `/test-coverage` `/update-docs` `/db-review` `/deploy-check` `/refactor-clean` `/e2e-test` `/lgpd-check` `/setup-pm` `/learn`

## Instalação

```bash
# Copie a pasta esquadrao/ para dentro do seu projeto, ou adicione como marketplace:
/plugin marketplace add ./esquadrao
/plugin install esquadrao
```

## Por que só 13 agentes e 18 skills, não 68 e 286

Cada agente e cada skill aqui carrega conteúdo específico, testável e verificável contra um caso real — não um título genérico com um parágrafo de preenchimento. Um catálogo de 286 skills só vale a pena se as 286 forem assim; caso contrário é ruído que compete por atenção com as que importam. Este pacote foi pensado para crescer por skill nova quando a necessidade aparecer de verdade, não para preencher uma tabela.

**Como estender**: use `/learn` depois de qualquer correção não trivial para capturar o padrão numa skill existente, ou peça um agente/skill/comando novo seguindo exatamente a estrutura dos existentes (frontmatter YAML + corpo focado em decisão, não em teoria genérica).

## Licença

MIT.
