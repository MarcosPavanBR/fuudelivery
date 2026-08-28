// Package asaas implementa o adapter do FuuDelivery para o gateway Asaas.
//
// O Asaas é o gateway alternativo do FuuDelivery, suportando:
//   - PIX (instantâneo)
//   - Cartão de crédito (com split)
//   - Cartão de débito (com split)
//   - Split de pagamento nativo (percentual + fixo via walletId)
//   - Sub-contas (wallets) com transferência automática
//   - Escrow D+X nativo
//   - Webhook com token de validação
//
// Documentação: https://docs.asaas.com/
package asaas

// ═══════════════════════════════════════════════════════════════
// REQUEST TYPES — Criação de Cobrança
// ═══════════════════════════════════════════════════════════════

// CreatePaymentRequest é o payload enviado ao POST /v3/payments
// para criar uma cobrança no Asaas.
type CreatePaymentRequest struct {
	// Customer é o ID do cliente no Asaas.
	Customer string `json:"customer"`

	// BillingType determina o método: "PIX", "CREDIT_CARD", "DEBIT_CARD".
	BillingType string `json:"billingType"`

	// Value é o valor em reais (ex: 50.00).
	Value float64 `json:"value"`

	// Description é a descrição da cobrança.
	Description string `json:"description,omitempty"`

	// ExternalReference é o ID do pedido no FuuDelivery.
	ExternalReference string `json:"externalReference,omitempty"`

	// DueDate é a data de vencimento (formato: yyyy-MM-dd).
	DueDate string `json:"dueDate"`

	// Split define as regras de split.
	Split []SplitRequest `json:"split,omitempty"`

	// CreditCard dados do cartão (quando BillingType = CREDIT_CARD ou DEBIT_CARD).
	CreditCard *CreditCardRequest `json:"creditCard,omitempty"`

	// CreditCardHolder dados do titular do cartão.
	CreditCardHolder *CreditCardHolderRequest `json:"creditCardHolder,omitempty"`

	// Metadata pares chave-valor.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SplitRequest define uma regra de split para o Asaas.
type SplitRequest struct {
	// WalletId é o ID da wallet (sub-conta) do recebedor no Asaas.
	WalletId string `json:"walletId"`

	// FixedValue é o valor fixo em reais.
	FixedValue float64 `json:"fixedValue,omitempty"`

	// PercentualValue é o percentual sobre netValue (0-100).
	PercentualValue float64 `json:"percentualValue,omitempty"`

	// Description é a descrição do split.
	Description string `json:"description,omitempty"`
}

// CreditCardRequest dados do cartão.
type CreditCardRequest struct {
	// Token é o token do cartão (gerado pelo frontend).
	Token string `json:"token"`

	// Installments é o número de parcelas.
	Installments int `json:"installments,omitempty"`

	// Holdback é a percentual de retenção (opcional).
	Holdback float64 `json:"holdback,omitempty"`
}

// CreditCardHolderRequest dados do titular do cartão.
type CreditCardHolderRequest struct {
	Name        string `json:"name"`
	CpfCnpj     string `json:"cpfCnpj"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	MobilePhone string `json:"mobilePhone,omitempty"`
}

// ═══════════════════════════════════════════════════════════════
// RESPONSE TYPES — Criação de Cobrança
// ═══════════════════════════════════════════════════════════════

// CreatePaymentResponse é a resposta do POST /v3/payments.
type CreatePaymentResponse struct {
	ID                    string                        `json:"id"`
	DateCreated           string                        `json:"dateCreated"`
	Customer              string                        `json:"customer"`
	BillingType           string                        `json:"billingType"`
	Status                string                        `json:"status"` // "PENDING", "RECEIVED", "CONFIRMED", etc.
	Value                 float64                       `json:"value"`
	NetValue              float64                       `json:"netValue"`
	Description           string                        `json:"description"`
	ExternalReference     string                        `json:"externalReference"`
	DueDate               string                        `json:"dueDate"`
	OriginalDueDate       string                        `json:"originalDueDate"`
	PaymentDate           string                        `json:"paymentDate,omitempty"`
	TransactionReceiptUrl string                        `json:"transactionReceiptUrl,omitempty"`
	PixQrCode             string                        `json:"pixQrCode,omitempty"`
	PixCopyPaste          string                        `json:"pixCopyPaste,omitempty"`
	InvoiceUrl            string                        `json:"invoiceUrl,omitempty"`
	BankSlipUrl           string                        `json:"bankSlipUrl,omitempty"`
	Chargebacks           []ChargebackResponse          `json:"chargebacks,omitempty"`
	Refunds               []RefundResponse              `json:"refunds,omitempty"`
	Split                 []SplitResponse               `json:"split,omitempty"`
	FinancialTransaction  *FinancialTransactionResponse `json:"financialTransaction,omitempty"`
}

// SplitResponse resultado de um split.
type SplitResponse struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"` // "PENDING", "CREDITED", "REFUSED"
	WalletId        string  `json:"walletId"`
	FixedValue      float64 `json:"fixedValue,omitempty"`
	PercentualValue float64 `json:"percentualValue,omitempty"`
	Amount          float64 `json:"amount"`
	Description     string  `json:"description,omitempty"`
}

// ChargebackResponse resultado de um chargeback.
type ChargebackResponse struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// RefundResponse é a resposta de um estorno.
type RefundResponse struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Value     float64 `json:"value"`
	CreatedAt string  `json:"dateCreated,omitempty"`
}

// FinancialTransactionResponse transação financeira associada.
type FinancialTransactionResponse struct {
	ID           string  `json:"id"`
	Balance      float64 `json:"balance"`
	Amount       float64 `json:"amount"`
	Fee          float64 `json:"fee"`
	EffectedDate string  `json:"effectedDate"`
	Description  string  `json:"description"`
}

// ═══════════════════════════════════════════════════════════════
// REQUEST/RESPONSE TYPES — Clientes
// ═══════════════════════════════════════════════════════════════

// CreateCustomerRequest cria um cliente no Asaas.
type CreateCustomerRequest struct {
	Name              string `json:"name"`
	CpfCnpj           string `json:"cpfCnpj"`
	Email             string `json:"email,omitempty"`
	Phone             string `json:"phone,omitempty"`
	MobilePhone       string `json:"mobilePhone,omitempty"`
	Address           string `json:"address,omitempty"`
	AddressNumber     string `json:"addressNumber,omitempty"`
	Complement        string `json:"complement,omitempty"`
	Province          string `json:"province,omitempty"`
	City              string `json:"city,omitempty"`
	CityName          string `json:"cityName,omitempty"`
	State             string `json:"state,omitempty"`
	PostalCode        string `json:"postalCode,omitempty"`
	Country           string `json:"country,omitempty"`
	ExternalReference string `json:"externalReference,omitempty"`
}

// CreateCustomerResponse é a resposta da criação de um cliente.
type CreateCustomerResponse struct {
	ID                string `json:"id"`
	DateCreated       string `json:"dateCreated"`
	Name              string `json:"name"`
	CpfCnpj           string `json:"cpfCnpj"`
	Email             string `json:"email"`
	ExternalReference string `json:"externalReference"`
}

// ═══════════════════════════════════════════════════════════════
// RESPONSE TYPES — Saldo
// ═══════════════════════════════════════════════════════════════

// BalanceResponse é a resposta de consulta de saldo.
type BalanceResponse struct {
	Available    float64 `json:"available"`
	Unavailable  float64 `json:"unavailable"`
	WaitingFunds float64 `json:"waitingFund"`
}

// ═══════════════════════════════════════════════════════════════
// WEBHOOK TYPES
// ═══════════════════════════════════════════════════════════════

// WebhookPayload é o payload bruto do webhook do Asaas.
type WebhookPayload struct {
	Event      string                 `json:"event"`
	Payment    *WebhookPaymentData    `json:"payment,omitempty"`
	SplitRule  *WebhookSplitData      `json:"splitRule,omitempty"`
	Chargeback *WebhookChargebackData `json:"chargeback,omitempty"`
}

// WebhookPaymentData dados do pagamento no webhook.
type WebhookPaymentData struct {
	ID                    string          `json:"id"`
	DateCreated           string          `json:"dateCreated"`
	Customer              string          `json:"customer"`
	BillingType           string          `json:"billingType"`
	Status                string          `json:"status"`
	Value                 float64         `json:"value"`
	NetValue              float64         `json:"netValue"`
	Description           string          `json:"description"`
	ExternalReference     string          `json:"externalReference"`
	DueDate               string          `json:"dueDate"`
	PaymentDate           string          `json:"paymentDate,omitempty"`
	TransactionReceiptUrl string          `json:"transactionReceiptUrl,omitempty"`
	PixQrCode             string          `json:"pixQrCode,omitempty"`
	PixCopyPaste          string          `json:"pixCopyPaste,omitempty"`
	Split                 []SplitResponse `json:"split,omitempty"`
}

// WebhookSplitData dados de split no webhook.
type WebhookSplitData struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	Payment         string  `json:"payment"`
	WalletId        string  `json:"walletId"`
	FixedValue      float64 `json:"fixedValue,omitempty"`
	PercentualValue float64 `json:"percentualValue,omitempty"`
	Amount          float64 `json:"amount"`
}

// WebhookChargebackData dados de chargeback no webhook.
type WebhookChargebackData struct {
	ID      string  `json:"id"`
	Status  string  `json:"status"`
	Payment string  `json:"payment"`
	Amount  float64 `json:"amount"`
	Reason  string  `json:"reason,omitempty"`
}
