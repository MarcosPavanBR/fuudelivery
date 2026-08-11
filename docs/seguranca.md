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
