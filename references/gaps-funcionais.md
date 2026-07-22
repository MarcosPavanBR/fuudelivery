# Gaps Funcionais — FuuDelivery

## TODOs pendentes no código

### 1. Pagamento — Ponte entre monólito e Payment Service

**Onde**: `cmd/fuudelivery` (monólito) → `Backend/Payment` (microsserviço)

**Problema**: O monólito recebe pedidos e pagamentos do AbacatePay, mas não há ponte automática para o Payment Service. Hoje:
- O monólito salva pagamentos no PostgreSQL
- O Payment Service tem seu próprio MongoDB
- Não há RabbitMQ real conectando os dois (o consumer no Payment Service tenta consumir, mas o producer no monólito não publica)

**Solução necessária**: Implementar producer no monólito que publique eventos de pagamento na fila, OU criar webhook do AbacatePay apontando para o Payment Service.

### 2. Página de cadastro de restaurante

**Onde**: README.md, seção "Próximos passos"

**Problema**: Não existe página de onboarding para restaurantes. Hoje, restaurantes são criados manualmente via API ou banco.

**Solução necessária**: Página de registro com formulário de dados do restaurante, upload de logo, configuração de horários.

### 3. Feature de relatórios

**Onde**: README.md, seção "Próximos passos"

**Problema**: Não há dashboard de relatórios (vendas por período, pedidos por restaurante, etc.).

**Solução necessária**: Relatórios de vendas, pedidos, entregadores, com exportação CSV/PDF.

---

## Duplicação: payment_api vs Backend/Payment

### O que existe

| Módulo | Localização | Banco | Escopo |
|---|---|---|---|
| `payment_api` | `Backend/payment_api/` | PostgreSQL (Supabase) | Processamento de pagamento (criar, webhook AbacatePay) |
| `Payment` | `Backend/Payment/` | MongoDB (Atlas) | Painel de aprovação, carteiras, score de risco, chargebacks |

### Por que existe

O `payment_api` é o módulo original do vercardapio que processa pagamentos via AbacatePay. O `Backend/Payment` foi criado depois como sistema separado para:
- Aprovação manual de pagamentos
- Score de risco
- Carteiras digitais
- Split de pagamento
- Painel administrativo

### O que fazer

**Opção A (recomendada)**: Documentar claramente a separação:
- `payment_api`: Processa pagamento (gateway AbacasePay)
- `Payment`: Gerencia aprovação, carteira, split, risco

**Opção B (futuro)**: Consolidar em um único serviço, mas requer refatoração significativa.

---

## README desatualizado

O README.md atual reflete o projeto original (vercardapio), não o fork (FuuDelivery). Informações que precisam ser atualizadas:

- [ ] Nome do projeto (vercardapio → FuuDelivery)
- [ ] Features novas (pagamento, carteira, chat, rastreio)
- [ ] Arquitetura (5 serviços no Render)
- [ ] Variáveis de ambiente necessárias
- [ ] Guia de setup local atualizado
- [ ] Licença (verificar se mantém MIT)
