// Package abacatepay implementa o adapter do FuuDelivery para o gateway AbacatePay.
//
// O AbacatePay é o gateway de fallback do FuuDelivery, suportando APENAS PIX.
// Não suporta: split, cartão de crédito/débito, pré-autorização, escrow.
//
// Uso: quando os gateways principais (Pagar.me, Asaas) estiverem indisponíveis,
// ou para transações PIX simples sem split.
//
// Documentação: https://docs.abacatepay.com/
package abacatepay

// ═══════════════════════════════════════════════════════════════
// REQUEST TYPES — Criação de Cobrança
// ═══════════════════════════════════════════════════════════════

// CreateBillingRequest é o payload para criar uma cobrança PIX.
type CreateBillingRequest struct {
	// Amount é o valor em centavos (ex: R$ 50,00 = 5000).
	Amount int64 `json:"amount"`

	// Description é a descrição da cobrança.
	Description string `json:"description,omitempty"`

	// ExternalID é o ID do pedido no FuuDelivery.
	ExternalID string `json:"externalId,omitempty"`

	// Metadata dados extras.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ═══════════════════════════════════════════════════════════════
// RESPONSE TYPES — Criação de Cobrança
// ═══════════════════════════════════════════════════════════════

// CreateBillingResponse é a resposta da criação de cobrança.
type CreateBillingResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"` // "waiting", "paid", "expired"
	Amount     int64  `json:"amount"`
	QRCode     string `json:"qrCode,omitempty"`
	CopyPaste  string `json:"copyPaste,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
}

// ═══════════════════════════════════════════════════════════════
// WEBHOOK TYPES
// ═══════════════════════════════════════════════════════════════

// WebhookPayload é o payload do webhook AbacatePay.
type WebhookPayload struct {
	ID         string `json:"id"`
	Status     string `json:"status"` // "paid", "expired", "refunded"
	Amount     int64  `json:"amount"`
	ExternalID string `json:"externalId,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	PaidAt     string `json:"paidAt,omitempty"`
}
