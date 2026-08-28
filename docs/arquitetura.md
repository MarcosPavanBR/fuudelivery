# Arquitetura do FuuDelivery


> ⚠️ **`Backend/Payment` foi arquivado e removido do repositório.** Todo o código
> de pagamento ativo vive em `payment_api` (embutido no monolito `cmd/fuudelivery`).
> As menções a `Backend/Payment` neste documento são **históricas** — não edite,
> não busque e não rode comandos apontando para esse diretório.
## Visão Geral

O FuuDelivery é uma plataforma de delivery colaborativa (cooperativa) que conecta restaurantes, clientes e entregadores com taxas significativamente menores que o modelo tradicional (5-12% vs 27-33% do iFood).

## Stack Tecnológica

### Backend (Go)
- **Monolito principal**: `cmd/fuudelivery/main.go` (361 handlers)
- **Módulos**: auth_api, orders_api, payment_api, delivery_api, chat_api, storage

### Infraestrutura
- **Banco de dados**: PostgreSQL (Supabase) — único banco relacional
- **Cache/Filas**: Redis (Redis Streams para comunicação entre serviços)
- **Deploy**: Render.com
- **Storage**: Supabase Storage (imagens)

### Frontend
- **WebAdmin**: React + Vite (painel administrativo)
- **WebRestaurant**: React + Vite (painel do restaurante)
- **AppComida**: React Native + Expo (cliente)
- **AppEntrega**: React Native + Expo (entregador)
- **AppRestaurante**: React Native + Expo (restaurante - em desenvolvimento)

## Fluxo de Dados

```
Cliente (App) → API Gateway → Pedidos → Pagamento (PIX via AbacatePay)
                                         ↓
                                    Webhook → Split Automático → Carteiras
                                         ↓
                                    Redis Streams → Restaurante (Notificação)
                                         ↓
                                    Aceite → Entregador (Matching)
                                         ↓
                                    Confirmação → Pagamento liberado
```

## Arquitetura de Microsserviços

O sistema usa um padrão híbrido:
- **Monolito**: Todos os domínios (auth, orders, payments, delivery, chat)
- **Comunicação interna**: Redis Streams (ordem dos eventos garantida)

## Decisões Arquiteturais

1. **Split automático**: Quando o pagamento é confirmado, o valor é dividido automaticamente entre restaurante e plataforma
2. **Engine de risco**: Analisa transações para detectar fraude antes de confirmar
3. **Matching de entregadores**: Sistema de calibration e zona para atribuição otimizada
4. **WebSocket**: Notificações em tempo real para todos os apps

## Segurança

- JWT com validação de algoritmo (prevenir algorithm confusion)
- HMAC em webhooks (comparação de tempo constante)
- bcrypt para senhas
- Rate limiting por IP (com TrustedProxies para IP real)
- RBAC: Controle de acesso por papel (admin, restaurante, entregador, cliente)
