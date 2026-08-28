// Package mercadopago implementa o adapter do FuuDelivery para o Mercado Pago.
//
// O Mercado Pago é o gateway reserva do FuuDelivery, suportando:
//   - PIX (instantâneo)
//   - Cartão de crédito (com split 1:1)
//   - Cartão de débito (com split 1:1)
//   - Split 1:1 via marketplace_fee (limitado — apenas 1 vendedor por transação)
//
// ⚠️ LIMITAÇÕES:
//   - Split apenas 1:1 (marketplace → 1 vendedor)
//   - Onboarding de vendedores requer OAuth manual
//   - Taxas mais altas que Pagar.me e Asaas
//
// Uso: último recurso quando todos os outros gateways estiverem indisponíveis.
//
// Documentação: https://www.mercadopago.com.br/developers/pt/docs
package mercadopago

// ═══════════════════════════════════════════════════════════════
// REQUEST TYPES
// ═══════════════════════════════════════════════════════════════

// CreatePaymentRequest é o payload para criar um pagamento.
type CreatePaymentRequest struct {
	// TransactionAmount é o valor em reais.
	TransactionAmount float64 `json:"transaction_amount"`

	// Token é o token do cartão (para cartão de crédito/débito).
	Token string `json:"token,omitempty"`

	// Description é a descrição do pagamento.
	Description string `json:"description,omitempty"`

	// Installments é o número de parcelas.
	Installments int `json:"installments,omitempty"`

	// PaymentMethodID é o método: "pix", "visa", "mastercard", etc.
	PaymentMethodID string `json:"payment_method_id,omitempty"`

	// Payer dados do pagador.
	Payer *PayerRequest `json:"payer,omitempty"`

	// ExternalReference é o ID do pedido.
	ExternalReference string `json:"external_reference,omitempty"`

	// Metadata dados extras.
	Metadata map[string]string `json:"metadata,omitempty"`

	// StatementDescription descrição no extrato.
	StatementDescription string `json:"statement_descriptor,omitempty"`
}

// PayerRequest dados do pagador.
type PayerRequest struct {
	Email          string                 `json:"email,omitempty"`
	FirstName      string                 `json:"first_name,omitempty"`
	LastName       string                 `json:"last_name,omitempty"`
	Identification *IdentificationRequest `json:"identification,omitempty"`
}

// IdentificationRequest documento do pagador.
type IdentificationRequest struct {
	Type   string `json:"type"`   // "CPF" ou "CNPJ"
	Number string `json:"number"` // Somente dígitos
}

// ═══════════════════════════════════════════════════════════════
// RESPONSE TYPES
// ═══════════════════════════════════════════════════════════════

// CreatePaymentResponse é a resposta da criação de pagamento.
type CreatePaymentResponse struct {
	ID                 int64               `json:"id"`
	Status             string              `json:"status"` // "pending", "approved", "rejected", "authorized", "in_process", "cancelled"
	StatusDetail       string              `json:"status_detail"`
	PaymentTypeID      string              `json:"payment_type_id"`   // "credit_card", "debit_card", "bank_transfer"
	PaymentMethodID    string              `json:"payment_method_id"` // "pix", "visa", "mastercard"
	TransactionAmount  float64             `json:"transaction_amount"`
	Description        string              `json:"description"`
	DateCreated        string              `json:"date_created"`
	DateApproved       string              `json:"date_approved,omitempty"`
	ExternalReference  string              `json:"external_reference"`
	PointOfInteraction *PointOfInteraction `json:"point_of_interaction,omitempty"`
	CardLastFour       string              `json:"card_last_four_digits,omitempty"`
	CardBrand          string              `json:"card_payment_type,omitempty"` // "visa", "mastercard"
}

// PointOfInteraction dados de interação (PIX QR Code).
type PointOfInteraction struct {
	Type   string      `json:"type"` // "PIX"
	QRCode *QRCodeData `json:"transaction_data,omitempty"`
}

// QRCodeData dados do QR Code PIX.
type QRCodeData struct {
	QRCode string `json:"qr_code_base64"`
	Ticket string `json:"ticket"`
}

// ═══════════════════════════════════════════════════════════════
// WEBHOOK TYPES
// ═══════════════════════════════════════════════════════════════

// WebhookPayload é o payload do webhook Mercado Pago.
type WebhookPayload struct {
	ID          int64        `json:"id"`
	Action      string       `json:"action"` // "payment.created", "payment.updated"
	LiveMode    bool         `json:"live_mode"`
	DateCreated string       `json:"date_created"`
	UserID      int64        `json:"user_id"`
	APIVersion  string       `json:"api_version"`
	Data        *WebhookData `json:"data,omitempty"`
}

// WebhookData dados do webhook.
type WebhookData struct {
	ID string `json:"id"` // ID do pagamento
}
