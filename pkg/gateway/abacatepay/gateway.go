package abacatepay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// AbacatePayGateway implementa a interface gateway.Gateway para o AbacatePay.
//
// ⚠️ LIMITAÇÕES:
//   - Suporta APENAS PIX
//   - NÃO suporta split de pagamento
//   - NÃO suporta cartão de crédito/débito
//   - NÃO suporta pré-autorização
//   - NÃO suporta escrow
//
// Uso: gateway de fallback para PIX simples sem split.
type AbacatePayGateway struct {
	client        *Client
	webhookSecret string
}

// NewGateway cria uma nova instância do gateway AbacatePay.
func NewGateway() (*AbacatePayGateway, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("abacatepay gateway: %w", err)
	}

	webhookSecret := os.Getenv("ABACATE_PAY_WEBHOOK_SECRET")

	return &AbacatePayGateway{
		client:        client,
		webhookSecret: webhookSecret,
	}, nil
}

// Name retorna o identificador único do gateway.
func (g *AbacatePayGateway) Name() string {
	return "abacatepay"
}

// ═══════════════════════════════════════════════════════════════
// TRANSAÇÕES — PIX ONLY
// ═══════════════════════════════════════════════════════════════

// CreateTransaction cria uma cobrança PIX no AbacatePay.
// ⚠️ Apenas PIX. Split NÃO é suportado.
func (g *AbacatePayGateway) CreateTransaction(
	ctx context.Context,
	req *gateway.TransactionRequest,
) (*gateway.TransactionResponse, error) {

	if req.PaymentMethod != gateway.MethodPIX {
		return nil, fmt.Errorf("abacatepay: only PIX is supported, got %s", req.PaymentMethod)
	}

	// Construir payload
	billingReq := CreateBillingRequest{
		Amount:      req.Amount,
		Description: req.Description,
		ExternalID:  fmt.Sprintf("%d", req.OrderID),
		Metadata:    req.Metadata,
	}

	// Enviar para a API
	respBody, err := g.client.post("/billings", billingReq)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Parsear resposta
	var billingResp CreateBillingResponse
	if err := json.Unmarshal(respBody, &billingResp); err != nil {
		return nil, fmt.Errorf("create transaction: failed to parse response: %w", err)
	}

	// Mapear para resposta normalizada
	expiresAt, _ := time.Parse(time.RFC3339, billingResp.ExpiresAt)

	return &gateway.TransactionResponse{
		GatewayID:     billingResp.ID,
		Gateway:       "abacatepay",
		Status:        mapAbacateStatus(billingResp.Status),
		PIXQRCode:     billingResp.QRCode,
		PIXCopyPaste:  billingResp.CopyPaste,
		PIXExpiresAt:  &expiresAt,
		SplitApplied:  false, // AbacatePay não suporta split
		SplitCount:    0,
		Metadata:      req.Metadata,
	}, nil
}

// CaptureTransaction não é suportado no AbacatePay (PIX é instantâneo).
func (g *AbacatePayGateway) CaptureTransaction(
	ctx context.Context,
	gatewayID string,
	amount int64,
) error {
	return fmt.Errorf("abacatepay: capture not supported (PIX is instant)")
}

// RefundTransaction estorna uma cobrança no AbacatePay.
func (g *AbacatePayGateway) RefundTransaction(
	ctx context.Context,
	gatewayID string,
	amount int64,
) (*gateway.RefundResponse, error) {

	path := fmt.Sprintf("/billings/%s/refund", gatewayID)
	respBody, err := g.client.post(path, nil)
	if err != nil {
		return nil, fmt.Errorf("refund transaction %s: %w", gatewayID, err)
	}

	var resp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("refund: failed to parse response: %w", err)
	}

	estimatedAt := time.Now().Add(24 * time.Hour) // PIX estorno leva ~24h

	return &gateway.RefundResponse{
		RefundID:    resp.ID,
		Gateway:     "abacatepay",
		Amount:      amount,
		Status:      resp.Status,
		EstimatedAt: &estimatedAt,
	}, nil
}

// VoidTransaction não é suportado no AbacatePay.
func (g *AbacatePayGateway) VoidTransaction(
	ctx context.Context,
	gatewayID string,
) error {
	return fmt.Errorf("abacatepay: void not supported (PIX is instant)")
}

// GetTransactionStatus consulta o status de uma cobrança.
func (g *AbacatePayGateway) GetTransactionStatus(
	ctx context.Context,
	gatewayID string,
) (gateway.TransactionStatus, error) {

	path := fmt.Sprintf("/billings/%s", gatewayID)
	respBody, err := g.client.get(path)
	if err != nil {
		return "", fmt.Errorf("get transaction status %s: %w", gatewayID, err)
	}

	var resp CreateBillingResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("get status: failed to parse response: %w", err)
	}

	return mapAbacateStatus(resp.Status), nil
}

// ═══════════════════════════════════════════════════════════════
// RECEBEDORES — NÃO SUPORTADO
// ═══════════════════════════════════════════════════════════════

// CreateRecipient não é suportado no AbacatePay (sem sub-contas).
func (g *AbacatePayGateway) CreateRecipient(
	ctx context.Context,
	req *gateway.RecipientRequest,
) (*gateway.RecipientResponse, error) {
	return nil, fmt.Errorf("abacatepay: recipients not supported (no split)")
}

// UpdateRecipient não é suportado.
func (g *AbacatePayGateway) UpdateRecipient(
	ctx context.Context,
	recipientID string,
	req *gateway.RecipientRequest,
) error {
	return fmt.Errorf("abacatepay: recipients not supported")
}

// GetRecipientBalance não é suportado.
func (g *AbacatePayGateway) GetRecipientBalance(
	ctx context.Context,
	recipientID string,
) (available int64, pending int64, err error) {
	return 0, 0, fmt.Errorf("abacatepay: balance not supported")
}

// ═══════════════════════════════════════════════════════════════
// WEBHOOK
// ═══════════════════════════════════════════════════════════════

// ValidateWebhook valida a assinatura do webhook AbacatePay.
func (g *AbacatePayGateway) ValidateWebhook(body []byte, headers map[string]string) bool {
	// Em modo dev (sem secret configurado), aceitar sempre
	if g.webhookSecret == "" {
		return true
	}

	signature := headers["x-abacatepay-signature"]
	if signature == "" {
		return false
	}

	return ValidateHMAC(body, signature, g.webhookSecret)
}

// ParseWebhook converte o payload do webhook em um WebhookEvent.
func (g *AbacatePayGateway) ParseWebhook(body []byte) (*gateway.WebhookEvent, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook: invalid JSON: %w", err)
	}

	status := mapAbacateStatus(payload.Status)
	eventType := payload.Status

	// Mapear para o tipo normalizado
	var normalizedType gateway.WebhookEventType
	switch payload.Status {
	case "paid":
		normalizedType = gateway.WebhookPaymentApproved
	case "expired", "refused":
		normalizedType = gateway.WebhookPaymentFailed
	case "refunded":
		normalizedType = gateway.WebhookRefundCompleted
	default:
		normalizedType = gateway.WebhookPaymentPending
	}

	return &gateway.WebhookEvent{
		Gateway:       "abacatepay",
		GatewayName:   "abacatepay",
		ID:            fmt.Sprintf("abtw_%s_%d", payload.ID, time.Now().UnixNano()),
		EventType:     eventType,
		Type:          normalizedType,
		TransactionID: payload.ID,
		GatewayID:     payload.ID,
		PaymentExternalID: payload.ID,
		OrderID:       payload.ExternalID,
		Amount:        payload.Amount,
		Status:        status,
		PaymentMethod: gateway.MethodPIX,
		RawPayload:    body,
		ReceivedAt:    time.Now(),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// CAPACIDADES
// ═══════════════════════════════════════════════════════════════

func (g *AbacatePayGateway) SupportsMethod(method gateway.PaymentMethod) bool {
	return method == gateway.MethodPIX // Apenas PIX
}

func (g *AbacatePayGateway) SupportsSplit() bool            { return false }
func (g *AbacatePayGateway) SupportsPreAuth() bool           { return false }
func (g *AbacatePayGateway) Supports3DS() bool               { return false }
func (g *AbacatePayGateway) SupportsEscrow() bool            { return false }
func (g *AbacatePayGateway) MaxSplitRecipients() int         { return 0 }

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

// mapAbacateStatus converte status do AbacatePay para status normalizado.
func mapAbacateStatus(status string) gateway.TransactionStatus {
	switch status {
	case "waiting":
		return gateway.StatusWaiting
	case "paid":
		return gateway.StatusPaid
	case "expired":
		return gateway.StatusExpired
	case "refunded":
		return gateway.StatusRefunded
	case "refused":
		return gateway.StatusFailed
	default:
		return gateway.StatusPending
	}
}
