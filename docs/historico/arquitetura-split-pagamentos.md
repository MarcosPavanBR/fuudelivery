# Arquitetura Multi-Gateway de Pagamentos — FuuDelivery

> **Versão**: 3.0 — Documento definitivo de engenharia
> **Data**: Agosto 2026
> **Status**: Plano de implementação
> **Métodos de pagamento suportados**: PIX · Cartão de Crédito · Cartão de Débito
> **Gateways**: AbacatePay (PIX fallback) · Pagar.me (principal) · Asaas (alternativo) · Mercado Pago (reserva)

---

## Sumário Executivo

O FuuDelivery precisa de uma arquitetura de pagamentos que suporte **split automático** entre plataforma, restaurantes e entregadores, **pré-autorização** para cartão, **escrow** (custódia) com repasse D+X, e **múltiplos gateways** para resiliência. Este documento define a camada de abstração unificada que permite trocar gateways sem alterar o código de negócio, com foco em segurança, idempotência, observabilidade e recuperação de falhas.

**Métodos de pagamento**: PIX (instantâneo) · Cartão de Crédito (parcelável, com 3DS) · Cartão de Débito (débito online, com 3DS)

**Não suportado**: Boleto bancário (fora do escopo do FuuDelivery).

---

## 1. Diagnóstico do Estado Atual

### 1.1 Componentes existentes no monolito

| Componente | Arquivo no monolito | Status |
|------------|---------------------|--------|
| Criação de cobrança PIX (AbacatePay) | `payment_api/pix.go` | ✅ Funcional |
| Criação de cobrança cartão (AbacatePay) | `payment_api/card.go` | ✅ Funcional |
| Webhook de confirmação (verificação via API) | `payment_api/webhook.go` | ✅ Funcional |
| Split de cálculo interno (75/15/10) | `payment_api/split.go` | ⚠️ Calcula, não envia ao gateway |
| Carteira digital (wallet ledger atômico) | `payment_api/wallet.go` | ✅ Funcional |
| Idempotência financeira (constraints UNIQUE) | `sql/11_idempotencia_financeira.sql` | ✅ Funcional |
| Fila Redis (Streams + DLQ + retry) | `pkg/queue/` | ✅ Funcional |
| WebSocket notificações | `cmd/fuudelivery/main.go` | ✅ `payment_updates` |
| Score de risco (0-100) | `payment_api` | ⚠️ Calcula, sem ação automática |

### 1.2 Gaps que bloqueiam produção

| Gap | Impacto | Prioridade | Esforço |
|-----|---------|:----------:|:-------:|
| Split real no gateway não existe | Dinheiro cai 100% na conta da plataforma. Repasse manual inviável. | 🔴 P0 | 5-6 dias |
| Sem sub-contas de recebedores | Restaurantes/entregadores não têm conta no gateway | 🔴 P0 | 3-4 dias |
| Sem pré-autorização (cartão) | Não há como "reservar" e capturar depois da entrega | 🔴 P0 | 4-5 dias |
| Sem escrow / D+X | Dinheiro cai imediatamente, sem trava de repasse | 🟡 P1 | 3-4 dias |
| Sem PIN de verificação | Motoboy pode confirmar entrega falsa → split indevido | 🟡 P1 | 2-3 dias |
| Sem 3DS (3D Secure) | Cartão sem autenticação → chargeback sem proteção | 🟡 P1 | 2-3 dias |
| Webhook sem HMAC | Webhook não valida assinatura → possível spoofing | 🟠 P2 | 1 dia |
| Rate limit ausente em /payments/* | Endpoints de dinheiro sem proteção contra abuso | 🟠 P2 | 1 dia |

### 1.3 Por que o AbacatePay sozinho não resolve

A página oficial de marketplaces da AbacatePay declara:

> **"Split automático entre plataforma e vendedores está em desenvolvimento. Em breve."**

A AbacatePay **não suporta**: split, sub-contas, pré-autorização, escrow, cartão de crédito/débito, 3DS. É exclusivamente PIX.

---

## 2. Matriz de Gateways — Análise Comparativa

### 2.1 Capacidades Funcionais

| Capacidade | AbacatePay | Pagar.me (v4) | Asaas | Mercado Pago |
|------------|:----------:|:-------------:|:-----:|:------------:|
| **PIX** | ✅ Nativo | ✅ Nativo | ✅ Nativo | ✅ Nativo |
| **Cartão de Crédito** | ❌ | ✅ | ✅ | ✅ |
| **Cartão de Débito** | ❌ | ✅ (`debit_card`) | ✅ | ✅ (Elo, Visa Electron) |
| **3D Secure (3DS)** | ❌ | ✅ (obrigatório) | ✅ (opcional) | ✅ |
| **Split de pagamento** | ❌ Em dev | ✅ Nativo | ✅ Nativo | ⚠️ 1:1 apenas |
| **Sub-contas (recipients)** | ❌ | ✅ Recipients | ✅ Wallets | ⚠️ OAuth por vendedor |
| **Pré-autorização (Auth)** | ❌ | ✅ Cartão | ✅ Cartão | ⚠️ Limitado |
| **Captura retardada (Capture)** | ❌ | ✅ | ✅ | ⚠️ |
| **Cancelamento (Void)** | ❌ | ✅ | ✅ | ⚠️ |
| **Split percentual** | — | ✅ | ✅ | ⚠️ marketplace_fee |
| **Split fixo (R$)** | — | ✅ | ✅ | ❌ |
| **Split percentual + fixo** | — | ✅ | ✅ | ❌ |
| **MDR configurável por recipient** | — | ✅ | ❌ | ❌ |
| **Responsabilidade por chargeback** | — | ✅ Por recipient | ❌ | ⚠️ Proporcional |
| **Escrow / D+X** | ❌ | ✅ Configurável | ✅ Nativo | ⚠️ Parcial |
| **Antecipação de saldo** | ❌ | ✅ | ✅ | ✅ |
| **Webhook padronizado** | ✅ | ✅ | ✅ | ✅ |
| **Ambiente sandbox** | ✅ | ✅ | ✅ | ✅ |

### 2.2 Comparativo Financeiro (taxas)

| Gateway | PIX | Crédito 1x | Crédito 3x | Crédito 12x | Débito | Split |
|---------|-----|------------|------------|-------------|--------|-------|
| **AbacatePay** | R$ 0,99 | — | — | — | — | — |
| **Pagar.me** | R$ 0,39 | 1,99%+R$0,39 | 2,87% | 3,99% | 0,99%+R$0,39 | Incluso |
| **Asaas** | R$ 0,99 | 1,99% | 2,99% | 3,99% | 1,99% | Incluso |
| **Mercado Pago** | R$ 0,39 | 3,99%+R$0,49 | 5,99% | 7,99% | 3,99% | Incluso |

### 2.3 Onboarding de Recebedores

| Gateway | Tempo estimado | KYC | Conta bancária | Via API |
|---------|:-------------:|:---:|:--------------:|:-------:|
| **Pagar.me** | ~5 min | Sim (documentos) | Sim | ✅ |
| **Asaas** | ~10 min | Sim (simplificado) | Sim | ✅ |
| **Mercado Pago** | ~30 min | Sim (completo) | Sim | ❌ OAuth manual |

### 2.4 Papel de cada Gateway no FuuDelivery

| Gateway | Papel | Motivo |
|---------|-------|--------|
| **Pagar.me** | 🔵 **PRINCIPAL** | Melhor custo-benefício, split nativo, MDR configurável, 3DS, débito |
| **Asaas** | 🟢 **ALTERNATIVO** | Split maduro, D+X nativo, cartão de débito |
| **AbacatePay** | 🟡 **FALLBACK PIX** | Já integrado, PIX simples sem split |
| **Mercado Pago** | ⚪ **RESERVA** | Split 1:1 limitado, taxas altas, onboarding complexo |

---

## 3. Arquitetura: Camada de Abstração Unificada

### 3.1 Princípios de Design

1. **Strategy Pattern**: cada gateway é uma implementação da interface `Gateway`
2. **Zero acoplamento**: o código de negócio nunca importa um gateway específico
3. **Feature flags**: habilitar/desabilitar gateways sem redeploy
4. **Webhook normalizado**: todos os gateways geram o mesmo `WebhookEvent`
5. **Fallback automático**: se o gateway primário falhar, tenta o alternativo
6. **Idempotência em duas camadas**: código (try-then-constraint) + banco (UNIQUE)
7. **Circuit breaker**: se o gateway falhar 5x em 1 minuto, desativa temporariamente
8. **Observabilidade**: métricas Prometheus, tracing estruturado, alertas

### 3.2 Diagrama de Camadas

```
┌──────────────────────────────────────────────────────────┐
│                      API Handlers                         │
│  POST /checkout   POST /webhook/:gateway   POST /refund  │
│  POST /capture    POST /authorize   POST /void           │
│  POST /delivery/confirm-with-pin                         │
│  GET  /wallet/balance   POST /wallet/withdraw            │
│  POST /recipients   GET  /recipients/:id                 │
└─────────────────────────┬────────────────────────────────┘
                          │
┌─────────────────────────▼────────────────────────────────┐
│                 PaymentService (domínio)                   │
│  Lógica de negócio: validar, orquestrar, publicar evento  │
│  Estado do pedido: máquina de estados                     │
│  Split rules: calcular % por recipient                    │
│  PIN: gerar, validar, expirar                             │
│  Escrow: controlar D+X                                    │
└─────────────────────────┬────────────────────────────────┘
                          │
┌─────────────────────────▼────────────────────────────────┐
│                  PaymentRouter                             │
│  Seleciona gateway por: método + split + flags             │
│  Circuit breaker por gateway                              │
│  Retry com backoff exponencial                            │
└──┬────────────┬────────────┬────────────┬────────────────┘
   │            │            │            │
┌──▼────┐  ┌───▼────┐  ┌───▼────┐  ┌───▼────┐
│Pagar  │  │ Asaas  │  │Abacate │  │  MP    │
│.me    │  │        │  │Pay     │  │        │
│PRINC. │  │ALTER.  │  │FALLBK  │  │RESERVA │
└───────┘  └────────┘  └────────┘  └────────┘
```

### 3.3 Interface Go Unificada — Definição Completa

```go
// pkg/gateway/gateway.go
//
// Camada de abstração unificada para gateways de pagamento.
// Cada provider (Pagar.me, Asaas, AbacatePay, Mercado Pago)
// implementa esta interface. O código de negócio usa apenas
// esta interface — nunca importa um gateway específico.

package gateway

import (
    "context"
    "errors"
    "time"
)

// ═══════════════════════════════════════════════════════════
// ENUMS
// ═══════════════════════════════════════════════════════════

// PaymentMethod representa o método de pagamento aceito.
type PaymentMethod string

const (
    MethodPIX        PaymentMethod = "pix"
    MethodCreditCard PaymentMethod = "credit_card"
    MethodDebitCard  PaymentMethod = "debit_card"
)

// TransactionStatus representa o ciclo de vida de uma transação.
type TransactionStatus string

const (
    StatusPending      TransactionStatus = "pending"
    StatusAuthorized   TransactionStatus = "authorized"   // Cartão: pré-autorizado
    StatusWaiting      TransactionStatus = "waiting"      // PIX: aguardando pagamento
    StatusPaid         TransactionStatus = "paid"         // Confirmado pelo gateway
    StatusCaptured     TransactionStatus = "captured"     // Cartão: capturado (descontado)
    StatusRefunded     TransactionStatus = "refunded"     // Estornado
    StatusVoided       TransactionStatus = "voided"       // Cancelado (pré-autorização)
    StatusFailed       TransactionStatus = "failed"       // Recusado
    StatusExpired      TransactionStatus = "expired"      // PIX expirado
    StatusChargeback   TransactionStatus = "chargeback"   // Contestado
)

// SplitStatus representa o estado de uma regra de split.
type SplitStatus string

const (
    SplitPending  SplitStatus = "pending"
    SplitPaid     SplitStatus = "paid"
    SplitFailed   SplitStatus = "failed"
    SplitRefunded SplitStatus = "refunded"
    SplitBlocked  SplitStatus = "blocked"   // Divergência de valores
)

// ═══════════════════════════════════════════════════════════
// TIPOS DE ENTRADA (REQUEST)
// ═══════════════════════════════════════════════════════════

// SplitRule define como o valor de um pagamento é dividido.
type SplitRule struct {
    RecipientID   string  // ID do recebedor no gateway (walletId, recipient_id)
    Percentage    float64 // Percentual sobre netValue (0-100). Mutuamente exclusivo com FixedValue.
    FixedValue    int64   // Valor fixo em centavos. Se 0, usa Percentage.
    Liable        bool    // Responsável pelo MDR (taxa de interchange do cartão)
    ChargebackResponsible bool // Responsável por chargeback
}

// TransactionRequest é o pedido unificado de criação de transação.
// O PaymentRouter traduz isso para o formato específico de cada gateway.
type TransactionRequest struct {
    // Identificação
    OrderID       int64           // ID interno do pedido
    IdempotencyKey string         // Chave de idempotência (UUID v4)

    // Valores
    Amount        int64           // Valor total em centavos (ex: R$ 50,00 = 5000)
    Currency      string          // "BRL"

    // Método de pagamento
    PaymentMethod PaymentMethod   // pix, credit_card, debit_card

    // Dados do cliente
    CustomerEmail string
    CustomerName  string
    CustomerDoc   string          // CPF ou CNPJ (somente dígitos)
    CustomerPhone string          // +5511999999999

    // Dados do cartão (quando method = credit_card ou debit_card)
    *CardData

    // Split
    SplitRules    []SplitRule     // Vazio = sem split (pagamento entra na conta principal)

    // Pré-autorização (apenas credit_card)
    Capture       bool            // false = auth only; true = auth + capture (default)
    CaptureDelay  int             // Minutos até auto-capture (0 = manual)

    // Metadados
    Description   string
    Metadata      map[string]string // Ex: {"order_id": "123", "customer_phone": "+55..."}
}

// CardData contém os dados tokenizados do cartão.
// Em produção, o frontend envia o token (nunca o número do cartão).
type CardData struct {
    Token           string // Token do cartão (gerado pelo SDK do gateway no frontend)
    Installments    int    // Número de parcelas (1 = à vista)
    HolderName      string // Nome no cartão
    HolderDoc       string // CPF do titular
    BillingZipCode  string // CEP de cobrança
}

// ═══════════════════════════════════════════════════════════
// TIPOS DE SAÍDA (RESPONSE)
// ═══════════════════════════════════════════════════════════

// TransactionResponse é a resposta normalizada de qualquer gateway.
type TransactionResponse struct {
    GatewayID       string            // ID da transação no gateway externo
    Gateway         string            // Nome do gateway ("pagarme", "asaas", etc.)
    Status          TransactionStatus // Estado atual da transação

    // PIX
    PIXQRCode       string            // Imagem do QR Code (base64 ou URL)
    PIXCopyPaste    string            // Código copia-e-cola (payload PIX)
    PIXExpiresAt    *time.Time        // Data de expiração do QR Code

    // Cartão
    CardBrand       string            // Visa, Mastercard, Elo, Amex
    CardLast4       string            // Últimos 4 dígitos
    RequiresAuth    bool              // 3DS necessário (redirecionar cliente)
    AuthURL         string            // URL de autenticação 3DS

    // Split
    SplitApplied    bool              // Se split foi aplicado
    SplitCount      int               // Número de recipients no split

    // Metadados
    Metadata        map[string]string
}

// RefundResponse é a resposta de um estorno.
type RefundResponse struct {
    RefundID    string  // ID do estorno no gateway
    Gateway     string
    Amount      int64   // Valor estornado em centavos
    Status      string  // pending, processing, completed
    EstimatedAt *time.Time // Previsão de crédito
}

// ═══════════════════════════════════════════════════════════
// TIPOS DE WEBHOOK
// ═══════════════════════════════════════════════════════════

// WebhookEvent é o evento normalizado de qualquer gateway.
// Cada adapter traduz o payload nativo do gateway para esta estrutura.
type WebhookEvent struct {
    Gateway         string
    EventType       string            // paid, failed, refunded, split_done, split_block
    TransactionID   string            // ID no gateway
    OrderID         string            // Extraído do metadata
    Amount          int64             // Valor em centavos
    Status          TransactionStatus
    SplitDetails    []SplitDetail     // Detalhes de cada split processado
    PaymentMethod   PaymentMethod
    CardBrand       string
    CardLast4       string
    RawPayload      []byte            // Payload original (para auditoria)
    ReceivedAt      time.Time
}

// SplitDetail representa o resultado de um split para um recipient.
type SplitDetail struct {
    RecipientID     string
    Amount          int64       // Valor em centavos
    Percentage      float64
    Status          SplitStatus
    FailureReason   string      // Se status = failed
}

// ═══════════════════════════════════════════════════════════
// TIPOS DE RECEBEDOR
// ═══════════════════════════════════════════════════════════

// RecipientRequest é o pedido unificado de criação de recebedor.
type RecipientRequest struct {
    UserType        string  // "restaurant" | "delivery_man"
    UserID          int64   // ID interno
    Name            string
    Document        string  // CPF ou CNPJ
    Email           string
    Phone           string
    BankCode        string  // Código do banco (ex: "341" = Itaú)
    BankAgency      string  // Agência
    BankAccount     string  // Conta
    BankAccountDV   string  // Dígito verificador
    BankAccountType string  // "conta_corrente" | "conta_poupanca"
    TransferInterval string // "daily" | "weekly" | "monthly"
    TransferDay     int     // Dia (1-28 para monthly, 1-7 para weekly)
}

// RecipientResponse é a resposta de criação de recebedor.
type RecipientResponse struct {
    RecipientID     string  // ID no gateway
    Gateway         string
    Status          string  // pending, active, blocked
    KYCStatus       string  // pending, approved, rejected
    Balance         int64   // Saldo disponível em centavos
    PendingBalance  int64   // Saldo pendente (em custódia)
    CreatedAt       time.Time
}

// ═══════════════════════════════════════════════════════════
// INTERFACE PRINCIPAL
// ═══════════════════════════════════════════════════════════

// Gateway é a interface que cada provider implementa.
// Todas as operações aceitam context.Context para timeout/cancel.
// Erros seguem o padrão Go: errors.New, errors.Is, errors.As.
type Gateway interface {
    // Name retorna o identificador único do gateway ("pagarme", "asaas", etc.)
    Name() string

    // ─── Transações ─────────────────────────────────────────

    // CreateTransaction cria uma transação no gateway.
    // Para PIX: retorna QR Code e código copia-e-cola.
    // Para cartão com Capture=true: autoriza e captura imediatamente.
    // Para cartão com Capture=false: apenas autoriza (pré-autorização).
    // Para split: inclui split_rules na transação.
    // Retorno: TransactionResponse com status e dados para exibição.
    CreateTransaction(ctx context.Context, req *TransactionRequest) (*TransactionResponse, error)

    // CaptureTransaction captura uma pré-autorização (cartão apenas).
    // Chamado quando o motoboy confirma a entrega com PIN.
    // amount: valor a capturar (pode ser menor que o autorizado, ex: estorno parcial).
    CaptureTransaction(ctx context.Context, gatewayID string, amount int64) error

    // RefundTransaction estorna uma transação paga.
    // amount: valor a estornar em centavos (0 = estorno total).
    // Retorno: RefundResponse com ID do estorno e previsão.
    RefundTransaction(ctx context.Context, gatewayID string, amount int64) (*RefundResponse, error)

    // VoidTransaction cancela uma pré-autorização (cartão apenas).
    // Diferente de Refund: não há cobrança, apenas libera o bloqueio no cartão.
    VoidTransaction(ctx context.Context, gatewayID string) error

    // GetTransactionStatus consulta o status atual de uma transação.
    // Usado como fallback quando o webhook não chega.
    GetTransactionStatus(ctx context.Context, gatewayID string) (TransactionStatus, error)

    // ─── Recebedores ────────────────────────────────────────

    // CreateRecipient cria uma sub-conta no gateway para recebimento de splits.
    // O recebedor precisa ter dados bancários para receber transferências.
    CreateRecipient(ctx context.Context, req *RecipientRequest) (*RecipientResponse, error)

    // UpdateRecipient atualiza dados bancários ou configurações de transferência.
    UpdateRecipient(ctx context.Context, recipientID string, req *RecipientRequest) error

    // GetRecipientBalance retorna o saldo disponível e pendente de um recebedor.
    GetRecipientBalance(ctx context.Context, recipientID string) (available int64, pending int64, err error)

    // ─── Webhook ────────────────────────────────────────────

    // ValidateWebhook valida a assinatura do webhook (HMAC, token, etc.)
    // Retorna true se a assinatura for válida. Retorna false caso contrário.
    ValidateWebhook(body []byte, headers map[string]string) bool

    // ParseWebhook converte o payload nativo do gateway em um WebhookEvent normalizado.
    // Se o payload não for reconhecido, retorna erro (não panic).
    ParseWebhook(body []byte) (*WebhookEvent, error)

    // ─── Capacidades ────────────────────────────────────────

    // SupportsMethod retorna true se o gateway suporta o método de pagamento.
    SupportsMethod(method PaymentMethod) bool

    // SupportsSplit retorna true se o gateway suporta split nativo.
    SupportsSplit() bool

    // SupportsPreAuth retorna true se o gateway suporta pré-autorização.
    SupportsPreAuth() bool

    // Supports3DS retorna true se o gateway suporta 3D Secure.
    Supports3DS() bool

    // SupportsEscrow retorna true se o gateway suporta escrow/D+X.
    SupportsEscrow() bool

    // MaxSplitRecipients retorna o máximo de recipients por transação (0 = ilimitado).
    MaxSplitRecipients() int
}
```

### 3.4 Estrutura de Pacotes

```
pkg/gateway/
├── gateway.go              # Interface + tipos + enums
├── router.go               # PaymentRouter (seleção + circuit breaker + retry)
├── registry.go             # Registry de gateways registrados
├── circuitbreaker.go       # Circuit breaker por gateway
├── events.go               # Evento normalizado + publisher Redis
│
├── pagarme/
│   ├── client.go           # HTTP client Pagar.me v4 (com retry + timeout)
│   ├── gateway.go          # Implementa Gateway para Pagar.me
│   ├── types.go            # Request/Response nativos do Pagar.me
│   ├── webhook.go          # HMAC SHA255 validation + parsing
│   └── gateway_test.go     # Testes com mock HTTP
│
├── asaas/
│   ├── client.go           # HTTP client Asaas API
│   ├── gateway.go          # Implementa Gateway para Asaas
│   ├── types.go            # Request/Response nativos do Asaas
│   ├── webhook.go          # Token validation + parsing
│   └── gateway_test.go     # Testes com mock HTTP
│
├── abacatepay/
│   ├── client.go           # HTTP client AbacatePay (adaptar do existente)
│   ├── gateway.go          # Implementa Gateway (PIX only, sem split)
│   ├── types.go            # Request/Response nativos
│   ├── webhook.go          # HMAC validation (já existe parcialmente)
│   └── gateway_test.go     # Testes com mock HTTP
│
└── mercadopago/
    ├── client.go           # HTTP client Mercado Pago
    ├── gateway.go          # Implementa Gateway (split 1:1)
    ├── types.go            # Request/Response nativos
    ├── webhook.go          # HMAC SHA256 validation + parsing
    └── gateway_test.go     # Testes com mock HTTP
```

---

## 4. Máquina de Estados do Pedido

### 4.1 Estados do Pedido

```
                    ┌──────────────┐
                    │   CRIADO     │
                    │  (created)   │
                    └──────┬───────┘
                           │
              ┌────────────▼────────────┐
              │    PAGAMENTO PENDENTE    │
              │  (awaiting_payment)      │
              │  PIX: aguardando QR     │
              │  Cartão: 3DS redirect   │
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │     PAGAMENTO PAGO       │
              │  (paid)                  │
              │  PIX: confirmado         │
              │  Cartão: autorizado      │
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │     EM PREPARO           │
              │  (preparing)             │
              │  Restaurante preparando  │
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │    AGUARDANDO ENTREGADOR │
              │  (waiting_delivery)      │
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │    EM ENTREGA            │
              │  (in_delivery)           │
              │  Motoboy a caminho      │
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │    ENTREGUE              │
              │  (delivered)             │
              │  PIN verificado ✅        │
              │  Cartão: capturado       │
              │  Split executado         │
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │    CONCLUÍDO             │
              │  (completed)             │
              │  Saldo em D+X           │
              └──────────────────────────┘
```

### 4.2 Estados de Pagamento

```
   PIX                           Cartão de Crédito/Débito
   ───                           ────────────────────────

   created ──► waiting ──► paid    created ──► authorized ──► captured
                   │                    │              │
                   ▼                    ▼              ▼
               expired              failed          refunded
                                   │
                                   ▼
                               chargeback
```

### 4.3 Transições Permitidas

| Estado Origem | Evento | Estado Destino | Ação no Gateway |
|:-------------:|:------:|:--------------:|:----------------|
| created | Checkout iniciado | awaiting_payment (PIX) / authorized (cartão) | CreateTransaction |
| awaiting_payment | Webhook paid | paid | Nenhuma (gateway já processou) |
| awaiting_payment | Expiração (15min) | expired | Void/Cancel automático |
| authorized | PIN confirmado | captured | CaptureTransaction |
| authorized | Admin cancela | voided | VoidTransaction |
| authorized | Timeout (30min sem PIN) | expired | VoidTransaction automático |
| paid/captured | Admin estorna | refunded | RefundTransaction |
| paid/captured | Cliente contesta | chargeback | Notificação admin |
| any | Gateway falha | failed | Log + retry |

---

## 5. Fluxos Detalhados por Método de Pagamento

### 5.1 PIX via Pagar.me (com split)

```
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│ Frontend │    │Monolito  │    │ Pagar.me │    │ Gateway  │
│ (Cliente)│    │ (Go)     │    │   API    │    │ Webhook  │
└────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘
     │               │               │               │
     │ POST /checkout│               │               │
     │ {order_id,    │               │               │
     │  method:"pix",│               │               │
     │  amount:5000, │               │               │
     │  split:[{     │               │               │
     │    recipient: │               │               │
     │    "rest_01", │               │               │
     │    pct: 75},  │               │               │
     │   {recipient: │               │               │
     │    "driver_01"│               │               │
     │    pct: 15}]} │               │               │
     │──────────────►│               │               │
     │               │               │               │
     │               │ CreateTransaction              │
     │               │ (split_rules, pix)│            │
     │               │──────────────►│               │
     │               │               │               │
     │               │◄──────────────│               │
     │               │ {pix_qr_code, │               │
     │               │  pix_copy_paste│              │
     │               │  gateway_id}  │               │
     │               │               │               │
     │◄──────────────│               │               │
     │ {qr_code,     │               │               │
     │  copy_paste,  │               │               │
     │  expires_at}  │               │               │
     │               │               │               │
     │  [Cliente paga PIX]           │               │
     │               │               │               │
     │               │               │  transaction.paid
     │               │               │──────────────►
     │               │               │               │
     │               │ ValidateWebhook               │
     │               │ (HMAC SHA256) │               │
     │               │◄──────────────│               │
     │               │               │               │
     │               │ ParseWebhook  │               │
     │               │ {status:"paid",│              │
     │               │  split:[{     │               │
     │               │   rest: R$37, │               │
     │               │   driver: R$7,│               │
     │               │   platform:   │               │
     │               │   R$5}]}      │               │
     │               │               │               │
     │               │ Atualizar:    │               │
     │               │ payment.paid  │               │
     │               │ split_rules   │               │
     │               │ →paid cada    │               │
     │               │               │               │
     │               │ Publish Redis │               │
     │               │ payment_updates               │
     │               │ order_updates │               │
     │               │               │               │
     │  WebSocket: "pagamento confirmado"            │
     │◄──────────────│               │               │
     │               │               │               │
```

### 5.2 Cartão de Crédito via Pagar.me (com 3DS + split)

```
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│ Frontend │    │Monolito  │    │ Pagar.me │    │ Frontend │
│ (Cliente)│    │ (Go)     │    │   API    │    │  3DS     │
└────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘
     │               │               │               │
     │ POST /checkout│               │               │
     │ {method:      │               │               │
     │  "credit_card",│              │               │
     │  card_token,  │               │               │
     │  installments:│               │               │
     │  3,           │               │               │
     │  capture:     │               │               │
     │  false,       │               │               │
     │  split:[...]} │               │               │
     │──────────────►│               │               │
     │               │               │               │
     │               │ CreateTransaction              │
     │               │ {capture:false,│               │
     │               │  split_rules} │               │
     │               │──────────────►│               │
     │               │               │               │
     │               │ Requires 3DS? │               │
     │               │◄──────────────│               │
     │               │ {requires_auth│               │
     │               │  :true,       │               │
     │               │  auth_url}    │               │
     │               │               │               │
     │  Redireciona para 3DS         │               │
     │◄──────────────│               │               │
     │──────────────────────────────►│               │
     │               │               │  Autenticação │
     │               │               │  3DS (senha/  │
     │               │               │  biom. do     │
     │               │               │  banco)       │
     │◄──────────────────────────────│               │
     │               │               │               │
     │               │ 3DS aprovado  │               │
     │               │ → autoriza    │               │
     │               │               │               │
     │               │ transaction.authorized         │
     │               │◄──────────────│               │
     │               │               │               │
     │               │ Atualizar:    │               │
     │               │ payment.auth  │               │
     │               │               │               │
     │  [Pedido em preparo...]       │               │
     │  [Motoboy a caminho...]       │               │
     │               │               │               │
     │ POST /delivery/confirm-with-pin               │
     │ {pin: "8472"}│               │               │
     │──────────────►│               │               │
     │               │               │               │
     │               │ Validar PIN:  │               │
     │               │ hash(8472)==  │               │
     │               │ stored_hash?  │               │
     │               │ TTL<30min?    │               │
     │               │ attempts<3?   │               │
     │               │               │               │
     │               │ CaptureTransaction             │
     │               │ (gateway_id,  │               │
     │               │  amount=5000) │               │
     │               │──────────────►│               │
     │               │               │               │
     │               │ Split processado:              │
     │               │ rest: R$37.50 │               │
     │               │ driver: R$7.50│               │
     │               │ platform: R$5 │               │
     │               │               │               │
     │               │ transaction.captured           │
     │               │◄──────────────│               │
     │               │               │               │
     │  WebSocket: "entrega confirmada, pagamento    │
     │              processado"      │               │
     │◄──────────────│               │               │
```

### 5.3 Cartão de Débito via Pagar.me

```
┌──────────┐    ┌──────────┐    ┌──────────┐
│ Frontend │    │Monolito  │    │ Pagar.me │
│ (Cliente)│    │ (Go)     │    │   API    │
└────┬─────┘    └────┬─────┘    └────┬─────┘
     │               │               │
     │ POST /checkout│               │
     │ {method:      │               │
     │  "debit_card",│               │
     │  card_token,  │               │
     │  capture:true,│               │
     │  split:[...]} │               │
     │──────────────►│               │
     │               │               │
     │               │ CreateTransaction              │
     │               │ {payment_method:│              │
     │               │  "debit_card", │               │
     │               │  capture:true} │               │
     │               │──────────────►│               │
     │               │               │
     │               │ 3DS obrigatório│               │
     │               │ para débito   │               │
     │               │◄──────────────│               │
     │               │ {requires_auth│               │
     │               │  :true,       │               │
     │               │  auth_url}    │               │
     │               │               │               │
     │  Redireciona para 3DS         │               │
     │◄──────────────│               │               │
     │──────────────────────────────►│               │
     │               │               │  Autenticação │
     │◄──────────────────────────────│               │
     │               │               │               │
     │               │ 3DS aprovado  │               │
     │               │ → autoriza +  │               │
     │               │   captura     │               │
     │               │               │               │
     │               │ transaction.paid               │
     │               │ (débito = imediato)            │
     │               │◄──────────────│               │
     │               │               │               │
     │               │ Split processado               │
     │               │ automaticamente│               │
     │               │               │               │
```

### 5.4 PIX via AbacatePay (fallback sem split)

```
┌──────────┐    ┌──────────┐    ┌──────────┐
│ Frontend │    │Monolito  │    │ AbacatePay│
│ (Cliente)│    │ (Go)     │    │   API    │
└────┬─────┘    └────┬─────┘    └────┬─────┘
     │               │               │
     │ POST /checkout│               │
     │ {method:"pix"}│               │
     │──────────────►│               │
     │               │               │
     │               │ CreateTransaction              │
     │               │ (sem split_rules)│             │
     │               │──────────────►│               │
     │               │               │
     │               │ {pix_qr_code, │               │
     │               │  pix_copy_paste│              │
     │               │  gateway_id}  │               │
     │               │◄──────────────│               │
     │               │               │
     │◄──────────────│               │
     │ {qr_code}     │               │
     │               │               │
     │  [Cliente paga PIX]           │
     │               │               │
     │               │  Webhook: pagou│               │
     │               │◄──────────────│               │
     │               │               │
     │               │ Verifica via API               │
     │               │ (re-verificação)│              │
     │               │──────────────►│               │
     │               │               │
     │               │ Split CALCULADO                │
     │               │ internamente:  │               │
     │               │ rest: 75%      │               │
     │               │ driver: 15%    │               │
     │               │ platform: 10%  │               │
     │               │               │
     │               │ ⚠️ REPASSE    │
     │               │ MANUAL         │
     │               │ (AbacatePay não│
     │               │ tem split)     │
```

---

## 6. Estratégia de Roteamento

### 6.1 Router com Circuit Breaker

```go
// pkg/gateway/router.go

package gateway

import (
    "context"
    "errors"
    "sync"
    "time"
)

var (
    ErrNoGatewayAvailable = errors.New("no gateway available for this method")
    ErrCircuitOpen        = errors.New("circuit breaker open")
)

// RouterSelection Strategy:
// 1. Verifica circuit breaker de cada gateway
// 2. Seleciona gateway que suporta método + split
// 3. Se falhar, tenta o próximo (fallback chain)

type Router struct {
    mu       sync.RWMutex
    gateways []gatewayEntry
}

type gatewayEntry struct {
    gateway Gateway
    cb      *CircuitBreaker
}

func NewRouter(gateways ...Gateway) *Router {
    r := &Router{}
    for _, g := range gateways {
        r.gateways = append(r.gateways, gatewayEntry{
            gateway: g,
            cb:      NewCircuitBreaker(5, 1*time.Minute), // 5 falhas = open por 1min
        })
    }
    return r
}

// Select retorna o melhor gateway disponível para a requisição.
func (r *Router) Select(method PaymentMethod, requiresSplit bool) (Gateway, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    for _, entry := range r.gateways {
        // Pula se circuit breaker está aberto
        if entry.cb.IsOpen() {
            continue
        }

        // Pula se não suporta o método
        if !entry.gateway.SupportsMethod(method) {
            continue
        }

        // Pula se requer split mas gateway não suporta
        if requiresSplit && !entry.gateway.SupportsSplit() {
            continue
        }

        return entry.gateway, nil
    }

    return nil, ErrNoGatewayAvailable
}

// RecordSuccess registra sucesso (fecha circuit breaker)
func (r *Router) RecordSuccess(name string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    for i := range r.gateways {
        if r.gateways[i].gateway.Name() == name {
            r.gateways[i].cb.RecordSuccess()
            return
        }
    }
}

// RecordFailure registra falha (incrementa contador do circuit breaker)
func (r *Router) RecordFailure(name string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    for i := range r.gateways {
        if r.gateways[i].gateway.Name() == name {
            r.gateways[i].cb.RecordFailure()
            return
        }
    }
}

// CreateTransactionWithFallback tenta criar transação com fallback automático.
func (r *Router) CreateTransactionWithFallback(
    ctx context.Context,
    req *TransactionRequest,
) (*TransactionResponse, error) {
    requiresSplit := len(req.SplitRules) > 0

    // Tenta cada gateway na ordem
    var lastErr error
    for _, entry := range r.gateways {
        if entry.cb.IsOpen() {
            continue
        }
        if !entry.gateway.SupportsMethod(req.PaymentMethod) {
            continue
        }
        if requiresSplit && !entry.gateway.SupportsSplit() {
            continue
        }

        resp, err := entry.gateway.CreateTransaction(ctx, req)
        if err != nil {
            entry.cb.RecordFailure()
            lastErr = err
            continue // tenta próximo gateway
        }

        entry.cb.RecordSuccess()
        return resp, nil
    }

    if lastErr != nil {
        return nil, lastErr
    }
    return nil, ErrNoGatewayAvailable
}
```

### 6.2 Circuit Breaker

```go
// pkg/gateway/circuitbreaker.go

package gateway

import (
    "sync"
    "time"
)

type CircuitState int

const (
    StateClosed   CircuitState = 0 // Normal: requisições passam
    StateOpen     CircuitState = 1 // Bloqueado: requisições rejeitadas
    StateHalfOpen CircuitState = 2 // Teste: 1 requisição permitida
)

type CircuitBreaker struct {
    mu           sync.Mutex
    state        CircuitState
    failCount    int
    successCount int
    threshold    int           // Falhas para abrir
    cooldown     time.Duration // Tempo para tentar half-open
    lastFailure  time.Time
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        state:     StateClosed,
        threshold: threshold,
        cooldown:  cooldown,
    }
}

func (cb *CircuitBreaker) IsOpen() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if cb.state == StateClosed {
        return false
    }

    if cb.state == StateOpen {
        if time.Since(cb.lastFailure) > cb.cooldown {
            cb.state = StateHalfOpen
            return false // permite 1 requisição de teste
        }
        return true
    }

    // HalfOpen: já permitiu 1, bloqueia o resto
    return cb.state == StateHalfOpen
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failCount = 0
    cb.successCount++
    cb.state = StateClosed
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failCount++
    cb.lastFailure = time.Now()

    if cb.failCount >= cb.threshold {
        cb.state = StateOpen
    }
}
```

### 6.3 Regras de Seleção

| Cenário | Método | Split? | Gateway Selecionado | Motivo |
|---------|--------|:------:|:--------------------:|--------|
| Pedido simples PIX | PIX | ❌ | Pagar.me | R$ 0,39 (vs R$ 0,99 AbacatePay) |
| Pedido com split PIX | PIX | ✅ | Pagar.me | Split nativo |
| Cartão crédito à vista | Crédito | ❌ | Pagar.me | 1,99% + R$ 0,39 |
| Cartão crédito parcelado | Crédito | ✅ | Pagar.me | Split + MDR configurável |
| Cartão débito | Débito | ✅ | Pagar.me | 0,99% + R$ 0,39 + 3DS |
| Pagar.me fora | Qualquer | ✅ | Asaas | Split nativo |
| Tudo fora | PIX | ❌ | AbacatePay | PIX fallback |

---

## 7. Schema do Banco de Dados

### 7.1 Migração 14: Recebedores

```sql
-- sql/14_recipients.sql
-- FUUDELIVERY — Recebedores multi-gateway
-- Idempotente: pode rodar quantas vezes quiser.

CREATE TABLE IF NOT EXISTS recipients (
    id                   BIGSERIAL PRIMARY KEY,
    user_type            VARCHAR(20) NOT NULL,        -- 'restaurant' | 'delivery_man'
    user_id              INTEGER NOT NULL,
    gateway              VARCHAR(20) NOT NULL,        -- 'pagarme' | 'asaas' | 'abacatepay' | 'mercadopago'
    gateway_recipient_id VARCHAR(128) NOT NULL,       -- ID no gateway
    status               VARCHAR(20) NOT NULL DEFAULT 'pending',
                         -- pending | active | blocked | kyc_pending | kyc_rejected
    bank_account_last4   VARCHAR(4),                  -- Últimos 4 dígitos (auditoria, não expor)
    transfer_interval    VARCHAR(20) DEFAULT 'daily', -- daily | weekly | monthly
    transfer_day         INTEGER,                     -- Dia (1-28 monthly, 1-7 weekly)
    metadata             JSONB DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_recipients_user_gateway UNIQUE (user_type, user_id, gateway),
    CONSTRAINT chk_recipients_user_type CHECK (user_type IN ('restaurant', 'delivery_man')),
    CONSTRAINT chk_recipients_gateway CHECK (gateway IN ('pagarme', 'asaas', 'abacatepay', 'mercadopago')),
    CONSTRAINT chk_recipients_status CHECK (status IN ('pending', 'active', 'blocked', 'kyc_pending', 'kyc_rejected'))
);

CREATE INDEX idx_recipients_user ON recipients (user_type, user_id);
CREATE INDEX idx_recipients_gateway ON recipients (gateway, gateway_recipient_id);
CREATE INDEX idx_recipients_active ON recipients (status) WHERE status = 'active';

COMMENT ON TABLE recipients IS
    'Recebedores multi-gateway. Cada participante (restaurante/entregador) '
    'pode ter sub-contas em vários gateways. Gateway padrão: Pagar.me.';

GRANT SELECT, INSERT, UPDATE, DELETE ON recipients TO app_backend;
GRANT USAGE, SELECT ON SEQUENCES IN SCHEMA public TO app_backend;
```

### 7.2 Migração 15: Regras de Split

```sql
-- sql/15_split_rules.sql
-- FUUDELIVERY — Regras de split por pagamento

CREATE TABLE IF NOT EXISTS payment_split_rules (
    id                BIGSERIAL PRIMARY KEY,
    payment_id        BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    recipient_id      BIGINT NOT NULL REFERENCES recipients(id),
    gateway           VARCHAR(20) NOT NULL,
    gateway_split_id  VARCHAR(128),                    -- ID do split no gateway
    percentage        DECIMAL(5,2),                    -- Percentual (ex: 75.00)
    fixed_value       INTEGER,                         -- Valor fixo em centavos
    amount            INTEGER NOT NULL,                -- Valor efetivo em centavos
    liable            BOOLEAN NOT NULL DEFAULT false,  -- Responsável pelo MDR
    chargeback_responsible BOOLEAN NOT NULL DEFAULT false,
    status            VARCHAR(20) NOT NULL DEFAULT 'pending',
                      -- pending | paid | failed | refunded | blocked
    failure_reason    TEXT,                            -- Motivo da falha
    paid_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_split_payment_recipient UNIQUE (payment_id, recipient_id),
    CONSTRAINT chk_split_status CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'blocked')),
    CONSTRAINT chk_split_amount CHECK (amount > 0)
);

CREATE INDEX idx_split_payment ON payment_split_rules (payment_id);
CREATE INDEX idx_split_recipient ON payment_split_rules (recipient_id);
CREATE INDEX idx_split_pending ON payment_split_rules (status) WHERE status = 'pending';

COMMENT ON TABLE payment_split_rules IS
    'Regras de split por pagamento. Cada linha = uma porção do valor para um recebedor. '
    'Atualizada via webhook quando o gateway confirma o split.';

GRANT SELECT, INSERT, UPDATE, DELETE ON payment_split_rules TO app_backend;
GRANT USAGE, SELECT ON SEQUENCES IN SCHEMA public TO app_backend;
```

### 7.3 Migração 16: Colunas na tabela payments

```sql
-- sql/16_payments_gateway_columns.sql
-- FUUDELIVERY — Colunas multi-gateway na tabela payments

-- Colunas novas (idempotente: ADD COLUMN IF NOT EXISTS)
ALTER TABLE payments ADD COLUMN IF NOT EXISTS gateway VARCHAR(20) DEFAULT 'abacatepay';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS gateway_transaction_id VARCHAR(128);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20) DEFAULT 'pix';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(64);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS authorized_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS captured_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS voided_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS refunded_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS refund_amount INTEGER DEFAULT 0;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS split_applied BOOLEAN DEFAULT false;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(64);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_expires_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS pin_attempts INTEGER DEFAULT 0;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS card_brand VARCHAR(20);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS card_last4 VARCHAR(4);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS installments INTEGER DEFAULT 1;

-- Índices
CREATE INDEX IF NOT EXISTS idx_payments_gateway
    ON payments (gateway, gateway_transaction_id)
    WHERE gateway_transaction_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_payments_idempotency
    ON payments (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_idempotency
    ON payments (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Constraints
ALTER TABLE payments ADD CONSTRAINT chk_payments_gateway
    CHECK (gateway IN ('abacatepay', 'pagarme', 'asaas', 'mercadopago'));

ALTER TABLE payments ADD CONSTRAINT chk_payments_method
    CHECK (payment_method IN ('pix', 'credit_card', 'debit_card'));

-- Comentários
COMMENT ON COLUMN payments.gateway IS 'Gateway que processou o pagamento';
COMMENT ON COLUMN payments.gateway_transaction_id IS 'ID da transação no gateway externo';
COMMENT ON COLUMN payments.payment_method IS 'Método: pix, credit_card, debit_card';
COMMENT ON COLUMN payments.idempotency_key IS 'Chave de idempotência (UUID v4, único)';
COMMENT ON COLUMN payments.authorized_at IS 'Timestamp da pré-autorização (cartão). NULL para PIX.';
COMMENT ON COLUMN payments.captured_at IS 'Timestamp da captura. Para PIX = created_at.';
COMMENT ON COLUMN payments.pin_hash IS 'SHA-256 do PIN de 4 dígitos para confirmação de entrega';
COMMENT ON COLUMN payments.pin_expires_at IS 'Expiração do PIN (TTL 30 minutos)';
COMMENT ON COLUMN payments.pin_attempts IS 'Tentativas de PIN (máx 3)';
```

### 7.4 Atualização do render.yaml

```yaml
envVars:
  # ── Gateway selection ──────────────────────────────
  - key: PAYMENT_GATEWAY_PRIMARY
    value: "pagarme"
  - key: PAYMENT_GATEWAY_FALLBACK
    value: "asaas"
  - key: PAYMENT_SPLIT_ENABLED
    value: "true"
  - key: PAYMENT_ESCROW_ENABLED
    value: "false"
  - key: PAYMENT_PIN_REQUIRED
    value: "true"

  # ── Pagar.me (principal) ──────────────────────────
  - key: PAGARME_API_KEY
    sync: false
  - key: PAGARME_ENCRYPTION_KEY
    sync: false
  - key: PAGARME_WEBHOOK_SECRET
    sync: false

  # ── Asaas (alternativo) ───────────────────────────
  - key: ASAAS_API_KEY
    sync: false
  - key: ASAAS_WEBHOOK_TOKEN
    sync: false

  # ── AbacatePay (fallback PIX) ─────────────────────
  - key: ABACATE_PAY_API_KEY
    sync: false
  - key: ABACATE_PAY_WEBHOOK_SECRET
    sync: false

  # ── Mercado Pago (reserva) ────────────────────────
  - key: MERCADOPAGO_ACCESS_TOKEN
    sync: false
  - key: MERCADOPAGO_WEBHOOK_SECRET
    sync: false
```

---

## 8. Plano de Implementação

### Fase 0 — Fundação (3-4 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 0.1 | Interface `Gateway` + tipos | `pkg/gateway/gateway.go` | Compila, `go vet` limpo |
| 0.2 | `Router` com fallback chain | `pkg/gateway/router.go` | Seleciona gateway correto |
| 0.3 | `CircuitBreaker` | `pkg/gateway/circuitbreaker.go` | Abre após 5 falhas, fecha após 1min |
| 0.4 | `Registry` | `pkg/gateway/registry.go` | Registra e busca por nome |
| 0.5 | Migration 14 (`recipients`) | `sql/14_recipients.sql` | Roda sem erro, idempotente |
| 0.6 | Migration 15 (`split_rules`) | `sql/15_split_rules.sql` | Roda sem erro, idempotente |
| 0.7 | Migration 16 (`payments columns`) | `sql/16_payments_gateway_columns.sql` | Roda sem erro, idempotente |
| 0.8 | Testes unitários Router/CB | `pkg/gateway/*_test.go` | 100% pass |

### Fase 1 — Pagar.me Gateway (5-6 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 1.1 | HTTP client com retry + timeout | `pagarme/client.go` | Retry 3x com backoff |
| 1.2 | CreateTransaction (PIX) | `pagarme/gateway.go` | Retorna QR Code |
| 1.3 | CreateTransaction (crédito, split) | `pagarme/gateway.go` | Split aplicado |
| 1.4 | CreateTransaction (débito, 3DS) | `pagarme/gateway.go` | 3DS redirect |
| 1.5 | CaptureTransaction | `pagarme/gateway.go` | Captura com split |
| 1.6 | RefundTransaction | `pagarme/gateway.go` | Estorno parcial/total |
| 1.7 | VoidTransaction | `pagarme/gateway.go` | Cancela pré-auth |
| 1.8 | CreateRecipient | `pagarme/gateway.go` | Cria sub-conta |
| 1.9 | Webhook HMAC validation | `pagarme/webhook.go` | Valida assinatura |
| 1.10 | ParseWebhook (pagar.me → normalizado) | `pagarme/webhook.go` | Todos os eventos |
| 1.11 | Testes unitários (mock HTTP) | `pagarme/gateway_test.go` | 100% pass |

### Fase 2 — Asaas Gateway (4-5 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 2.1 | HTTP client | `asaas/client.go` | Retry + timeout |
| 2.2 | CreateTransaction (PIX + split) | `asaas/gateway.go` | Split com walletId |
| 2.3 | CreateTransaction (crédito + split) | `asaas/gateway.go` | Cartão com split |
| 2.4 | CreateTransaction (débito) | `asaas/gateway.go` | Débito com split |
| 2.5 | CaptureTransaction | `asaas/gateway.go` | Captura |
| 2.6 | RefundTransaction | `asaas/gateway.go` | Estorno |
| 2.7 | CreateRecipient | `asaas/gateway.go` | Cria wallet |
| 2.8 | Webhook token validation | `asaas/webhook.go` | Valida token |
| 2.9 | ParseWebhook (asaas → normalizado) | `asaas/webhook.go` | Todos os eventos |
| 2.10 | Testes unitários | `asaas/gateway_test.go` | 100% pass |

### Fase 3 — AbacatePay Gateway (2-3 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 3.1 | Adaptar código existente | `abacatepay/gateway.go` | `SupportsSplit() = false` |
| 3.2 | Webhook existente | `abacatepay/webhook.go` | HMAC existente |
| 3.3 | PIX only (sem split, sem cartão) | — | `SupportsMethod(PIX) = true` |
| 3.4 | Testes | `abacatepay/gateway_test.go` | 100% pass |

### Fase 4 — Integração no Monolito (4-5 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 4.1 | Instanciar Router no `main.go` | `cmd/fuudelivery/main.go` | Gateways registrados |
| 4.2 | Integrar Router no `POST /checkout` | `payment_api` | Usa Router, não gateway direto |
| 4.3 | Webhook unificado `POST /payments/webhook/:gateway` | `payment_api` | Roteia por gateway |
| 4.4 | Onboarding `POST /recipients` | `payment_api` | Cria recipient no gateway |
| 4.5 | Atualizar `split.go` | `payment_api/split.go` | Usa SplitRules do gateway |
| 4.6 | Feature flags (`PAYMENT_GATEWAY_PRIMARY`) | `cmd/fuudelivery/main.go` | Configurável via env |
| 4.7 | Dual gateway (AbacatePay + Pagar.me) | — | Transição segura |

### Fase 5 — Pré-autorização, 3DS e PIN (5-6 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 5.1 | `POST /payments/card/authorize` | `payment_api` | Auth com capture=false |
| 5.2 | `POST /payments/card/capture` | `payment_api` | Captura com split |
| 5.3 | `POST /payments/card/cancel` | `payment_api` | Void (sem cobrança) |
| 5.4 | Gerar PIN SHA-256 | `payment_api/pin.go` | Hash armazenado, não plaintext |
| 5.5 | `POST /delivery/confirm-with-pin` | `payment_api` | Valida PIN + captura |
| 5.6 | TTL 30min + max 3 tentativas | `payment_api/pin.go` | Bloqueio automático |
| 5.7 | Novo PIN se expirado | `payment_api/pin.go` | Gera novo hash |
| 5.8 | App entregador: UI PIN | `Frontend/AppEntrega` | Campo de PIN |
| 5.9 | App cliente: exibir PIN | `Frontend/AppComida` | PIN visível |

### Fase 6 — Escrow e Repasse (3-4 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 6.1 | Configurar D+1 (entregador) | Pagar.me dashboard | Transferência diária |
| 6.2 | Configurar D+7 (restaurante) | Pagar.me dashboard | Transferência semanal |
| 6.3 | `POST /wallet/withdraw` | `payment_api` | Solicita saque |
| 6.4 | `GET /wallet/balance` | `payment_api` | Saldo disponível + pendente |
| 6.5 | Ledger de escrow | `wallet.go` | Registra previsão |
| 6.6 | Job de verificação D+X | background job | Atualiza status |

### Fase 7 — Segurança (3-4 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 7.1 | HMAC Pagar.me | `pagarme/webhook.go` | Valida SHA256 |
| 7.2 | HMAC Asaas | `asaas/webhook.go` | Valida token |
| 7.3 | Rate limit `/payments/*` | middleware | 10-20/min por IP |
| 7.4 | Rate limit `/wallet/*` | middleware | 5-10/min por IP |
| 7.5 | Rate limit PIN attempts | `pin.go` | 5/min por pedido |
| 7.6 | Score de risco automático | `payment_api` | ≥40 → bloqueio |
| 7.7 | Logs de auditoria financeira | `audit_log` | Toda operação logada |
| 7.8 | Idempotência reforçada | constraints + código | UNIQUE constraints |

### Fase 8 — Mercado Pago + Testes E2E (3-4 dias)

| # | Tarefa | Arquivo | Critério de aceite |
|---|--------|---------|-------------------|
| 8.1 | Mercado Pago Gateway | `mercadopago/gateway.go` | Split 1:1 funcional |
| 8.2 | OAuth flow | `mercadopago/oauth.go` | Onboarding vendedor |
| 8.3 | Testes E2E: PIX → split → wallet | script | Fluxo completo |
| 8.4 | Testes E2E: Crédito → auth → PIN → capture | script | Fluxo completo |
| 8.5 | Testes E2E: Débito → 3DS → split | script | Fluxo completo |
| 8.6 | Testes de estorno | script | Refund parcial/total |
| 8.7 | Testes de falha | script | Gateway down → fallback |
| 8.8 | Testes de idempotência | script | Duplo POST idempotente |
| 8.9 | Documentação OpenAPI | swagger | Toda API documentada |

**Total: 5-6 semanas** (1 dev) ou **3-4 semanas** (2 devs paralelos)

---

## 9. Segurança e Prevenção de Riscos

### 9.1 Matriz de Riscos e Mitigações

| # | Risco | O que acontece | Culpa | Mitigações no FuuDelivery |
|---|-------|---------------|-------|--------------------------|
| 1 | **Motoboy não entrega** | Cliente pagou, pedido sumiu | Entregador | PIN obrigatório antes de capturar. Sem PIN = sem split. Score de risco ≥40 → bloqueio. |
| 2 | **Restaurante cancela** | Pedido não começou | Restaurante | Void (cartão) ou refund (PIX). Cliente recebe 100%. |
| 3 | **Pedido errado/estragado** | Cliente reclama | Restaurante | Admin usa `/admin/refund`. Estorno da parte do restaurante. Entregador recebe normalmente. |
| 4 | **Cliente cancela pós-preparo** | Restaurante gastou insumos | Cliente | Taxa de cancelamento configurável. Restaurante recebe sua parte. |
| 5 | **Chargeback (fraude)** | Cartão clonado | Cliente/Fraude | 3DS obrigatório. Score de risco. Pagar.me repassa ao recipient liable. |
| 6 | **Gateway fora do ar** | PIX ou cartão não processa | Infra | Circuit breaker + fallback automático. Feature flag para trocar gateway. |
| 7 | **Webhook não chega** | Status desatualizado | Rede | Retry policy no gateway + polling como fallback (GetTransactionStatus). |
| 8 | **Duplo pagamento** | Idempotência falhou | Concorrência | UNIQUE constraint + idempotency_key. Código trata 409 como idempotente. |
| 9 | **PIN vazado** | Alguém vê o PIN | UI | PIN expira em 30min. Máx 3 tentativas. Hash SHA-256 (nunca plaintext). |
| 10 | **Split com valores errados** | Dinheiro vai pra pessoa errada | Código | Validação server-side: soma = 100%, todos recipients ativos, valores positivos. |

### 9.2 PIN de Verificação — Especificação

```
┌─────────────────────────────────────────────────────┐
│              PIN DE VERIFICAÇÃO DE ENTREGA           │
├─────────────────────────────────────────────────────┤
│                                                     │
│  GERAÇÃO:                                           │
│  - 4 dígitos aleatórios (0000-9999)                 │
│  - Armazenado como SHA-256(pin + salt)              │
│  - TTL: 30 minutos                                  │
│  - Máximo: 3 tentativas por PIN                     │
│                                                     │
│  FLUXO:                                             │
│  1. Pedido criado → PIN gerado                      │
│  2. PIN enviado ao cliente via WebSocket            │
│  3. Motoboy chega → app mostra campo "Digite PIN"  │
│  4. Motoboy digita → POST /delivery/confirm-with-pin│
│  5. Validação:                                      │
│     a. PIN correto? ✅ → Captura pagamento + split  │
│     b. PIN incorreto? ❌ → +1 tentativa             │
│     c. Expirado? ❌ → Erro, precisa novo PIN        │
│     d. 3 tentativas? 🔒 → Bloqueio, admin alert     │
│                                                     │
│  SEGURANÇA:                                         │
│  - PIN nunca em plaintext no banco                  │
│  - PIN nunca logado                                 │
│  - PIN nunca retornado em API response              │
│  - Novo PIN gerado se expirado ou bloqueado         │
│  - Auditoria: toda tentativa registrada             │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 9.3 3D Secure (3DS) — Especificação

```
┌─────────────────────────────────────────────────────┐
│              3D SECURE (3DS) PARA CARTÃO             │
├─────────────────────────────────────────────────────┤
│                                                     │
│  OBRIGATÓRIO PARA:                                  │
│  - Cartão de débito (todas as transações)           │
│  - Cartão de crédito com valor > R$ 200             │
│  - Primeira transação do cliente no FuuDelivery     │
│                                                     │
│  OPCIONAL PARA:                                     │
│  - Cartão de crédito ≤ R$ 200 (cliente recorrente)  │
│                                                     │
│  FLUXO:                                             │
│  1. Frontend envia card_token via SDK               │
│  2. Backend cria transação com card_token           │
│  3. Gateway verifica se 3DS necessário              │
│  4. Se sim: retorna auth_url                        │
│  5. Frontend redireciona para 3DS                   │
│  6. Cliente autentica no app/bancário               │
│  7. Gateway recebe resultado (aprovado/reprovado)   │
│  8. Webhook confirma: transaction.paid ou .failed   │
│                                                     │
│  CARGA DE PROVA:                                    │
│  - Com 3DS: fraude é responsabilidade do emissor    │
│  - Sem 3DS: fraude é responsabilidade do merchant   │
│                                                     │
│  INTEGRAÇÃO POR GATEWAY:                            │
│  - Pagar.me: requires_auth + auth_url (obrigatório) │
│  - Asaas: billingType + cardAuthenticate = true     │
│  - Mercado Pago: token + authentication_required    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 9.4 Webhook HMAC Validation

```go
// pkg/gateway/pagarme/webhook.go

package pagarme

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "os"
)

// ValidateWebhook valida a assinatura HMAC-SHA256 do Pagar.me.
// Header: x-pagarme-signature
// Secret: PAGARME_WEBHOOK_SECRET
func (g *PagarMeGateway) ValidateWebhook(body []byte, headers map[string]string) bool {
    signature := headers["x-pagarme-signature"]
    if signature == "" {
        return false
    }

    secret := os.Getenv("PAGARME_WEBHOOK_SECRET")
    if secret == "" {
        return false
    }

    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))

    // Comparação em tempo constante (previne timing attack)
    return hmac.Equal([]byte(signature), []byte(expected))
}

// pkg/gateway/asaas/webhook.go
func (g *AsaasGateway) ValidateWebhook(body []byte, headers map[string]string) bool {
    token := headers["access_token"]
    if token == "" {
        token = headers["Authorization"]
    }

    expected := os.Getenv("ASAAS_WEBHOOK_TOKEN")
    return token == expected && expected != ""
}
```

### 9.5 Rate Limiting

```go
// Middleware de rate limit para endpoints de dinheiro
// Aplicado em: /payments/*, /wallet/*, /recipients/*

rateLimitConfig := map[string]RateLimit{
    "/payments/checkout":     {Max: 20, Window: 1 * time.Minute},
    "/payments/card/authorize": {Max: 10, Window: 1 * time.Minute},
    "/payments/card/capture":   {Max: 10, Window: 1 * time.Minute},
    "/payments/refund":         {Max: 5,  Window: 1 * time.Minute},
    "/wallet/withdraw":         {Max: 5,  Window: 1 * time.Minute},
    "/delivery/confirm-with-pin": {Max: 10, Window: 1 * time.Minute},
}
```

---

## 10. Observabilidade

### 10.1 Métricas Prometheus

```go
// Métricas por gateway
payment_gateway_requests_total{gateway, method, status, split}
payment_gateway_latency_seconds{gateway, method}
payment_gateway_errors_total{gateway, method, error_type}
payment_gateway_circuit_breaker_state{gateway}  // 0=closed, 1=open, 2=half-open

// Métricas de split
payment_split_total{gateway, recipient_type, status}
payment_split_amount_cents{gateway, recipient_type}

// Métricas de PIN
payment_pin_generated_total
payment_pin_validated_total{result}  // success, fail, expired, locked
payment_pin_lockout_total

// Métricas de escrow
payment_escrow_pending_count{gateway}
payment_escrow_pending_amount_cents{gateway}

// Métricas de webhook
payment_webhook_received_total{gateway, event_type}
payment_webhook_validation_failed_total{gateway}
payment_webhook_processing_latency_seconds{gateway}

// Métricas de cartão
payment_card_auth_total{method, brand, result}  // method=credit/debit
payment_card_3ds_required_total{brand}
payment_card_3ds_result_total{result}  // success, failed, abandoned
```

### 10.2 Alertas

| Alerta | Condição | Severidade | Ação |
|--------|----------|:----------:|------|
| Gateway error rate alto | > 5% em 5min | 🔴 P1 | Ativar fallback, notificar admin |
| Circuit breaker aberto | Qualquer gateway | 🟡 P2 | Investigar causa, considerar fallback |
| Webhook validation failures | > 10 em 5min | 🔴 P1 | Possível ataque, bloquear IP |
| Split divergence block | Qualquer evento | 🟡 P2 | Revisar configuração de split |
| PIN lockouts | > 5 em 1h | 🟡 P2 | Possível fraude, investigar |
| Escrow pending growing | > 100 transações | 🟢 P3 | Verificar D+X configuration |
| Card 3DS failures | > 20% em 1h | 🟡 P2 | Verificar configuração 3DS |

### 10.3 Logging Estruturado

```go
// Toda operação financeira gera log estruturado:
log.Printf("[PAYMENT] gateway=%s method=%s order_id=%d amount=%d split=%v status=%s",
    "pagarme", "credit_card", 12345, 5000, true, "authorized")

log.Printf("[SPLIT] gateway=%s payment_id=%d recipient=%s amount=%d status=%s",
    "pagarme", 12345, "rest_01", 3750, "paid")

log.Printf("[PIN] order_id=%d action=%s attempts=%d",
    12345, "validated", 1)

log.Printf("[WEBHOOK] gateway=%s event=%s transaction_id=%s validated=%v",
    "pagarme", "transaction.paid", "txn_abc123", true)

log.Printf("[CIRCUIT_BREAKER] gateway=%s state=%s failures=%d",
    "asaas", "open", 5)
```

---

## 11. Checklist de Deploy

### Pré-deploy

- [ ] Contas criadas em Pagar.me + Asaas (sandbox + produção)
- [ ] API keys geradas e salvas no Render Dashboard
- [ ] Webhooks configurados: `POST /payments/webhook/pagarme` e `/payments/webhook/asaas`
- [ ] Migrations 14, 15, 16 rodadas no Supabase
- [ ] Feature flags: `PAYMENT_GATEWAY_PRIMARY=pagarme`, `PAYMENT_PIN_REQUIRED=true`
- [ ] Testes unitários passando: `go test ./pkg/gateway/...`
- [ ] Testes de integração com sandbox de cada gateway
- [ ] Rate limit configurado
- [ ] HMAC secrets configurados

### Pós-deploy

- [ ] `/health` retorna 200
- [ ] PIX via Pagar.me funciona (criar transação sandbox)
- [ ] Cartão crédito via Pagar.me funciona (com 3DS)
- [ ] Cartão débito via Pagar.me funciona (com 3DS)
- [ ] Split 75/15/10 aplicado corretamente
- [ ] Webhook recebido e processado
- [ ] PIN gerado e validado
- [ ] Fallback para AbacatePay funciona
- [ ] Rate limit ativo
- [ ] Métricas em `/metrics`
- [ ] Logs estruturados aparecendo

### Rollback

1. `PAYMENT_GATEWAY_PRIMARY=abacatepay` no Render
2. Deploy (ou revert commit)
3. Transações em andamento continuam no gateway original
4. Novas transações vão para AbacatePay (PIX sem split)

---

## 12. Custos Estimados

### 12.1 Taxas por gateway

| Método | Pagar.me | Asaas | AbacatePay | Mercado Pago |
|--------|----------|-------|------------|-------------|
| PIX | R$ 0,39 | R$ 0,99 | R$ 0,99 | R$ 0,39 |
| Crédito 1x | 1,99%+R$0,39 | 1,99% | — | 3,99%+R$0,49 |
| Crédito 3x | 2,87% | 2,99% | — | 5,99% |
| Débito | 0,99%+R$0,39 | 1,99% | — | 3,99% |
| Split | Incluso | Incluso | — | Incluso |

### 12.2 Projeção (cenário base: 1.000 pedidos/mês, ticket R$ 50)

| Composição | Volume | Gateway | Custo mensal |
|-----------|--------|---------|-------------|
| PIX (70%) | 700 | Pagar.me | 700 × R$ 0,39 = R$ 273 |
| Crédito 1x (20%) | 200 | Pagar.me | 200 × (1,99%×R$50 + R$0,39) = R$ 278 |
| Débito (10%) | 100 | Pagar.me | 100 × (0,99%×R$50 + R$0,39) = R$ 89 |
| **Total gateway** | | | **R$ 640/mês** |
| Custo repasse manual (atual) | | | R$ 2.000/mês |
| **Custo total atual** | | | **R$ 2.990/mês** |
| **Custo total novo** | | | **R$ 640/mês** |
| **ECONOMIA** | | | **R$ 2.350/mês (79%)** |

---

*Documento v3.0 — Agosto 2026. Referência definitiva para implementação de pagamentos multi-gateway no FuuDelivery.*
