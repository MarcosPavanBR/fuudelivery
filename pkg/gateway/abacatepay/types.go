// Package abacatepay implementa o adapter do FuuDelivery para o gateway AbacatePay.
//
// O AbacatePay é o gateway de fallback do FuuDelivery, suportando APENAS PIX.
// Não suporta: split, cartão de crédito/débito, pré-autorização, escrow.
//
// Uso: quando os gateways principais (Pagar.me, Asaas) estiverem indisponíveis,
// ou para transações PIX simples sem split.
//
// Contrato da API v2 (idêntico ao do client legado em
// Backend/payment_api/app/services/abacatepay.go — mantidos em paridade de
// propósito):
//   - Endpoint de criação: POST /transparents/create (o antigo /v1/charge/pix
//     foi descontinuado e responde "Not found").
//   - Respostas vêm num envelope {"success": bool, "data": ..., "error": ...}.
//   - O corpo vai aninhado em "data": {method: "PIX", data: {...}}.
//   - QR: brCode é o copia-e-cola; brCodeBase64 vem com o prefixo
//     "data:image/png;base64," — o frontend espera o base64 PURO.
//
// Documentação: https://docs.abacatepay.com/
package abacatepay

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

// CreateBillingResponse é a resposta da criação de cobrança, já
// desembrulhada do envelope v2 e normalizada.
type CreateBillingResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"` // "waiting", "paid", "expired", ...
	Amount     int64  `json:"amount"` // centavos
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

// ═══════════════════════════════════════════════════════════════
// ENVELOPE v2
// ═══════════════════════════════════════════════════════════════

// apiEnvelope é o wrapper padrão das respostas v2: {"success": bool, "data": ..., "error": ...}.
type apiEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *string         `json:"error"`
}

// unwrapEnvelope desembrulha o envelope v2 e devolve o "data" puro.
func unwrapEnvelope(body []byte) (json.RawMessage, error) {
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parse envelope v2: %w", err)
	}
	if !env.Success {
		msg := "unknown error"
		if env.Error != nil {
			msg = *env.Error
		}
		return nil, fmt.Errorf("abacatepay API error: %s", msg)
	}
	return env.Data, nil
}

// stripBase64Prefix remove o prefixo "data:image/png;base64," que a API
// antepõe ao brCodeBase64 — o frontend (PIXQRCode.tsx) espera o base64 puro.
func stripBase64Prefix(s string) string {
	if idx := strings.Index(s, "base64,"); idx >= 0 {
		return s[idx+len("base64,"):]
	}
	return s
}
