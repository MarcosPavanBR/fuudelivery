# Guia de Contribuição - FuuDelivery

## Visão Geral

O FuuDelivery é um projeto open-source que busca democratizar o delivery brasileiro com taxas justas. Contribuições são bem-vindas!

## Como Contribuir

### 1. Fork o Repositório
```bash
# No GitHub, clique em Fork
git clone https://github.com/SEU-USER/fuudelivery.git
cd fuudelivery
git remote add upstream https://github.com/MarcosPavanBR/fuudelivery.git
```

### 2. Crie uma Branch
```bash
git checkout -b feature/nova-funcionalidade
# ou
git checkout -b fix/correcao-de-bug
```

### 3. Desenvolva
- Siga os padrões de código (ver abaixo)
- Teste suas mudanças
- Faça commits descritivos

### 4. Abra um Pull Request
- Descreva o que mudou e por quê
- Link issues relacionadas
- Aguarde review

## Padrões de Código

### Go
```bash
# Formatação obrigatória
gofmt -s -w .

# Lint
golangci-lint run ./...

# Testes
go test ./...
```

**Regras:**
- Comentários godoc em todas as funções exportadas
- Nunca ignorar erros com `_`
- Usar `slog` para logs (nunca `fmt.Println`)
- Tratar erros com `fmt.Errorf("contexto: %w", err)`

### React Native / TypeScript
```bash
# Lint
npm run lint

# Formatação
npx prettier --write .

# Testes
npm test
```

**Regras:**
- TypeScript estrito (nunca `any`)
- Componentes funcionais com hooks
- Nomes em `camelCase` (funções/variáveis) ou `PascalCase` (componentes)
- Um componente por arquivo

### Commits

Padrão [Conventional Commits](https://www.conventionalcommits.org/):

```
tipo(escopo): descrição curta

tipo: feat, fix, docs, style, refactor, test, chore
escopo: auth, payment, orders, frontend, etc.

Exemplos:
feat(payment): adicionar split automático
fix(auth): corrigir validação de token expirado
docs(readme): atualizar guia de setup
```

## Estrutura do Repositório

```
fuudelivery/
├── Backend/           # Serviços Go
│   ├── Payment/       # Núcleo financeiro
│   └── ...outros módulos
├── Frontend/
│   ├── WebAdmin/      # Painel administrativo (React)
│   ├── WebRestaurant/ # Painel do restaurante (React)
│   ├── AppComida/     # App do cliente (React Native)
│   ├── AppEntrega/    # App do entregador (React Native)
│   └── AppRestaurante/# App do restaurante (React Native)
├── cmd/fuudelivery/   # Monolito principal
├── docs/              # Documentação
└── scripts/           # Scripts de build/deploy
```

## Ambiente de Desenvolvimento

### Pré-requisitos
- Go 1.23+
- Node.js 18+
- Docker (para testes com testcontainers)
- Git

### Setup
```bash
# 1. Clonar e configurar
git clone https://github.com/MarcosPavanBR/fuudelivery.git
cd fuudelivery
cp .env.example .env

# 2. Backend
cd cmd/fuudelivery
go mod tidy
go run main.go

# 3. Frontend (em outro terminal)
cd Frontend/WebAdmin
npm install
npm run dev
```

## Areas de Contribuição

### 🔴 Prioridade Alta
- [ ] Testes E2E dos apps React Native
- [ ] Notificações push (Firebase)
- [ ] Tratamento de erros nos apps
- [ ] App do restaurante (React Native)

### 🟡 Prioridade Média
- [ ] Documentação da API (OpenAPI)
- [ ] Melhoria de performance
- [ ] Acessibilidade (a11y)
- [ ] Internacionalização (i18n)

### 🟢 Baixa
- [ ] Temas/visual customizável
- [ ] Modo offline
- [ ] Analytics avançado

## Issues e Bugs

### Como Reportar
1. Busque issues existentes
2. Se não encontrar, abra nova issue
3. Use template: descrição, passos, comportamento esperado, ambiente

### Labels
- `bug`: Bug confirmado
- `feature`: Nova funcionalidade
- `documentation`: Documentação
- `good first issue`: Bom para iniciantes
- `help wanted`: Precisa de ajuda

## Code Review

### Expectativas
- Reviews em até 48h
- No mínimo 1 approve para merge
- CI deve passar
- Testes devem cobrir mudanças

### Dicas para um bom PR
- PRs pequenos (< 500 linhas)
- Descrição clara do problema e solução
- Screenshots/gifs para mudanças visuais
- Testes para mudanças de lógica

## Licença

Projeto sob licença MIT. Ao contribuir, você concorda que seu código será distribuído sob esta licença.

## Contato

- **Issues**: GitHub Issues
- **Discussions**: GitHub Discussions
- **Email**: (se disponível)
