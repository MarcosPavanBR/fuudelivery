package abacatepay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	} // Construir payload — endpoint v2 /transparents/create, corpo aninhado
	// em "data" com method=PIX (o antigo /v1/charge/pix responde "Not found").
	var pixReq struct {
		Method string               `json:"method"`
		Data   CreateBillingRequest `json:"data"`
	}
	pixReq.Method = "PIX"
	pixReq.Data = CreateBillingRequest{
		Amount:      req.Amount,
		Description: req.Description,
		ExternalID:  externalIDFromRequest(req),
		Metadata:    req.Metadata,
	}

	// Enviar para a API
	headers := map[string]string{}
	if req.IdempotencyKey != "" {
		headers["X-Idempotency-Key"] = req.IdempotencyKey
	}
	respBody, err := g.client.postWithHeaders("/transparents/create", pixReq, headers)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Desembrulhar o envelope v2 {"success", "data", "error"} antes de parsear.
	data, err := unwrapEnvelope(respBody)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Parsear resposta
	var raw struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Amount       int64  `json:"amount"`
		BRCode       string `json:"brCode"`
		BRCodeBase64 string `json:"brCodeBase64"`
		ExpiresAt    string `json:"expiresAt"`
		ExternalID   string `json:"externalId"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("create transaction: failed to parse response: %w", err)
	}

	// Mapear para resposta normalizada.
	//
	// Campos PIX, em paridade com o client legado (services/abacatepay.go):
	// brCode é o copia-e-cola; brCodeBase64 chega com o prefixo
	// "data:image/png;base64," e vai para o frontend como base64 PURO.
	// PIXQRCode e PIXCopyPaste carregam o MESMO brCode: o PixQRCode da tabela
	// payments é lido como copia-e-cola pelo handler de polling e o WebAdmin,
	// e o QRCodeBase64 alimenta o desenho do QR — os três campos existem para
	// manter o contrato de API dos painéis/apps.
	var expiresAt time.Time
	if raw.ExpiresAt != "" {
		expiresAt, _ = time.Parse(time.RFC3339, raw.ExpiresAt)
	}

	return &gateway.TransactionResponse{
		GatewayID:    raw.ID,
		Gateway:      "abacatepay",
		Status:       mapAbacateStatus(raw.Status),
		PIXQRCode:    raw.BRCode,
		PIXCopyPaste: raw.BRCode,
		Metadata:     req.Metadata,
		// extras via metadata não; base64 vai num campo dedicado abaixo.
		PIXQRCodeBase64: stripBase64Prefix(raw.BRCodeBase64),
		PIXExpiresAt:    &expiresAt,
		SplitApplied:    false, // AbacatePay não suporta split
		SplitCount:      0,
	}, nil
}

// externalIDFromRequest resolve o externalId enviado ao gateway: o ID do
// pedido. IDs de pedido legados são STRINGS (hex ObjectID), mas
// TransactionRequest.OrderID é int64 — um id string passa pelo Metadata
// ["order_id"] e o campo numérico é só para pedidos internos numéricos.
// Sem isto, toda cobrança de pedido legado chegava ao dashboard do gateway
// como externalId "0", inútil para conciliação.
func externalIDFromRequest(req *gateway.TransactionRequest) string {
	if id := req.Metadata["order_id"]; id != "" {
		return id
	}
	if req.OrderID > 0 {
		return fmt.Sprintf("%d", req.OrderID)
	}
	return ""
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
// Usa o /transparents/check?id= da v2 (mesma consulta que o webhook faz
// server-side) e desembrulha o envelope antes de mapear o status.
func (g *AbacatePayGateway) GetTransactionStatus(
	ctx context.Context,
	gatewayID string,
) (gateway.TransactionStatus, error) {

	respBody, err := g.client.get("/transparents/check?id=" + gatewayID)
	if err != nil {
		return "", fmt.Errorf("get transaction status %s: %w", gatewayID, err)
	}

	data, err := unwrapEnvelope(respBody)
	if err != nil {
		return "", fmt.Errorf("get status: %w", err)
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
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
	if g.webhookSecret == "" {
		return gateway.AllowUnsignedWebhook("ABACATEPAY", "ABACATE_PAY_WEBHOOK_SECRET")
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
		Gateway:           "abacatepay",
		GatewayName:       "abacatepay",
		ID:                fmt.Sprintf("abtw_%s_%d", payload.ID, time.Now().UnixNano()),
		EventType:         eventType,
		Type:              normalizedType,
		TransactionID:     payload.ID,
		GatewayID:         payload.ID,
		PaymentExternalID: payload.ID,
		OrderID:           payload.ExternalID,
		Amount:            payload.Amount,
		Status:            status,
		PaymentMethod:     gateway.MethodPIX,
		RawPayload:        body,
		ReceivedAt:        time.Now(),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// CAPACIDADES
// ═══════════════════════════════════════════════════════════════

func (g *AbacatePayGateway) SupportsMethod(method gateway.PaymentMethod) bool {
	return method == gateway.MethodPIX // Apenas PIX
}

func (g *AbacatePayGateway) SupportsSplit() bool     { return false }
func (g *AbacatePayGateway) SupportsPreAuth() bool   { return false }
func (g *AbacatePayGateway) Supports3DS() bool       { return false }
func (g *AbacatePayGateway) SupportsEscrow() bool    { return false }
func (g *AbacatePayGateway) MaxSplitRecipients() int { return 0 }

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

// mapAbacateStatus converte status do AbacatePay para status normalizado.
//
// A API v2 mistura caixas: /transparents/create devolve "waiting" (lowercase)
// enquanto /transparents/check e os webhooks devolvem "PAID", "EXPIRED"
// (uppercase). Comparação é case-insensitive para cobrir os dois.
func mapAbacateStatus(status string) gateway.TransactionStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
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
	// "canceled" (1 L) é a grafia que a v2 usa para cobrança cancelada.
	case "canceled", "cancelled":
		return gateway.StatusFailed
	default:
		return gateway.StatusPending
	}
}

// GetChargeDetails consulta a cobrança na API v2 (/transparents/check) e
// devolve o objeto bruto do envelope {"success","data"} — paridade com o
// GetCharge do client legado (services/abacatepay.go). O webhook usa isto
// para re-verificar server-side o status sem confiar no corpo do POST.
func (g *AbacatePayGateway) GetChargeDetails(chargeID string) (map[string]interface{}, error) {
	respBody, err := g.client.get("/transparents/check?id=" + chargeID)
	if err != nil {
		return nil, fmt.Errorf("get charge %s: %w", chargeID, err)
	}

	data, err := unwrapEnvelope(respBody)
	if err != nil {
		return nil, fmt.Errorf("get charge: %w", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("get charge: failed to parse response: %w", err)
	}
	return out, nil
}
