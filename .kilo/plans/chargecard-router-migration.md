# Plano: Migrar ChargeCard para PaymentRouter

## Contexto

Hoje `ChargeCard` e `ProcessPayment` instanciam `services.NewAbacatePayClient()` diretamente, ignorando o `PaymentRouter` (fallback chain + circuit breaker). Isso significa que se o AbacatePay cair, essas rotas não fazem fallback para Pagar.me/Asaas/MercadoPago.

## Objetivo

Migrar `ChargeCard` e `ProcessPayment` para usar `gateway.Router.CreateTransactionWithFallback()`.

## Passos

### 1. Inicializar gateways no setup do payment_api

**Arquivo:** `cmd/fuudelivery/main.go`

No setup do `payment_api`, criar as instâncias dos gateways e montar o router:

```go
pagarmeGW, _ := pagarme.NewGateway()
asaasGW, _ := asaas.NewGateway()
abacatepayGW, _ := abacatepay.NewGateway()
mpGW, _ := mercadopago.NewGateway()

paymentRouter := gateway.NewRouter(pagarmeGW, asaasGW, abacatepayGW, mpGW)
paymentRouter.SetStrategy(gateway.StrategyOrdered)
```

### 2. Disponibilizar router via contexto Fiber

**Arquivo:** `cmd/fuudelivery/main.go`

Criar middleware que injeta o router no contexto:

```go
func paymentRouterMiddleware(router *gateway.Router) fiber.Handler {
    return func(c *fiber.Ctx) error {
        c.Locals("payment_router", router)
        return c.Next()
    }
}
```

Aplicar apenas nas rotas de payment_api:

```go
app.Group("/payments", paymentRouterMiddleware(paymentRouter))
app.Group("/wallets", paymentRouterMiddleware(paymentRouter))
// ... etc
```

### 3. Modificar ChargeCard para usar router

**Arquivo:** `Backend/payment_api/app/handlers/card.go`

Alterar `ChargeCard` para:
1. Ler o router do contexto: `router := c.Locals("payment_router").(*gateway.Router)`
2. Construir `gateway.TransactionRequest` com `PaymentMethod: MethodCreditCard`
3. Chamar `router.CreateTransactionWithFallback(ctx, req)`
4. Mapear resposta normalizada

### 4. Modificar ProcessPayment para usar router

Mesmo arquivo. `ProcessPayment` também cria `services.NewAbacatePayClient()` diretamente. Mesma migração do item 3.

### 5. Atualizar validação de amount

Ambos handlers já chamam `validateChargeAmount(req.OrderID, req.Amount)`. Manter essa validação antes de montar o `TransactionRequest`.

### 6. Testes

- `Backend/payment_api/app/handlers/card_test.go`: adicionar teste mockando router e verificando que `CreateTransactionWithFallback` é chamado.
- Simular circuit breaker aberto no AbacatePay → verificar fallback para próximo gateway.

## Validação

```bash
cd Backend/payment_api && go test ./app/handlers/ -v -run TestChargeCard
cd Backend/payment_api && go test ./app/handlers/ -v -run TestProcessPayment
```

## Risco

- Mudança em fluxo de pagamento ativo. Requer testes E2E antes do deploy.
- Fallback para cartão pode mudar gateway sem aviso prévio ao frontend. O frontend deve tratar `gateway` na resposta.

## Ordem de Execução

1. `card.go` — `ChargeCard`
2. `card.go` — `ProcessPayment`
3. `pix.go` — `GeneratePIX` (se usar client direto)
