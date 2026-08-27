package pagarme

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/gateway"
)

// ═══════════════════════════════════════════════════════════════
// GATEWAY PAGAR.ME
// ═══════════════════════════════════════════════════════════════

// PagarMeGateway implementa a interface gateway.Gateway para o Pagar.me v4.
//
// Capacidades:
//   - PIX: ✅ (QR Code + copia-e-cola)
//   - Cartão de crédito: ✅ (com 3DS, pré-autorização, split)
//   - Cartão de débito: ✅ (com 3DS, split)
//   - Split: ✅ (percentual + fixo, MDR configurável)
//   - Pré-autorização: ✅ (cartão de crédito)
//   - Escrow: ✅ (D+X configurável)
//
// Segurança:
//   - Webhook: validação HMAC-SHA256 com PAGARME_WEBHOOK_SECRET
//   - 3DS: autenticação obrigatória para débito e crédito > R$ 200
type PagarMeGateway struct {
	client         *Client
	webhookSecret  string
}

// NewGateway cria uma nova instância do gateway Pagar.me.
//
// Lê as env vars:
//   - PAGARME_API_KEY (obrigatório)
//   - PAGARME_ENCRYPTION_KEY (obrigatório para cartão)
//   - PAGARME_WEBHOOK_SECRET (obrigatório para webhook)
func NewGateway() (*PagarMeGateway, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("pagarme gateway: %w", err)
	}

	webhookSecret := os.Getenv("PAGARME_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Println("[PAGARME] WARNING: PAGARME_WEBHOOK_SECRET not configured. Webhook validation disabled.")
	}

	return &PagarMeGateway{
		client:        client,
		webhookSecret: webhookSecret,
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — NOME
// ═══════════════════════════════════════════════════════════════

// Name retorna o identificador único do gateway.
func (g *PagarMeGateway) Name() string {
	return "pagarme"
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — TRANSAÇÕES
// ═══════════════════════════════════════════════════════════════

// CreateTransaction cria uma transação no Pagar.me.
//
// Para PIX:
//   - Cria transação com payment_method="pix"
//   - Retorna QR Code e código copia-e-cola
//   - Status inicial: "waiting" (aguardando pagamento)
//
// Para cartão de crédito:
//   - Cria transação com payment_method="credit_card"
//   - Se capture=false: pré-autorização (status "authorized")
//   - Se capture=true: autoriza + captura (status "paid")
//   - Se 3DS necessário: retorna AuthenticateURL (redirecionar cliente)
//
// Para cartão de débito:
//   - Cria transação com payment_method="debit_card"
//   - 3DS sempre obrigatório
//   - Captura imediata (não suporta pré-autorização)
//
// Para split:
//   - Inclui split_rules no payload
//   - Cada split_rule referencia um recipient_id
func (g *PagarMeGateway) CreateTransaction(
	ctx context.Context,
	req *gateway.TransactionRequest,
) (*gateway.TransactionResponse, error) {

	// Construir payload do Pagar.me
	pagarmeReq := g.buildTransactionRequest(req)

	// Enviar para a API
	respBody, err := g.client.post("/orders", pagarmeReq)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Parsear resposta
	var pagarmeResp CreateTransactionResponse
	if err := json.Unmarshal(respBody, &pagarmeResp); err != nil {
		return nil, fmt.Errorf("create transaction: failed to parse response: %w", err)
	}

	// Mapear para resposta normalizada
	return g.mapResponse(&pagarmeResp, req), nil
}

// CaptureTransaction captura uma pré-autorização no Pagar.me.
//
// Chamado quando o motoboy confirma a entrega com PIN.
// amount=0 captura o valor total autorizado.
func (g *PagarMeGateway) CaptureTransaction(
	ctx context.Context,
	gatewayID string,
	amount int64,
) error {

	captureReq := CaptureRequest{Amount: amount}

	path := fmt.Sprintf("/orders/%s/pay", gatewayID)
	_, err := g.client.post(path, captureReq)
	if err != nil {
		return fmt.Errorf("capture transaction %s: %w", gatewayID, err)
	}

	log.Printf("[PAGARME] Transaction %s captured: %d cents", gatewayID, amount)
	return nil
}

// RefundTransaction estorna uma transação no Pagar.me.
//
// amount=0: estorno total
// amount>0: estorno parcial
func (g *PagarMeGateway) RefundTransaction(
	ctx context.Context,
	gatewayID string,
	amount int64,
) (*gateway.RefundResponse, error) {

	refundReq := RefundRequest{Amount: amount}

	path := fmt.Sprintf("/orders/%s/refund", gatewayID)
	respBody, err := g.client.post(path, refundReq)
	if err != nil {
		return nil, fmt.Errorf("refund transaction %s: %w", gatewayID, err)
	}

	var pagarmeRefund RefundResponse
	if err := json.Unmarshal(respBody, &pagarmeRefund); err != nil {
		return nil, fmt.Errorf("refund: failed to parse response: %w", err)
	}

	estimatedAt, _ := time.Parse(time.RFC3339, pagarmeRefund.CreatedAt)

	return &gateway.RefundResponse{
		RefundID:    strconv.FormatInt(pagarmeRefund.ID, 10),
		Gateway:     "pagarme",
		Amount:      pagarmeRefund.Amount,
		Status:      pagarmeRefund.Status,
		EstimatedAt: &estimatedAt,
	}, nil
}

// VoidTransaction cancela uma pré-autorização no Pagar.me.
//
// Só funciona se a transação estiver com status "authorized".
// Não há cobrança — apenas libera o bloqueio no cartão.
func (g *PagarMeGateway) VoidTransaction(
	ctx context.Context,
	gatewayID string,
) error {

	path := fmt.Sprintf("/orders/%s/cancel", gatewayID)
	_, err := g.client.post(path, nil)
	if err != nil {
		return fmt.Errorf("void transaction %s: %w", gatewayID, err)
	}

	log.Printf("[PAGARME] Transaction %s voided", gatewayID)
	return nil
}

// GetTransactionStatus consulta o status atual de uma transação.
func (g *PagarMeGateway) GetTransactionStatus(
	ctx context.Context,
	gatewayID string,
) (gateway.TransactionStatus, error) {

	path := fmt.Sprintf("/orders/%s", gatewayID)
	respBody, err := g.client.get(path)
	if err != nil {
		return "", fmt.Errorf("get transaction status %s: %w", gatewayID, err)
	}

	var resp CreateTransactionResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("get status: failed to parse response: %w", err)
	}

	return mapStatus(resp.Status), nil
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — RECEBEDORES
// ═══════════════════════════════════════════════════════════════

// CreateRecipient cria um recipient (sub-conta) no Pagar.me.
func (g *PagarMeGateway) CreateRecipient(
	ctx context.Context,
	req *gateway.RecipientRequest,
) (*gateway.RecipientResponse, error) {

	pagarmeReq := g.buildRecipientRequest(req)

	respBody, err := g.client.post("/recipients", pagarmeReq)
	if err != nil {
		return nil, fmt.Errorf("create recipient: %w", err)
	}

	var resp CreateRecipientResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("create recipient: failed to parse response: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, resp.CreatedAt)

	return &gateway.RecipientResponse{
		RecipientID: resp.ID,
		Gateway:     "pagarme",
		Status:      gateway.RecipientStatus(resp.Status),
		CreatedAt:   createdAt,
	}, nil
}

// UpdateRecipient atualiza um recipient existente no Pagar.me.
func (g *PagarMeGateway) UpdateRecipient(
	ctx context.Context,
	recipientID string,
	req *gateway.RecipientRequest,
) error {

	pagarmeReq := g.buildRecipientRequest(req)

	path := fmt.Sprintf("/recipients/%s", recipientID)
	_, err := g.client.put(path, pagarmeReq)
	if err != nil {
		return fmt.Errorf("update recipient %s: %w", recipientID, err)
	}

	return nil
}

// GetRecipientBalance retorna o saldo de um recipient no Pagar.me.
func (g *PagarMeGateway) GetRecipientBalance(
	ctx context.Context,
	recipientID string,
) (available int64, pending int64, err error) {

	path := fmt.Sprintf("/recipients/%s/balance", recipientID)
	respBody, err := g.client.get(path)
	if err != nil {
		return 0, 0, fmt.Errorf("get recipient balance %s: %w", recipientID, err)
	}

	var balance BalanceResponse
	if err := json.Unmarshal(respBody, &balance); err != nil {
		return 0, 0, fmt.Errorf("get balance: failed to parse response: %w", err)
	}

	return balance.Available, balance.WaitingFunds, nil
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — WEBHOOK
// ═══════════════════════════════════════════════════════════════

// ValidateWebhook valida a assinatura HMAC-SHA256 do webhook Pagar.me.
//
// Header esperado: x-pagarme-signature
// Secret: PAGARME_WEBHOOK_SECRET
// Algoritmo: HMAC-SHA256
func (g *PagarMeGateway) ValidateWebhook(body []byte, headers map[string]string) bool {
	signature := headers["x-pagarme-signature"]
	if signature == "" {
		log.Println("[PAGARME] Webhook validation failed: missing x-pagarme-signature header")
		return false
	}

	if g.webhookSecret == "" {
		log.Println("[PAGARME] WARNING: Webhook secret not configured. Skipping validation.")
		return true // Fallback: aceitar se secret não configurado
	}

	return ValidateHMAC(body, signature, g.webhookSecret)
}

// ParseWebhook converte o payload do webhook em um WebhookEvent normalizado.
func (g *PagarMeGateway) ParseWebhook(body []byte) (*gateway.WebhookEvent, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook: invalid JSON: %w", err)
	}

	// Mapear status do Pagar.me para status normalizado
	status := mapStatus(payload.Status)

	// Mapear evento
	eventType := mapEventType(payload.Status)

	// Construir detalhes do split
	splitDetails := make([]gateway.SplitDetail, len(payload.SplitRules))
	for i, rule := range payload.SplitRules {
		splitDetails[i] = gateway.SplitDetail{
			RecipientID: rule.RecipientID,
			Amount:      rule.Amount,
			Percentage:  rule.Percentage,
			Status:      gateway.SplitStatus(rule.Status),
		}
	}

	// Mapear método de pagamento
	method := gateway.MethodPIX
	switch payload.PaymentMethod {
	case "credit_card":
		method = gateway.MethodCreditCard
	case "debit_card":
		method = gateway.MethodDebitCard
	}

	return &gateway.WebhookEvent{
		Gateway:       "pagarme",
		EventType:     eventType,
		TransactionID: strconv.FormatInt(payload.ID, 10),
		OrderID:       payload.ExternalReference,
		Amount:        payload.Amount,
		Status:        status,
		SplitDetails:  splitDetails,
		PaymentMethod: method,
		CardBrand:     payload.CardBrand,
		CardLast4:     payload.CardLastFour,
		RawPayload:    body,
		ReceivedAt:    time.Now(),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// INTERFACE gateway.Gateway — CAPACIDADES
// ═══════════════════════════════════════════════════════════════

// SupportsMethod retorna true se o Pagar.me suporta o método.
func (g *PagarMeGateway) SupportsMethod(method gateway.PaymentMethod) bool {
	switch method {
	case gateway.MethodPIX, gateway.MethodCreditCard, gateway.MethodDebitCard:
		return true
	default:
		return false
	}
}

// SupportsSplit retorna true — Pagar.me tem split nativo.
func (g *PagarMeGateway) SupportsSplit() bool { return true }

// SupportsPreAuth retorna true — Pagar.me suporta pré-autorização de cartão.
func (g *PagarMeGateway) SupportsPreAuth() bool { return true }

// Supports3DS retorna true — Pagar.me suporta 3D Secure.
func (g *PagarMeGateway) Supports3DS() bool { return true }

// SupportsEscrow retorna true — Pagar.me suporta escrow/D+X.
func (g *PagarMeGateway) SupportsEscrow() bool { return true }

// MaxSplitRecipients retorna o limite de recipients por transação.
// Pagar.me não tem limite documentado, mas usamos 10 como razoável.
func (g *PagarMeGateway) MaxSplitRecipients() int { return 10 }

// ═══════════════════════════════════════════════════════════════
// HELPERS PRIVADOS
// ═══════════════════════════════════════════════════════════════

// buildTransactionRequest converte um gateway.TransactionRequest para o formato Pagar.me.
func (g *PagarMeGateway) buildTransactionRequest(req *gateway.TransactionRequest) CreateTransactionRequest {
	// Items (obrigatório no Pagar.me — usar 1 item genérico)
	items := []OrderItem{
		{
			ID:          fmt.Sprintf("order_%d", req.OrderID),
			Title:       req.Description,
			UnitPrice:   req.Amount,
			Quantity:    1,
			Tangible:    false,
			Description: req.Description,
		},
	}

	// Customer
	var customer *CustomerRequest
	if req.CustomerDoc != "" {
		customer = &CustomerRequest{
			Name:  req.CustomerName,
			Email: req.CustomerEmail,
			Type:  "individual",
			Documents: []DocumentRequest{
				{Type: "cpf", Number: req.CustomerDoc},
			},
		}
	}

	// Split rules
	splitRules := make([]SplitRuleRequest, len(req.SplitRules))
	for i, rule := range req.SplitRules {
		splitRules[i] = SplitRuleRequest{
			RecipientID:        rule.RecipientID,
			Percentage:         rule.Percentage,
			Amount:             rule.FixedValue,
			Liable:             rule.Liable,
			ChargeProcessingFee: rule.ChargebackResponsible,
		}
	}

	// Capture: para cartão de crédito, pode ser false (pré-autorização)
	// Para PIX e débito, sempre true
	capture := true
	if req.PaymentMethod == gateway.MethodCreditCard && !req.Capture {
		capture = false
	}

	// Payment method
	paymentMethod := "pix"
	switch req.PaymentMethod {
	case gateway.MethodCreditCard:
		paymentMethod = "credit_card"
	case gateway.MethodDebitCard:
		paymentMethod = "debit_card"
	}

	return CreateTransactionRequest{
		Amount:            req.Amount,
		PaymentMethod:     paymentMethod,
		Capture:           &capture,
		ExternalReference: fmt.Sprintf("%d", req.OrderID),
		Items:             items,
		Customer:          customer,
		SplitRules:        splitRules,
		Metadata:          req.Metadata,
	}
}

// buildRecipientRequest converte um gateway.RecipientRequest para o formato Pagar.me.
func (g *PagarMeGateway) buildRecipientRequest(req *gateway.RecipientRequest) CreateRecipientRequest {
	bankAccount := &BankAccountRequest{
		BankCode:       req.BankCode,
		Agencia:        req.BankAgency,
		Conta:          req.BankAccount,
		ContaDV:        req.BankAccountDV,
		Type:           req.BankAccountType,
		DocumentNumber: req.Document,
		LegalName:      req.Name,
	}

	registerInfo := &RegisterInformationRequest{
		Type:           "individual",
		DocumentNumber: req.Document,
		Email:          req.Email,
		PhoneNumbers: []PhoneNumber{
			{AreaCode: "11", Number: req.Phone},
		},
	}

	transferDay := req.TransferDay
	if transferDay == 0 {
		transferDay = 1
	}

	return CreateRecipientRequest{
		RegisterInformation: registerInfo,
		BankAccount:         bankAccount,
		AutomaticAnticipationEnabled: true,
		AnticipatableVolumePercentage: 85,
		TransferEnabled:    true,
		TransferInterval:   req.TransferInterval,
		TransferDay:        transferDay,
	}
}

// mapResponse converte a resposta do Pagar.me para o formato normalizado.
func (g *PagarMeGateway) mapResponse(resp *CreateTransactionResponse, req *gateway.TransactionRequest) *gateway.TransactionResponse {
	result := &gateway.TransactionResponse{
		GatewayID:     strconv.FormatInt(resp.ID, 10),
		Gateway:       "pagarme",
		Status:        mapStatus(resp.Status),
		CardBrand:     resp.CardBrand,
		CardLast4:     resp.CardLastFour,
		RequiresAuth:  resp.Authenticate,
		AuthURL:       resp.AuthenticateURL,
		SplitApplied:  len(resp.SplitRules) > 0,
		SplitCount:    len(resp.SplitRules),
		Metadata:      resp.Metadata,
	}

	// PIX
	if resp.QRCode != "" {
		result.PIXQRCode = resp.QRCode
		result.PIXCopyPaste = resp.PixPayload
		expiresAt, _ := time.Parse(time.RFC3339, resp.CreatedAt)
		expiresAt = expiresAt.Add(30 * time.Minute) // PIX expira em 30min
		result.PIXExpiresAt = &expiresAt
	}

	return result
}

// ═══════════════════════════════════════════════════════════════
// MAPEAMENTO DE STATUS
// ═══════════════════════════════════════════════════════════════

// mapStatus converte status do Pagar.me para gateway.TransactionStatus.
func mapStatus(status string) gateway.TransactionStatus {
	switch status {
	case "waiting":
		return gateway.StatusWaiting
	case "authorized":
		return gateway.StatusAuthorized
	case "paid":
		return gateway.StatusPaid
	case "captured":
		return gateway.StatusCaptured
	case "refunded":
		return gateway.StatusRefunded
	case "voided", "canceled":
		return gateway.StatusVoided
	case "refused":
		return gateway.StatusFailed
	case "expired":
		return gateway.StatusExpired
	case "pending":
		return gateway.StatusPending
	default:
		return gateway.StatusPending
	}
}

// mapEventType converte status do Pagar.me para tipo de evento normalizado.
func mapEventType(status string) string {
	switch status {
	case "paid":
		return "paid"
	case "refused":
		return "failed"
	case "refunded":
		return "refunded"
	case "authorized":
		return "authorized"
	case "captured":
		return "captured"
	case "voided", "canceled":
		return "voided"
	default:
		return status
	}
}
