# Política de Segurança - FuuDelivery

## Visão Geral

O FuuDelivery trata dados financeiros e pessoais de milhões de usuários. A segurança é prioridade máxima.

## Vulnerabilidades Corrigidas

| Vulnerabilidade | Status | Commit |
|----------------|--------|--------|
| JWT sem validação de algoritmo | ✅ Corrigido | `algorithm confusion prevention` |
| Senhas sem bcrypt | ✅ Corrigido | `bcrypt em todas as senhas` |
| Webhook sem HMAC | ✅ Corrigido | `hmac.Equal (comparação constante)` |
| Rate limit ausente | ✅ Corrigido | `TrustedProxies + c.IP()` |
| Upload sem ownership | ✅ Corrigido | `verificação de establishment_id` |
| RBAC ausente em pagamentos | ✅ Corrigido | `AdminRequired() em 8 rotas` |
| `.env` no repositório | ✅ Corrigido | `.gitignore` +`.env.example` |
| Frete escolhido pelo cliente (`distance` no corpo) | ✅ Corrigido | `preço por região de CEP, decidido no servidor` |

## Riscos aceitos (conhecidos, não corrigidos)

Registrados aqui de propósito: risco conhecido e escrito é decisão; risco
conhecido e não escrito vira surpresa na produção de outra pessoa.

### CEP de entrega vem do corpo da requisição

**Onde:** `orders_api/app/handlers/delivery.go` — `resolveBaseFee()` usa
`Location.Cep` para achar a faixa de preço da região.

**O que dá para fazer:** mandar o CEP de uma região barata junto com um
logradouro de uma região cara. O entregador vai ao endereço real e a
plataforma cobrou o frete errado.

**Por que está aceito:**

- É estritamente melhor que o estado anterior, em que bastava mandar
  `"distance": 0` para pagar só a taxa fixa, sem deixar rastro nenhum.
- A mentira fica registrada no pedido: CEP e logradouro/bairro incoerentes,
  à vista do restaurante e do admin.
- Não existe frete grátis por essa via — sem região que case, o cálculo cai
  na taxa por km do estabelecimento, nunca em zero.

**O que fecharia de vez:** validar o CEP contra o ViaCEP no próprio servidor,
com cache e *fail-closed* na taxa mais cara. É uma chamada externa dentro do
checkout — custo, latência e mais um ponto de falha no caminho do dinheiro.
Decisão adiada de propósito até haver dado: quantos pedidos chegam com CEP
incoerente com o logradouro.

## Controles de Segurança

### Autenticação
- **JWT**: HS256 com secret obrigatório (min 32 caracteres)
- **Expiração**: Tokens expiram em 24h
- **Refresh**: Endpoint para renovar token sem re-login
- **bcrypt**: Custo padrão (10 rounds) para todas as senhas

### Autorização (RBAC)
- **admin**: Acesso total (aprovar pagamentos, configurar regras)
- **restaurant**: Gerenciar próprio restaurante e pedidos
- **deliverer**: Aceitar e entregar pedidos
- **client**: Criar pedidos e acompanhar entregas

### Controle de Acesso
- **Ownership**: Usuários só acessam seus próprios recursos
- **IDOR protection**: Verificação de dono em todos os endpoints
- **Middleware**: `AuthRequired()` + `AdminRequired()`

### Proteção contra Ataques

#### SQL Injection
- GORM com parâmetros preparados (nunca concatenação)
- Validação de input antes de queries

#### XSS
- Sanitização de HTML em inputs
- Content Security Policy nos frontends

#### CSRF
- Tokens CSRF em formulários
- SameSite cookies

#### Rate Limiting
- Login: 10 tentativas/minuto
- Pagamentos: 20/minuto
- Webhook: 100/minuto
- IP real via TrustedProxies (não header forjável)

### Webhooks
- HMAC-SHA256 em todas as assinaturas
- Comparação de tempo constante (`hmac.Equal`)
- Validação de timestamp (replay attack protection)

## Rotação de Credenciais

### Processo
1. Gerar novas credenciais nos painéis dos serviços
2. Atualizar no Render Dashboard (Environment)
3. Deploy automático atualiza o serviço
4. Revogar credenciais antigas

### Credenciais para Rotacionar
- MongoDB Atlas: Database Access → Users
- Supabase: Settings → API
- Redis: Render Dashboard → Environment
- AbacatePay: Dashboard → API Keys
- JWT_SECRET: Gerar novo com `openssl rand -hex 32`
- Admin password: Atualizar no Render

### Frequência
- **JWT_SECRET**: A cada 90 dias
- **API Keys**: A cada 180 dias
- **Senhas**: A cada 90 dias
- **Após incidente**: Imediatamente

## Limpeza do Histórico Git

### Por que é necessário
Arquivos como `CREDENTIALS.md` podem ter sido commitados acidentalmente e depois removidos. O histórico do Git mantém cópias antigas.

### Como limpar
```bash
# Instalar BFG Repo-Cleaner
# https://rtyley.github.io/bfg-repo-cleaner/

# Remover arquivos sensíveis
java -jar bfg.jar --delete-files CREDENTIALS.md
java -jar bfg.jar --delete-files _simple.env

# Forçar push
git push --force
```

### Após limpeza
1. Todos os devs devem re-clonar o repositório
2. Rotacionar todas as credenciais
3. Verificar se nenhum segredo ainda está exposto

## Logs e Auditoria

### O que é logado
- Requisições HTTP (método, path, status, latência)
- Erros de autenticação
- Operações financeiras (pagamento, split, chargeback)
- Ações administrativas

### O que NÃO é logado
- Senhas (nunca)
- Tokens JWT completos
- Dados de cartão de crédito
- CPF/CNPJ completos (usar máscara)

### Retenção
- Logs de aplicação: 30 dias
- Logs de auditoria: 1 ano
- Logs de erro: 90 dias

## Resposta a Incidentes

### Classificação
- **P1 (Crítico)**: Vazamento de dados, sistema indisponível
- **P2 (Alto)**: Vulnerabilidade explorável, degradação severa
- **Médio**: Bug com impacto limitado
- **Baixo**: Melhoria de segurança sem urgência

### Procedimento
1. **Conter**: Revogar credenciais comprometidas
2. **Avaliar**: Determinar escopo do impacto
3. **Corrigir**: Aplicar patch e deploy
4. **Comunicar**: Notificar usuários afetados (se P1/P2)
5. **Documentar**: Post-mortem e lições aprendidas

## Checklist de Segurança

### Antes de cada release
- [ ] `npm audit` sem vulnerabilidades críticas
- [ ] `govulncheck` sem vulnerabilidades críticas
- [ ] Testes de autenticação passando
- [ ] Rate limits testados
- [ ] RBAC verificado em todos os endpoints

### Mensalmente
- [ ] Revisar logs de auditoria
- [ ] Verificar dependências desatualizadas
- [ ] Testar rotação de credenciais
- [ ] Revisar acesso de usuários
