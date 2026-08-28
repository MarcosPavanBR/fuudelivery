package mercadopago

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// MercadoPagoGateway implementa a interface gateway.Gateway para o Mercado Pago.
//
// ⚠️ LIMITAÇÕES:
//   - Split apenas 1:1 (marketplace → 1 vendedor)
//   - Onboarding de vendedores requer OAuth manual
//   - Taxas mais altas que Pagar.me e Asaas
//
// Uso: último recurso quando todos os outros gateways estiverem indisponíveis.
type MercadoPagoGateway struct {
	client        *Client
	webhookSecret string
}

// NewGateway cria uma nova instância do gateway Mercado Pago.
func NewGateway() (*MercadoPagoGateway, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("mercadopago gateway: %w", err)
	}

	webhookSecret := os.Getenv("MERCADOPAGO_WEBHOOK_SECRET")

	return &MercadoPagoGateway{
		client:        client,
		webhookSecret: webhookSecret,
	}, nil
}

// Name retorna o identificador único do gateway.
func (g *MercadoPagoGateway) Name() string {
	return "mercadopago"
}

// ═══════════════════════════════════════════════════════════════
// TRANSAÇÕES
// ═══════════════════════════════════════════════════════════════

// CreateTransaction cria um pagamento no Mercado Pago.
func (g *MercadoPagoGateway) CreateTransaction(
	ctx context.Context,
	req *gateway.TransactionRequest,
) (*gateway.TransactionResponse, error) {

	// Converter valor de centavos para reais
	amount := float64(req.Amount) / 100.0

	// Determinar payment_method_id
	paymentMethodID := "pix"
	switch req.PaymentMethod {
	case gateway.MethodCreditCard:
		if req.CardData != nil {
			// Bandeira do cartão (precisa ser detectada pelo frontend)
			paymentMethodID = "visa" // Default, frontend detecta a bandeira
		}
	case gateway.MethodDebitCard:
		paymentMethodID = "debit_card"
	}

	// Construir payload
	paymentReq := CreatePaymentRequest{
		TransactionAmount: amount,
		Description:       req.Description,
		ExternalReference: fmt.Sprintf("%d", req.OrderID),
		PaymentMethodID:   paymentMethodID,
		Installments:      1,
		Metadata:          req.Metadata,
	}

	// Dados do cartão
	if req.CardData != nil && req.PaymentMethod != gateway.MethodPIX {
		paymentReq.Token = req.CardData.Token
		paymentReq.Installments = req.CardData.Installments
		if paymentReq.Installments == 0 {
			paymentReq.Installments = 1
		}

		paymentReq.Payer = &PayerRequest{
			Email: req.CustomerEmail,
			Identification: &IdentificationRequest{
				Type:   "CPF",
				Number: req.CustomerDoc,
			},
		}
	}

	// Enviar para a API
	respBody, err := g.client.post("/payments", paymentReq)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Parsear resposta
	var mpResp CreatePaymentResponse
	if err := json.Unmarshal(respBody, &mpResp); err != nil {
		return nil, fmt.Errorf("create transaction: failed to parse response: %w", err)
	}

	// Mapear para resposta normalizada
	return g.mapResponse(&mpResp, req), nil
}

// CaptureTransaction não é suportado (Mercado Pago captura automaticamente).
func (g *MercadoPagoGateway) CaptureTransaction(
	ctx context.Context,
	gatewayID string,
	amount int64,
) error {
	return fmt.Errorf("mercadopago: manual capture not supported")
}

// RefundTransaction estorna um pagamento no Mercado Pago.
func (g *MercadoPagoGateway) RefundTransaction(
	ctx context.Context,
	gatewayID string,
	amount int64,
) (*gateway.RefundResponse, error) {

	// Mercado Pago usa refund parcial via endpoint de refund
	path := fmt.Sprintf("/payments/%s/refunds", gatewayID)
	refundReq := map[string]interface{}{
		"amount": float64(amount) / 100.0,
	}

	respBody, err := g.client.post(path, refundReq)
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

	estimatedAt := time.Now().Add(48 * time.Hour)

	return &gateway.RefundResponse{
		RefundID:    resp.ID,
		Gateway:     "mercadopago",
		Amount:      amount,
		Status:      resp.Status,
		EstimatedAt: &estimatedAt,
	}, nil
}

// VoidTransaction cancela uma pré-autorização no Mercado Pago.
func (g *MercadoPagoGateway) VoidTransaction(
	ctx context.Context,
	gatewayID string,
) error {

	path := fmt.Sprintf("/payments/%s", gatewayID)
	cancelReq := map[string]interface{}{
		"status": "cancelled",
	}

	_, err := g.client.put(path, cancelReq)
	return err
}

// GetTransactionStatus consulta o status de um pagamento.
func (g *MercadoPagoGateway) GetTransactionStatus(
	ctx context.Context,
	gatewayID string,
) (gateway.TransactionStatus, error) {

	path := fmt.Sprintf("/payments/%s", gatewayID)
	respBody, err := g.client.get(path)
	if err != nil {
		return "", fmt.Errorf("get transaction status %s: %w", gatewayID, err)
	}

	var resp CreatePaymentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("get status: failed to parse response: %w", err)
	}

	return mapMPStatus(resp.Status), nil
}

// ═══════════════════════════════════════════════════════════════
// RECEBEDORES
// ═══════════════════════════════════════════════════════════════

// CreateRecipient cria um vendedor no Mercado Pago (requer OAuth).
func (g *MercadoPagoGateway) CreateRecipient(
	ctx context.Context,
	req *gateway.RecipientRequest,
) (*gateway.RecipientResponse, error) {
	// Mercado Pago requer OAuth flow para criar vendedores
	// Isso precisa ser feito manualmente ou via OAuth flow
	return nil, fmt.Errorf("mercadopago: recipient creation requires OAuth flow (manual setup)")
}

// UpdateRecipient atualiza um vendedor.
func (g *MercadoPagoGateway) UpdateRecipient(
	ctx context.Context,
	recipientID string,
	req *gateway.RecipientRequest,
) error {
	return fmt.Errorf("mercadopago: recipient update requires OAuth flow")
}

// GetRecipientBalance retorna o saldo de um vendedor.
func (g *MercadoPagoGateway) GetRecipientBalance(
	ctx context.Context,
	recipientID string,
) (available int64, pending int64, err error) {
	return 0, 0, fmt.Errorf("mercadopago: balance query requires OAuth flow")
}

// ═══════════════════════════════════════════════════════════════
// WEBHOOK
// ═══════════════════════════════════════════════════════════════

// ValidateWebhook valida a assinatura do webhook Mercado Pago.
func (g *MercadoPagoGateway) ValidateWebhook(body []byte, headers map[string]string) bool {
	// Em modo dev (sem secret configurado), aceitar sempre
	if g.webhookSecret == "" {
		return true
	}

	// Mercado Pago usa HMAC-SHA256 com x-signature header
	signature := headers["x-signature"]
	if signature == "" {
		return false
	}

	return validateHMAC(body, signature, g.webhookSecret)
}

// ParseWebhook converte o payload do webhook em um WebhookEvent.
func (g *MercadoPagoGateway) ParseWebhook(body []byte) (*gateway.WebhookEvent, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook: invalid JSON: %w", err)
	}

	if payload.Data == nil {
		return nil, fmt.Errorf("parse webhook: missing data")
	}

	paymentID := payload.Data.ID

	// Tentar buscar detalhes do pagamento (pode falhar em tests/dev)
	var payment CreatePaymentResponse
	var amount int64
	var status gateway.TransactionStatus
	var method gateway.PaymentMethod
	var cardBrand, cardLast4, externalRef string

	path := fmt.Sprintf("/payments/%s", paymentID)
	respBody, err := g.client.get(path)
	if err == nil {
		_ = json.Unmarshal(respBody, &payment)
		amount = int64(payment.TransactionAmount * 100)
		status = mapMPStatus(payment.Status)
		cardBrand = payment.CardBrand
		cardLast4 = payment.CardLastFour
		externalRef = payment.ExternalReference

		switch payment.PaymentTypeID {
		case "credit_card":
			method = gateway.MethodCreditCard
		case "debit_card":
			method = gateway.MethodDebitCard
		default:
			method = gateway.MethodPIX
		}
	} else {
		// Fallback: mapear apenas pela action do webhook
		status = mapMPStatus(payload.Action)
		method = gateway.MethodPIX
	}

	// Mapear tipo normalizado
	var normalizedType gateway.WebhookEventType
	switch status {
	case gateway.StatusPaid, gateway.StatusCaptured:
		normalizedType = gateway.WebhookPaymentApproved
	case gateway.StatusFailed, gateway.StatusExpired:
		normalizedType = gateway.WebhookPaymentFailed
	case gateway.StatusRefunded:
		normalizedType = gateway.WebhookRefundCompleted
	case gateway.StatusVoided:
		normalizedType = gateway.WebhookPaymentCancelled
	default:
		normalizedType = gateway.WebhookPaymentPending
	}

	return &gateway.WebhookEvent{
		Gateway:           "mercadopago",
		GatewayName:       "mercadopago",
		ID:                fmt.Sprintf("mpwh_%s_%d", paymentID, time.Now().UnixNano()),
		EventType:         payload.Action,
		Type:              normalizedType,
		TransactionID:     paymentID,
		GatewayID:         paymentID,
		PaymentExternalID: paymentID,
		OrderID:           externalRef,
		Amount:            amount,
		Status:            status,
		PaymentMethod:     method,
		CardBrand:         cardBrand,
		CardLast4:         cardLast4,
		RawPayload:        body,
		ReceivedAt:        time.Now(),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// CAPACIDADES
// ═══════════════════════════════════════════════════════════════

func (g *MercadoPagoGateway) SupportsMethod(method gateway.PaymentMethod) bool {
	switch method {
	case gateway.MethodPIX, gateway.MethodCreditCard, gateway.MethodDebitCard:
		return true
	default:
		return false
	}
}

func (g *MercadoPagoGateway) SupportsSplit() bool     { return true } // 1:1 only
func (g *MercadoPagoGateway) SupportsPreAuth() bool   { return false }
func (g *MercadoPagoGateway) Supports3DS() bool       { return true }
func (g *MercadoPagoGateway) SupportsEscrow() bool    { return false }
func (g *MercadoPagoGateway) MaxSplitRecipients() int { return 1 } // 1:1 only

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

func (g *MercadoPagoGateway) mapResponse(resp *CreatePaymentResponse, req *gateway.TransactionRequest) *gateway.TransactionResponse {
	result := &gateway.TransactionResponse{
		GatewayID: strconv.FormatInt(resp.ID, 10),
		Gateway:   "mercadopago",
		Status:    mapMPStatus(resp.Status),
		CardBrand: resp.CardBrand,
		CardLast4: resp.CardLastFour,
	}

	// PIX QR Code
	if resp.PointOfInteraction != nil && resp.PointOfInteraction.QRCode != nil {
		result.PIXQRCode = resp.PointOfInteraction.QRCode.QRCode
		result.PIXCopyPaste = resp.PointOfInteraction.QRCode.Ticket
		expiresAt := time.Now().Add(30 * time.Minute)
		result.PIXExpiresAt = &expiresAt
	}

	return result
}

// mapMPStatus converte status do Mercado Pago para status normalizado.
func mapMPStatus(status string) gateway.TransactionStatus {
	switch status {
	case "pending":
		return gateway.StatusPending
	case "authorized":
		return gateway.StatusAuthorized
	case "in_process":
		return gateway.StatusPending
	case "approved":
		return gateway.StatusPaid
	case "rejected":
		return gateway.StatusFailed
	case "cancelled":
		return gateway.StatusVoided
	case "refunded":
		return gateway.StatusRefunded
	case "charged_back":
		return gateway.StatusChargeback
	case "expired":
		return gateway.StatusExpired
	default:
		return gateway.StatusPending
	}
}

// validateHMAC valida uma assinatura HMAC-SHA256.
func validateHMAC(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}
