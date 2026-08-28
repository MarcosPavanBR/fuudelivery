package asaas

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// ═══════════════════════════════════════════════════════════════
// GATEWAY ASAAS
// ═══════════════════════════════════════════════════════════════

// AsaasGateway implementa a interface gateway.Gateway para o Asaas.
//
// Capacidades:
//   - PIX: ✅ (payload copia-e-cola)
//   - Cartão de crédito: ✅ (com split)
//   - Cartão de débito: ✅ (com split)
//   - Split: ✅ (percentual + fixo via walletId)
//   - Pré-autorização: ✅ (cartão de crédito)
//   - Escrow: ✅ (D+X nativo)
//
// Segurança:
//   - Webhook: validação por token (access_token)
type AsaasGateway struct {
	client       *Client
	webhookToken string
}

// NewGateway cria uma nova instância do gateway Asaas.
//
// Lê as env vars:
//   - ASAAS_API_KEY (obrigatório)
//   - ASAAS_WEBHOOK_TOKEN (obrigatório para webhook)
func NewGateway() (*AsaasGateway, error) {
	client := NewClient()
	// NewClient no longer returns error

	webhookToken := os.Getenv("ASAAS_WEBHOOK_TOKEN")
	if webhookToken == "" {
		log.Println("[ASAAS] WARNING: ASAAS_WEBHOOK_TOKEN not configured. Webhook validation disabled.")
	}

	return &AsaasGateway{
		client:       client,
		webhookToken: webhookToken,
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — NOME
// ═══════════════════════════════════════════════════════════════

// Name retorna o identificador único do gateway.
func (g *AsaasGateway) Name() string {
	return "asaas"
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — TRANSAÇÕES
// ═══════════════════════════════════════════════════════════════

// CreateTransaction cria uma cobrança no Asaas.
//
// Para PIX:
//   - Cria cobrança com billingType="PIX"
//   - Retorna payload copia-e-cola
//   - Split via array split com walletId
//
// Para cartão de crédito:
//   - Cria cobrança com billingType="CREDIT_CARD"
//   - Dados do cartão via creditCard.token
//   - Split via array split com walletId
//
// Para cartão de débito:
//   - Cria cobrança com billingType="DEBIT_CARD"
//   - Split via array split com walletId
func (g *AsaasGateway) CreateTransaction(
	ctx context.Context,
	req *gateway.TransactionRequest,
) (*gateway.TransactionResponse, error) {

	// Converter valor de centavos para reais
	value := float64(req.Amount) / 100.0

	// Determinar billing type
	billingType := "PIX"
	switch req.PaymentMethod {
	case gateway.MethodCreditCard:
		billingType = "CREDIT_CARD"
	case gateway.MethodDebitCard:
		billingType = "DEBIT_CARD"
	}

	// Criar cliente no Asaas (se necessário)
	customerID, err := g.ensureCustomer(req)
	if err != nil {
		return nil, fmt.Errorf("create transaction: ensure customer: %w", err)
	}

	// Data de vencimento: hoje (para PIX e cartão)
	dueDate := time.Now().Format("2006-01-02")

	// Construir payload
	asaasReq := CreatePaymentRequest{
		Customer:          customerID,
		BillingType:       billingType,
		Value:             value,
		Description:       req.Description,
		ExternalReference: fmt.Sprintf("%d", req.OrderID),
		DueDate:           dueDate,
	}

	// Split rules
	if len(req.SplitRules) > 0 {
		asaasReq.Split = make([]SplitRequest, len(req.SplitRules))
		for i, rule := range req.SplitRules {
			split := SplitRequest{
				WalletId: rule.RecipientID,
			}
			if rule.FixedValue > 0 {
				split.FixedValue = float64(rule.FixedValue) / 100.0
			} else {
				split.PercentualValue = rule.Percentage
			}
			asaasReq.Split[i] = split
		}
	}

	// Dados do cartão
	if req.PaymentMethod == gateway.MethodCreditCard || req.PaymentMethod == gateway.MethodDebitCard {
		if req.CardData != nil {
			asaasReq.CreditCard = &CreditCardRequest{
				Token:        req.CardData.Token,
				Installments: req.CardData.Installments,
			}
			asaasReq.CreditCardHolder = &CreditCardHolderRequest{
				Name:    req.CardData.HolderName,
				CpfCnpj: req.CardData.HolderDoc,
			}
		}
	}

	// Metadata
	if req.Metadata != nil {
		asaasReq.Metadata = req.Metadata
	}

	headers := map[string]string{}
	if req.IdempotencyKey != "" {
		headers["X-Idempotency-Key"] = req.IdempotencyKey
	}

	// Enviar para a API
	respBody, err := g.client.postWithHeaders("/payments", asaasReq, headers)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Parsear resposta
	var asaasResp CreatePaymentResponse
	if err := json.Unmarshal(respBody, &asaasResp); err != nil {
		return nil, fmt.Errorf("create transaction: failed to parse response: %w", err)
	}

	// Mapear para resposta normalizada
	return g.mapResponse(&asaasResp, req), nil
}

// CaptureTransaction captura uma pré-autorização no Asaas.
func (g *AsaasGateway) CaptureTransaction(
	ctx context.Context,
	gatewayID string,
	amount int64,
) error {

	path := fmt.Sprintf("/payments/%s/capture", gatewayID)
	value := float64(amount) / 100.0
	_, err := g.client.post(path, map[string]interface{}{"value": value})
	if err != nil {
		return fmt.Errorf("capture transaction %s: %w", gatewayID, err)
	}

	log.Printf("[ASAAS] Transaction %s captured: %.2f", gatewayID, value)
	return nil
}

// RefundTransaction estorna uma cobrança no Asaas.
func (g *AsaasGateway) RefundTransaction(
	ctx context.Context,
	gatewayID string,
	amount int64,
) (*gateway.RefundResponse, error) {

	value := float64(amount) / 100.0

	refundReq := map[string]interface{}{
		"value": value,
	}

	path := fmt.Sprintf("/payments/%s/refund", gatewayID)
	respBody, err := g.client.post(path, refundReq)
	if err != nil {
		return nil, fmt.Errorf("refund transaction %s: %w", gatewayID, err)
	}

	var asaasRefund RefundResponse
	if err := json.Unmarshal(respBody, &asaasRefund); err != nil {
		return nil, fmt.Errorf("refund: failed to parse response: %w", err)
	}

	estimatedAt, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))

	return &gateway.RefundResponse{
		RefundID:    asaasRefund.ID,
		Gateway:     "asaas",
		Amount:      amount,
		Status:      asaasRefund.Status,
		EstimatedAt: &estimatedAt,
	}, nil
}

// VoidTransaction cancela uma pré-autorização no Asaas.
func (g *AsaasGateway) VoidTransaction(
	ctx context.Context,
	gatewayID string,
) error {

	path := fmt.Sprintf("/payments/%s/cancel", gatewayID)
	_, err := g.client.post(path, nil)
	if err != nil {
		return fmt.Errorf("void transaction %s: %w", gatewayID, err)
	}

	log.Printf("[ASAAS] Transaction %s voided", gatewayID)
	return nil
}

// GetTransactionStatus consulta o status de uma cobrança.
func (g *AsaasGateway) GetTransactionStatus(
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

	return mapAsaasStatus(resp.Status), nil
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — RECEBEDORES
// ═══════════════════════════════════════════════════════════════

// CreateRecipient cria um cliente no Asaas (usado como recipient).
func (g *AsaasGateway) CreateRecipient(
	ctx context.Context,
	req *gateway.RecipientRequest,
) (*gateway.RecipientResponse, error) {

	customerReq := CreateCustomerRequest{
		Name:    req.Name,
		CpfCnpj: req.Document,
		Email:   req.Email,
		Phone:   req.Phone,
	}

	respBody, err := g.client.post("/customers", customerReq)
	if err != nil {
		return nil, fmt.Errorf("create recipient: %w", err)
	}

	var resp CreateCustomerResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("create recipient: failed to parse response: %w", err)
	}

	createdAt, _ := time.Parse("2006-01-02T15:04:05", resp.DateCreated)

	return &gateway.RecipientResponse{
		RecipientID: resp.ID,
		Gateway:     "asaas",
		Status:      gateway.RecipientActive,
		CreatedAt:   createdAt,
	}, nil
}

// UpdateRecipient atualiza um cliente no Asaas.
func (g *AsaasGateway) UpdateRecipient(
	ctx context.Context,
	recipientID string,
	req *gateway.RecipientRequest,
) error {

	customerReq := CreateCustomerRequest{
		Name:    req.Name,
		CpfCnpj: req.Document,
		Email:   req.Email,
		Phone:   req.Phone,
	}

	path := fmt.Sprintf("/customers/%s", recipientID)
	_, err := g.client.post(path, customerReq)
	return err
}

// GetRecipientBalance retorna o saldo de um cliente no Asaas.
func (g *AsaasGateway) GetRecipientBalance(
	ctx context.Context,
	recipientID string,
) (available int64, pending int64, err error) {

	path := fmt.Sprintf("/customers/%s/balance", recipientID)
	respBody, err := g.client.get(path)
	if err != nil {
		return 0, 0, fmt.Errorf("get recipient balance %s: %w", recipientID, err)
	}

	var balance BalanceResponse
	if err := json.Unmarshal(respBody, &balance); err != nil {
		return 0, 0, fmt.Errorf("get balance: failed to parse response: %w", err)
	}

	available = int64(balance.Available * 100) // Reais → centavos
	pending = int64(balance.WaitingFunds * 100)
	return available, pending, nil
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — WEBHOOK
// ═══════════════════════════════════════════════════════════════

// ValidateWebhook valida o token do webhook Asaas.
func (g *AsaasGateway) ValidateWebhook(body []byte, headers map[string]string) bool {
	token := headers["access_token"]
	if token == "" {
		token = headers["Authorization"]
	}

	if g.webhookToken == "" {
		log.Println("[ASAAS] WARNING: Webhook token not configured. Skipping validation.")
		return true
	}

	return token == g.webhookToken
}

// ParseWebhook converte o payload do webhook em um WebhookEvent normalizado.
func (g *AsaasGateway) ParseWebhook(body []byte) (*gateway.WebhookEvent, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook: invalid JSON: %w", err)
	}

	if payload.Payment == nil {
		return nil, fmt.Errorf("parse webhook: missing payment data")
	}

	// Converter valor de reais para centavos
	amount := int64(payload.Payment.Value * 100)

	// Mapear status
	status := mapAsaasStatus(payload.Payment.Status)
	eventType := mapAsaasEventType(payload.Event)

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

	// Mapear método de pagamento
	method := gateway.MethodPIX
	switch payload.Payment.BillingType {
	case "CREDIT_CARD":
		method = gateway.MethodCreditCard
	case "DEBIT_CARD":
		method = gateway.MethodDebitCard
	}

	// Detalhes do split
	var splitDetails []gateway.SplitDetail
	if payload.SplitRule != nil {
		splitAmount := int64(payload.SplitRule.Amount * 100)
		splitDetails = append(splitDetails, gateway.SplitDetail{
			RecipientID: payload.SplitRule.WalletId,
			Amount:      splitAmount,
			Status:      mapAsaasSplitStatus(payload.SplitRule.Status),
		})
	} else if len(payload.Payment.Split) > 0 {
		splitDetails = make([]gateway.SplitDetail, len(payload.Payment.Split))
		for i, split := range payload.Payment.Split {
			splitAmount := int64(split.Amount * 100)
			splitDetails[i] = gateway.SplitDetail{
				RecipientID: split.WalletId,
				Amount:      splitAmount,
				Status:      mapAsaasSplitStatus(split.Status),
			}
		}
	}

	return &gateway.WebhookEvent{
		Gateway:       "asaas",
		EventType:     eventType,
		Type:          normalizedType,
		TransactionID: payload.Payment.ID,
		OrderID:       payload.Payment.ExternalReference,
		Amount:        amount,
		Status:        status,
		SplitDetails:  splitDetails,
		PaymentMethod: method,
		RawPayload:    body,
		ReceivedAt:    time.Now(),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — CAPACIDADES
// ═══════════════════════════════════════════════════════════════

func (g *AsaasGateway) SupportsMethod(method gateway.PaymentMethod) bool {
	switch method {
	case gateway.MethodPIX, gateway.MethodCreditCard, gateway.MethodDebitCard:
		return true
	default:
		return false
	}
}

func (g *AsaasGateway) SupportsSplit() bool     { return true }
func (g *AsaasGateway) SupportsPreAuth() bool   { return true }
func (g *AsaasGateway) Supports3DS() bool       { return true }
func (g *AsaasGateway) SupportsEscrow() bool    { return true }
func (g *AsaasGateway) MaxSplitRecipients() int { return 10 }

// ═══════════════════════════════════════════════════════════════
// HELPERS PRIVADOS
// ═══════════════════════════════════════════════════════════════

// ensureCustomer cria ou busca um cliente no Asaas.
func (g *AsaasGateway) ensureCustomer(req *gateway.TransactionRequest) (string, error) {
	if req.CustomerDoc == "" {
		return "", fmt.Errorf("customer document required")
	}

	customerReq := CreateCustomerRequest{
		Name:    req.CustomerName,
		CpfCnpj: req.CustomerDoc,
		Email:   req.CustomerEmail,
		Phone:   req.CustomerPhone,
	}

	respBody, err := g.client.post("/customers", customerReq)
	if err != nil {
		return "", err
	}

	var resp CreateCustomerResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", err
	}

	return resp.ID, nil
}

// mapResponse converte a resposta do Asaas para o formato normalizado.
func (g *AsaasGateway) mapResponse(resp *CreatePaymentResponse, req *gateway.TransactionRequest) *gateway.TransactionResponse {
	result := &gateway.TransactionResponse{
		GatewayID:    resp.ID,
		Gateway:      "asaas",
		Status:       mapAsaasStatus(resp.Status),
		SplitApplied: len(resp.Split) > 0,
		SplitCount:   len(resp.Split),
	}

	// PIX
	if resp.PixCopyPaste != "" {
		result.PIXCopyPaste = resp.PixCopyPaste
		result.PIXQRCode = resp.PixQrCode
		expiresAt := time.Now().Add(30 * time.Minute)
		result.PIXExpiresAt = &expiresAt
	}

	return result
}

// ═══════════════════════════════════════════════════════════════
// MAPEAMENTO DE STATUS
// ═══════════════════════════════════════════════════════════════

// mapAsaasStatus converte status do Asaas para gateway.TransactionStatus.
func mapAsaasStatus(status string) gateway.TransactionStatus {
	switch status {
	case "PENDING":
		return gateway.StatusPending
	case "RECEIVED":
		return gateway.StatusPaid
	case "CONFIRMED":
		return gateway.StatusPaid
	case "OVERDUE":
		return gateway.StatusExpired
	case "REFUNDED":
		return gateway.StatusRefunded
	case "RECEIVED_IN_CASH":
		return gateway.StatusPaid
	case "AWAITING_RISK_ANALYSIS":
		return gateway.StatusPending
	case "CHARGEBACK_REQUESTED":
		return gateway.StatusChargeback
	case "CHARGEBACK_DISPUTE":
		return gateway.StatusChargeback
	case "AWAITING_CHARGEBACK_REVERSAL":
		return gateway.StatusChargeback
	case "DUNNING_REQUESTED":
		return gateway.StatusPending
	case "DUNNING_RECEIVED":
		return gateway.StatusPaid
	case "AWAITING_CREDIT_CARD_ANALYSIS":
		return gateway.StatusPending
	default:
		return gateway.StatusPending
	}
}

// mapAsaasEventType converte evento do Asaas para tipo normalizado.
func mapAsaasEventType(event string) string {
	switch event {
	case "PAYMENT_RECEIVED", "PAYMENT_CREDITED":
		return "paid"
	case "PAYMENT_OVERDUE":
		return "expired"
	case "PAYMENT_REFUNDED":
		return "refunded"
	case "PAYMENT_SPLIT_DONE":
		return "split_done"
	case "PAYMENT_SPLIT_DIVERGENCE_BLOCK":
		return "split_block"
	case "CHARGEBACK_REQUESTED":
		return "chargeback"
	default:
		return event
	}
}

// mapAsaasSplitStatus converte status de split do Asaas.
func mapAsaasSplitStatus(status string) gateway.SplitStatus {
	switch status {
	case "PENDING":
		return gateway.SplitPending
	case "CREDITED", "DONE":
		return gateway.SplitPaid
	case "REFUSED", "FAILED":
		return gateway.SplitFailed
	case "REFUNDED":
		return gateway.SplitRefunded
	case "BLOCKED":
		return gateway.SplitBlocked
	default:
		return gateway.SplitPending
	}
}
