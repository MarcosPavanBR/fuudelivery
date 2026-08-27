// Package gateway fornece a camada de abstração unificada para gateways
// de pagamento do FuuDelivery. Cada provider (Pagar.me, Asaas, AbacatePay,
// Mercado Pago) implementa a interface Gateway. O código de negócio usa
// apenas esta interface — nunca importa um gateway específico.
//
// Estrutura:
//
//	gateway.go         – Interface Gateway + tipos + enums
//	router.go          – PaymentRouter (seleção + fallback + circuit breaker)
//	circuitbreaker.go  – Circuit breaker por gateway
//	registry.go        – Registro de gateways
//	pagarme/           – Adapter Pagar.me v4
//	asaas/             – Adapter Asaas API
//	abacatepay/        – Adapter AbacatePay (PIX only)
//	mercadopago/       – Adapter Mercado Pago
package gateway

import (
	"context"
	"time"
)

// ═══════════════════════════════════════════════════════════════
// ENUMS
// ═══════════════════════════════════════════════════════════════

// PaymentMethod representa o método de pagamento aceito pelo FuuDelivery.
// Métodos suportados: PIX, Cartão de Crédito, Cartão de Débito.
// Boleto NÃO é suportado (fora do escopo).
type PaymentMethod string

const (
	MethodPIX        PaymentMethod = "pix"
	MethodCreditCard PaymentMethod = "credit_card"
	MethodDebitCard  PaymentMethod = "debit_card"
)

// TransactionStatus representa o ciclo de vida de uma transação
// independentemente do gateway.
type TransactionStatus string

const (
	StatusPending    TransactionStatus = "pending"
	StatusAuthorized TransactionStatus = "authorized" // Cartão: pré-autorizado
	StatusWaiting    TransactionStatus = "waiting"    // PIX: aguardando pagamento
	StatusPaid       TransactionStatus = "paid"       // Confirmado pelo gateway
	StatusCaptured   TransactionStatus = "captured"   // Cartão: capturado (descontado)
	StatusRefunded   TransactionStatus = "refunded"   // Estornado
	StatusVoided     TransactionStatus = "voided"     // Cancelado (pré-autorização)
	StatusFailed     TransactionStatus = "failed"     // Recusado
	StatusExpired    TransactionStatus = "expired"    // PIX/cartão expirado
	StatusChargeback TransactionStatus = "chargeback" // Contestado pelo cliente
)

// SplitStatus representa o estado de uma regra de split individual.
type SplitStatus string

const (
	SplitPending  SplitStatus = "pending"
	SplitPaid     SplitStatus = "paid"
	SplitFailed   SplitStatus = "failed"
	SplitRefunded SplitStatus = "refunded"
	SplitBlocked  SplitStatus = "blocked" // Divergência de valores
)

// RecipientStatus representa o estado de um recebedor no gateway.
type RecipientStatus string

const (
	RecipientPending    RecipientStatus = "pending"
	RecipientActive     RecipientStatus = "active"
	RecipientBlocked    RecipientStatus = "blocked"
	RecipientKYCPending RecipientStatus = "kyc_pending"
	RecipientKYCRejected RecipientStatus = "kyc_rejected"
)

// ═══════════════════════════════════════════════════════════════
// TIPOS DE ENTRADA (REQUEST)
// ═══════════════════════════════════════════════════════════════

// SplitRule define como o valor de um pagamento é dividido entre recebedores.
//
// Regras de validação:
//   - Percentage e FixedValue são mutuamente excludentes por regra.
//   - Soma dos Percentages (ou FixedValues) não pode ultrapassar o Amount.
//   - Pelo menos um recipient deve ter Liable=true (MDR).
//   - Pelo menos um recipient deve ter ChargebackResponsible=true.
type SplitRule struct {
	// RecipientID é o ID do recebedor no gateway externo
	// (walletId no Asaas, recipient_id no Pagar.me, etc.)
	RecipientID string

	// Percentage é o percentual sobre netValue (0-100).
	// Ex: 75.00 = 75% do valor líquido vai para este recipient.
	Percentage float64

	// FixedValue é o valor fixo em centavos.
	// Ex: 500 = R$ 5,00 fixo para este recipient.
	// Se > 0, ignora Percentage.
	FixedValue int64

	// Liable indica se este recipient é responsável pelo MDR
	// (taxa de interchange do cartão de crédito/débito).
	// Pelo menos um recipient deve ter Liable=true.
	Liable bool

	// ChargebackResponsible indica se este recipient é responsável
	// por chargebacks (contestação do cliente).
	ChargebackResponsible bool
}

// TransactionRequest é o pedido unificado de criação de transação.
// O PaymentRouter traduz isso para o formato específico de cada gateway.
type TransactionRequest struct {
	// ── Identificação ──────────────────────────────────────────

	// OrderID é o ID interno do pedido no FuuDelivery.
	OrderID int64

	// IdempotencyKey é uma chave única (UUID v4) para garantir
	// que requisições duplicadas não criem transações duplicadas.
	IdempotencyKey string

	// ── Valores ────────────────────────────────────────────────

	// Amount é o valor total em centavos (ex: R$ 50,00 = 5000).
	Amount int64

	// Currency é o código da moeda (padrão "BRL").
	Currency string

	// ── Método de pagamento ────────────────────────────────────

	// PaymentMethod é o método escolhido: pix, credit_card, debit_card.
	PaymentMethod PaymentMethod

	// ── Dados do cliente ───────────────────────────────────────

	CustomerEmail string
	CustomerName  string
	CustomerDoc   string // CPF ou CNPJ (somente dígitos)
	CustomerPhone string // +5511999999999

	// ── Dados do cartão ────────────────────────────────────────

	// Card contém os dados tokenizados do cartão.
	// nil quando PaymentMethod = pix.
	*CardData

	// ── Split ──────────────────────────────────────────────────

	// SplitRules define como o valor é dividido entre recebedores.
	// Slice vazio = sem split (pagamento entra na conta principal do gateway).
	SplitRules []SplitRule

	// ── Pré-autorização (apenas credit_card) ────────────────────

	// Capture controla se a transação deve ser capturada imediatamente.
	// false = apenas autoriza (pré-autorização). CaptureTransaction é necessário depois.
	// true = autoriza e captura imediatamente (padrão para PIX e débito).
	// Ignorado para PaymentMethod = pix.
	Capture bool

	// CaptureDelay é o número de minutos até auto-capture.
	// 0 = captura manual (via CaptureTransaction).
	// >0 = auto-capture após N minutos (ex: 30 = 30 minutos).
	// Ignorado quando Capture=true.
	CaptureDelay int

	// ── Metadados ──────────────────────────────────────────────

	// Description é a descrição da transação (aparece no extrato do cliente).
	Description string

	// Metadata são pares chave-valor que são salvos no gateway
	// e retornados nos webhooks. Útil para rastrear pedidos.
	// Ex: {"order_id": "123", "customer_phone": "+55..."}
	Metadata map[string]string
}

// CardData contém os dados tokenizados do cartão de crédito ou débito.
// Em produção, o frontend usa o SDK do gateway para tokenizar o cartão
// e envia apenas o token (nunca o número do cartão).
type CardData struct {
	// Token é o token do cartão gerado pelo SDK do gateway no frontend.
	Token string

	// Installments é o número de parcelas (1 = à vista).
	// Ignorado para cartão de débito (sempre 1).
	Installments int

	// HolderName é o nome impresso no cartão.
	HolderName string

	// HolderDoc é o CPF do titular do cartão (somente dígitos).
	HolderDoc string

	// BillingZipCode é o CEP de cobrança do titular.
	BillingZipCode string
}

// RecipientRequest é o pedido unificado de criação de recebedor (sub-conta).
type RecipientRequest struct {
	// UserType é o tipo de participante: "restaurant" ou "delivery_man".
	UserType string

	// UserID é o ID interno no FuuDelivery.
	UserID int64

	// Name é o nome completo ou razão social.
	Name string

	// Document é o CPF ou CNPJ (somente dígitos).
	Document string

	// Email é o e-mail de contato.
	Email string

	// Phone é o telefone com código do país.
	Phone string

	// ── Dados bancários ────────────────────────────────────────

	BankCode      string // Código do BACEN (ex: "341" = Itaú, "001" = BB)
	BankAgency    string // Agência (somente dígitos)
	BankAccount   string // Conta (somente dígitos)
	BankAccountDV string // Dígito verificador da conta
	BankAccountType string // "conta_corrente" | "conta_poupanca"

	// ── Configuração de transferência ──────────────────────────

	TransferInterval string // "daily" | "weekly" | "monthly"
	TransferDay      int    // Dia: 1-28 (monthly), 1-7 (weekly), ignorado (daily)
}

// ═══════════════════════════════════════════════════════════════
// TIPOS DE SAÍDA (RESPONSE)
// ═══════════════════════════════════════════════════════════════

// TransactionResponse é a resposta normalizada de qualquer gateway.
type TransactionResponse struct {
	// GatewayID é o ID da transação no gateway externo.
	GatewayID string

	// Gateway é o nome do gateway ("pagarme", "asaas", etc.)
	Gateway string

	// Status é o estado atual da transação.
	Status TransactionStatus

	// ── PIX ────────────────────────────────────────────────────

	// PIXQRCode é a imagem do QR Code em base64 (data:image/png;base64,...).
	PIXQRCode string

	// PIXCopyPaste é o código copia-e-cola (payload PIX).
	PIXCopyPaste string

	// PIXExpiresAt é a data de expiração do QR Code (geralmente 30 minutos).
	PIXExpiresAt *time.Time

	// ── Cartão ─────────────────────────────────────────────────

	// CardBrand é a bandeira do cartão: "visa", "mastercard", "elo", "amex".
	CardBrand string

	// CardLast4 são os últimos 4 dígitos do cartão.
	CardLast4 string

	// RequiresAuth indica se 3D Secure é necessário.
	// Se true, o frontend deve redirecionar o cliente para AuthURL.
	RequiresAuth bool

	// AuthURL é a URL de autenticação 3D Secure.
	// O frontend redireciona o cliente para esta URL.
	AuthURL string

	// ── Split ──────────────────────────────────────────────────

	// SplitApplied indica se split foi aplicado na transação.
	SplitApplied bool

	// SplitCount é o número de recipients no split.
	SplitCount int

	// ── Metadados ──────────────────────────────────────────────

	Metadata map[string]string
}

// RefundResponse é a resposta de um estorno.
type RefundResponse struct {
	// RefundID é o ID do estorno no gateway.
	RefundID string

	// Gateway é o nome do gateway.
	Gateway string

	// Amount é o valor estornado em centavos.
	Amount int64

	// Status é o estado do estorno: "pending", "processing", "completed".
	Status string

	// EstimatedAt é a previsão de crédito na conta do cliente.
	EstimatedAt *time.Time
}

// RecipientResponse é a resposta de criação/consulta de recebedor.
type RecipientResponse struct {
	// RecipientID é o ID do recebedor no gateway.
	RecipientID string

	// Gateway é o nome do gateway.
	Gateway string

	// Status é o estado do recebedor.
	Status RecipientStatus

	// KYCStatus é o estado da verificação de identidade.
	KYCStatus string

	// Balance é o saldo disponível em centavos.
	Balance int64

	// PendingBalance é o saldo pendente (em custódia/escrow).
	PendingBalance int64

	// CreatedAt é a data de criação no gateway.
	CreatedAt time.Time
}

// ═══════════════════════════════════════════════════════════════
// TIPOS DE WEBHOOK
// ═══════════════════════════════════════════════════════════════

// WebhookEvent é o evento normalizado de qualquer gateway.
// Cada adapter traduz o payload nativo do gateway para esta estrutura.
type WebhookEvent struct {
	// ID é um identificador único do evento (para idempotência).
	ID string

	// Gateway é o nome do gateway que enviou o webhook.
	Gateway string

	// GatewayName é alias para Gateway (compatibilidade com normalizer).
	GatewayName string

	// EventType é o tipo do evento normalizado:
	// "paid", "failed", "refunded", "split_done", "split_block",
	// "authorized", "captured", "voided", "chargeback"
	EventType string

	// Type é alias para EventType (compatibilidade com normalizer).
	Type WebhookEventType

	// TransactionID é o ID da transação no gateway.
	TransactionID string

	// GatewayID é alias para TransactionID (compatibilidade com normalizer).
	GatewayID string

	// PaymentExternalID é alias para TransactionID (compatibilidade).
	PaymentExternalID string

	// OrderID é o ID do pedido extraído do metadata.
	OrderID string

	// Amount é o valor em centavos.
	Amount int64

	// Status é o estado normalizado da transação.
	Status TransactionStatus

	// SplitDetails contém os detalhes de cada split processado.
	SplitDetails []SplitDetail

	// PaymentMethod é o método de pagamento.
	PaymentMethod PaymentMethod

	// CardBrand é a bandeira do cartão (se aplicável).
	CardBrand string

	// CardLast4 são os últimos 4 dígitos (se aplicável).
	CardLast4 string

	// Metadata contém metadados extras do gateway.
	Metadata map[string]string

	// RawPayload é o payload original do gateway (para auditoria).
	RawPayload []byte

	// ReceivedAt é o timestamp de recebimento do webhook.
	ReceivedAt time.Time
}

// WebhookEventType representa o tipo normalizado de evento de webhook.
type WebhookEventType string

const (
	WebhookPaymentApproved WebhookEventType = "payment_approved"
	WebhookPaymentFailed   WebhookEventType = "payment_failed"
	WebhookPaymentPending  WebhookEventType = "payment_pending"
	WebhookPaymentUpdated  WebhookEventType = "payment_updated"
	WebhookPaymentCancelled WebhookEventType = "payment_cancelled"
	WebhookRefundCompleted WebhookEventType = "refund_completed"
	WebhookSplitDone       WebhookEventType = "split_done"
	WebhookSplitFailed     WebhookEventType = "split_failed"
	WebhookUnknown         WebhookEventType = "unknown"
)

// SplitDetail representa o resultado de um split para um recipient específico.
type SplitDetail struct {
	// RecipientID é o ID do recebedor no gateway.
	RecipientID string

	// Amount é o valor transferido em centavos.
	Amount int64

	// Percentage é o percentual aplicado.
	Percentage float64

	// Status é o estado do split: "pending", "paid", "failed".
	Status SplitStatus

	// FailureReason é o motivo da falha (se Status = "failed").
	FailureReason string
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE PRINCIPAL
// ═══════════════════════════════════════════════════════════════

// Gateway é a interface que cada provider de pagamento implementa.
// Todas as operações aceitam context.Context para timeout/cancel.
// Erros seguem o padrão Go: errors.New, errors.Is, errors.As.
//
// Implementações conhecidas:
//   - pagarme.PagarMeGateway
//   - asaas.AsaasGateway
//   - abacatepay.AbacatePayGateway
//   - mercadopago.MercadoPagoGateway
type Gateway interface {
	// Name retorna o identificador único do gateway.
	// Deve ser lowercase e sem espaços: "pagarme", "asaas", "abacatepay", "mercadopago".
	Name() string

	// ─── Transações ─────────────────────────────────────────────

	// CreateTransaction cria uma transação no gateway.
	//
	// Para PIX: retorna QR Code e código copia-e-cola.
	// Para cartão com Capture=true: autoriza e captura imediatamente.
	// Para cartão com Capture=false: apenas autoriza (pré-autorização).
	// Para split: inclui SplitRules na transação.
	//
	// Retorno: TransactionResponse com status e dados para exibição.
	// Erros: errors.Wrap para adicionar contexto.
	CreateTransaction(ctx context.Context, req *TransactionRequest) (*TransactionResponse, error)

	// CaptureTransaction captura uma pré-autorização (cartão apenas).
	// Chamado quando o motoboy confirma a entrega com PIN.
	//
	// amount: valor a capturar em centavos (pode ser menor que o autorizado
	// para estorno parcial).
	//
	// Retorna error se a transação não estiver autorizada ou se o gateway falhar.
	CaptureTransaction(ctx context.Context, gatewayID string, amount int64) error

	// RefundTransaction estorna uma transação paga.
	//
	// amount: valor a estornar em centavos. 0 = estorno total.
	// amount > 0 = estorno parcial.
	//
	// Retorno: RefundResponse com ID do estorno e previsão de crédito.
	RefundTransaction(ctx context.Context, gatewayID string, amount int64) (*RefundResponse, error)

	// VoidTransaction cancela uma pré-autorização (cartão apenas).
	// Diferente de Refund: não há cobrança, apenas libera o bloqueio no cartão.
	// Só funciona se a transação estiver com status "authorized".
	VoidTransaction(ctx context.Context, gatewayID string) error

	// GetTransactionStatus consulta o status atual de uma transação.
	// Usado como fallback quando o webhook não chega.
	GetTransactionStatus(ctx context.Context, gatewayID string) (TransactionStatus, error)

	// ─── Recebedores ────────────────────────────────────────────

	// CreateRecipient cria uma sub-conta no gateway para recebimento de splits.
	// O recebedor precisa ter dados bancários para receber transferências.
	CreateRecipient(ctx context.Context, req *RecipientRequest) (*RecipientResponse, error)

	// UpdateRecipient atualiza dados bancários ou configurações de transferência.
	UpdateRecipient(ctx context.Context, recipientID string, req *RecipientRequest) error

	// GetRecipientBalance retorna o saldo disponível e pendente de um recebedor.
	// Disponível = pronto para saque. Pendente = em custódia (D+X).
	GetRecipientBalance(ctx context.Context, recipientID string) (available int64, pending int64, err error)

	// ─── Webhook ────────────────────────────────────────────────

	// ValidateWebhook valida a assinatura do webhook.
	// Cada gateway tem seu mecanismo: HMAC, token, etc.
	// Retorna true se a assinatura for válida.
	ValidateWebhook(body []byte, headers map[string]string) bool

	// ParseWebhook converte o payload nativo do gateway em um WebhookEvent normalizado.
	// Se o payload não for reconhecido, retorna erro (não panic).
	ParseWebhook(body []byte) (*WebhookEvent, error)

	// ─── Capacidades ────────────────────────────────────────────

	// SupportsMethod retorna true se o gateway suporta o método de pagamento.
	SupportsMethod(method PaymentMethod) bool

	// SupportsSplit retorna true se o gateway suporta split nativo.
	SupportsSplit() bool

	// SupportsPreAuth retorna true se o gateway suporta pré-autorização (cartão).
	SupportsPreAuth() bool

	// Supports3DS retorna true se o gateway suporta 3D Secure.
	Supports3DS() bool

	// SupportsEscrow retorna true se o gateway suporta escrow/D+X.
	SupportsEscrow() bool

	// MaxSplitRecipients retorna o máximo de recipients por transação.
	// 0 = ilimitado.
	MaxSplitRecipients() int
}
