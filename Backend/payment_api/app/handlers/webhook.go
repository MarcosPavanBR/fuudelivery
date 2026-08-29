package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/carloshomar/fuudelivery/pkg/queue"
	"github.com/gofiber/fiber/v2"
)

const (
	// paymentRedisQueueKey e o canal onde o webhook publica confirmacoes
	// de pagamento. Deve coincidir com o canal escutado pelo monolito
	// (cmd/fuudelivery/main.go startQueueListeners) para que a ponte
	// WebSocket notifique o cliente em tempo real.
	paymentRedisQueueKey = "payment_updates"
	orderRedisQueueKey   = "order_updates"
)

// publishToOrderQueue publica uma mensagem na fila de pedidos usando
// o pacote compartilhado pkg/queue (Redis LPush ou Go channels fallback).
func publishToOrderQueue(body []byte) error {
	q := queue.New()
	if err := q.Publish(orderRedisQueueKey, body); err != nil {
		log.Printf("[ORDER_QUEUE] Erro ao publicar: %v", err)
		return err
	}
	log.Printf("[ORDER_QUEUE] Confirmacao de pedido publicada: %s", string(body))
	return nil
}

// publishToPaymentQueue publica uma mensagem na fila de pagamentos usando
// o pacote compartilhado pkg/queue (Redis LPush ou Go channels fallback).
func publishToPaymentQueue(body []byte) error {
	q := queue.New()
	if err := q.Publish(paymentRedisQueueKey, body); err != nil {
		log.Printf("[PAYMENT_QUEUE] Erro ao publicar: %v", err)
		return err
	}
	log.Printf("[PAYMENT_QUEUE] Pagamento publicado: %s", string(body))
	return nil
}

// updateLocalPaymentStatus atualiza status/confirmação do pagamento no
// Postgres (corte 4) com dual-write no Mongo legado.
func updateLocalPaymentStatus(abacatepayID string, status string) {
	now := time.Now()

	payment, err := findPaymentByAbacatePayID(abacatepayID)
	if err != nil {
		log.Printf("Failed to update payment %s: %v", abacatepayID, err)
		return
	}

	updates := map[string]interface{}{"status": status}
	switch status {
	case "paid", "CONFIRMED":
		updates["confirmed_at"] = now
	case "REFUNDED":
		updates["refunded_at"] = now
	}

	if err := models.DB.Model(payment).Updates(updates).Error; err != nil {
		log.Printf("Failed to update payment %s: %v", abacatepayID, err)
		return
	}
}

// establishmentShare soma o valor destinado ao estabelecimento nas split
// rules (receiver_type == "establishment"). É o crédito que precisa ser
// revertido na carteira quando o pagamento é estornado.
//
// IMPORTANTE: recebe as REGRAS, não o pagamento — no fluxo de confirmação as
// rules são calculadas ali mesmo e ainda não estão persistidas em
// payment.SplitRules (passar o pagamento zeraria o share e o estabelecimento
// nunca seria creditado — bug pego pelo teste E2E de cashback).
func establishmentShare(rules models.SplitRules) float64 {
	var share float64
	for _, rule := range rules {
		if rule.ReceiverType == "establishment" {
			share += rule.Amount
		}
	}
	return share
}

// customerCashbackShare soma o valor destinado ao cliente nas split rules do
// pagamento (receiver_type == "customer") — o cashback creditado na carteira
// do cliente quando o pagamento é confirmado. Também precisa ser revertido no
// estorno, além do crédito do estabelecimento.
func customerCashbackShare(rules models.SplitRules) float64 {
	var share float64
	for _, rule := range rules {
		if rule.ReceiverType == "customer" {
			share += rule.Amount
		}
	}
	return share
}

// reverseWalletCredit debita um valor da carteira do usuário de forma
// atômica via AdjustWalletBalance (transação + SELECT FOR UPDATE + guarda de
// saldo). Retorna true se o débito foi realmente aplicado. Usado pelo
// chargeback para reverter os créditos de split (estabelecimento e cashback
// do cliente) e o top-up de carteira.
//
// Nota: nunca deixa saldo negativo; se a carteira não existe ou o saldo é
// insuficiente, o débito é recusado e logado.
func reverseWalletCredit(userID int64, amount float64, abacatepayID, description string, now time.Time) bool {
	if amount <= 0 {
		return false
	}

	wallet, err := ensureWalletSeeded(models.DB, userID, walletTypeForUser(userID))
	if err != nil {
		log.Printf("[REFUND] Carteira do usuário %d NAO debitada em %.2f (falha ao carregar carteira): %v", userID, amount, err)
		return false
	}

	_, dErr := models.AdjustWalletBalance(models.DB, userID, wallet.UserType, "debit", "", amount, abacatepayID, description, "")
	if dErr != nil {
		log.Printf("[REFUND] Carteira do usuário %d NAO debitada em %.2f (%v)", userID, amount, dErr)
		return false
	}
	log.Printf("[REFUND] Carteira do usuário %d debitada em %.2f (payment=%s)", userID, amount, abacatepayID)
	return true
}

// processPaymentRefund trata o estorno/chargeback de um pagamento:
//  1. Reverte os créditos de carteira (somente se o pagamento estava
//     CONFIRMED — só assim houve crédito a reverter):
//     a. o crédito do estabelecimento (share do split);
//     b. o crédito de cashback do cliente (receiver_type == "customer");
//     c. o top-up de carteira, quando o pagamento foi usado pelo cliente
//     (wallet_credited_at preenchido);
//  2. Publica o evento PAYMENT_REFUNDED nas filas order_updates/payment_updates
//     para o monolito notificar o cliente em tempo real;
//  3. Marca o pagamento como REFUNDED + refunded_at.
//
// Idempotente: um webhook reprocessado não debita duas vezes (a segunda
// chamada encontra o status já REFUNDED e pula a reversão).
func processPaymentRefund(abacatepayID string) {
	payment, err := findPaymentByAbacatePayID(abacatepayID)
	if err != nil {
		log.Printf("[REFUND] Payment not found for AbacatePay ID %s: %v", abacatepayID, err)
		return
	}

	now := time.Now()

	// Reversões de carteira — somente pagamentos CONFIRMED tiveram
	// split/crédito de carteira ou top-up. Pagamentos PENDING/EXPIRED não têm
	// nada a reverter.
	if payment.Status == "CONFIRMED" {
		// 1. Crédito do estabelecimento (share do split)
		reverseWalletCredit(
			payment.EstablishmentID,
			establishmentShare(payment.SplitRules),
			abacatepayID,
			"Refund/chargeback: estorno do pagamento "+payment.OrderID,
			now,
		)

		// 2. Crédito de cashback do cliente (receiver_type == "customer")
		reverseWalletCredit(
			payment.CustomerID,
			customerCashbackShare(payment.SplitRules),
			abacatepayID,
			"Refund/chargeback: estorno do cashback do pagamento "+payment.OrderID,
			now,
		)

		// 3. Top-up de carteira quando o pagamento foi usado pelo cliente
		// (o crédito do top-up é o valor total do pagamento).
		if payment.WalletCreditedAt != nil {
			reverseWalletCredit(
				payment.CustomerID,
				payment.Amount,
				abacatepayID,
				"Refund/chargeback: reversão do top-up do pagamento "+payment.OrderID,
				now,
			)
		}

		// Notifica o cliente em tempo real (ponte WebSocket do monolito)
		orderMsg := map[string]interface{}{
			"type":        "PAYMENT_REFUNDED",
			"order_id":    payment.OrderID,
			"payment_id":  payment.IDString(),
			"user_id":     payment.CustomerID,
			"status":      "REFUNDED",
			"amount":      payment.Amount,
			"method":      payment.Method,
			"refunded_at": now.Format(time.RFC3339),
		}
		if msgBody, mErr := json.Marshal(orderMsg); mErr == nil {
			if pErr := publishToOrderQueue(msgBody); pErr != nil {
				log.Printf("[REFUND] Falha ao publicar estorno na fila de pedidos: %v", pErr)
			}
		}

		paymentMsg := map[string]interface{}{
			"order_id":         payment.OrderID,
			"establishment_id": payment.EstablishmentID,
			"user_id":          payment.CustomerID,
			"amount":           payment.Amount,
			"delivery_amount":  payment.DeliveryAmount,
			"status":           "refunded",
		}
		if msgBody, mErr := json.Marshal(paymentMsg); mErr == nil {
			if pErr := publishToPaymentQueue(msgBody); pErr != nil {
				log.Printf("[REFUND] Falha ao publicar estorno na fila de pagamentos: %v", pErr)
			}
		}
	}

	payment.Status = "REFUNDED"
	payment.RefundedAt = &now
	if err := models.DB.Model(payment).Updates(map[string]interface{}{
		"status":      "REFUNDED",
		"refunded_at": now,
	}).Error; err != nil {
		log.Printf("[REFUND] Falha ao marcar payment %s como REFUNDED: %v", abacatepayID, err)
		return
	}
}

// SplitConfigResolver e chamado para obter os percentuais de split
// de um estabelecimento com base na zona/praca a que ele pertence.
// Retorna (platformFeePercent, establishmentPercent).
// Se nao configurado, usa o padrao 5/85.
type SplitConfigResolver func(establishmentID int64) (platformPct, establishmentPct float64)

// GetSplitConfigForEstablishment e um callback que pode ser definido
// pelo monólito (cmd/fuudelivery/main.go) para buscar a configuracao
// de split da zona do estabelecimento no PostgreSQL.
var GetSplitConfigForEstablishment SplitConfigResolver

var OnPaymentApproved func(customerPhone, orderID string, orderValue float64) error

// publishPaymentApproved confirma o pagamento: calcula e grava as regras de
// split, credita a carteira do estabelecimento (idempotente via ledger),
// publica eventos nas filas e marca confirmed_at. Tudo em Postgres (corte 4)
// com dual-write best-effort no Mongo.
func publishPaymentApproved(abacatepayID string) {
	payment, err := findPaymentByAbacatePayID(abacatepayID)
	if err != nil {
		log.Printf("Payment not found for AbacatePay ID %s: %v", abacatepayID, err)
		return
	}

	now := time.Now()
	orderMsg := map[string]interface{}{
		"order_id":     payment.OrderID,
		"payment_id":   payment.IDString(),
		"status":       "PAYMENT_CONFIRMED",
		"amount":       payment.Amount,
		"method":       payment.Method,
		"confirmed_at": now.Format(time.RFC3339),
	}

	msgBody, _ := json.Marshal(orderMsg)
	if err := publishToOrderQueue(msgBody); err != nil {
		log.Printf("Failed to publish payment confirmation to order queue: %v", err)
	}

	// Publica na fila de pagamentos para que a carteira do restaurante
	// seja creditada pelo fluxo de split abaixo.
	paymentMsg := map[string]interface{}{
		"order_id":         payment.OrderID,
		"establishment_id": payment.EstablishmentID,
		"amount":           payment.Amount,
		"delivery_amount":  payment.DeliveryAmount,
		"status":           "approved",
	}
	paymentMsgBody, _ := json.Marshal(paymentMsg)
	if err := publishToPaymentQueue(paymentMsgBody); err != nil {
		log.Printf("Failed to publish to payment queue: %v", err)
	}

	// Determina os percentuais de split com base na zona do estabelecimento
	platformPct, establishmentPct := 5.0, 85.0
	if GetSplitConfigForEstablishment != nil {
		platformPct, establishmentPct = GetSplitConfigForEstablishment(payment.EstablishmentID)
	}

	splitResult, err := services.CalculateSplitRules(payment, platformPct, establishmentPct)
	if err != nil {
		log.Printf("[SPLIT] Erro ao calcular split para %s: %v", abacatepayID, err)
		log.Printf("[SPLIT] Erro ao calcular split: %v", err)
		return
	}
	splitRules := splitResult.Rules

	setFields := map[string]interface{}{
		"status":       "CONFIRMED",
		"split_rules":  models.SplitRules(splitRules),
		"confirmed_at": now,
	}

	// Credita a carteira do estabelecimento pelo share do split. A idempotência
	// é estrutural: o UNIQUE uq_wallet_txns_credit_ref (sql/11) faz o segundo
	// crédito para o mesmo pagamento retornar ErrDuplicateCredit — reprocessar
	// o webhook não credita de novo, mesmo com entregas concorrentes.
	// (NÃO usar payment.Status == "PENDING" como guard: o webhook chama
	// updateLocalPaymentStatus ANTES de publishPaymentApproved, então o
	// pagamento já está CONFIRMED aqui e o guard nunca dispararia.)
	if payment.EstablishmentCreditedAt == nil && payment.Status != "REFUNDED" {
		// Usa as regras recém-calculadas (ainda não persistidas em payment).
		credit := establishmentShare(models.SplitRules(splitRules))
		if credit > 0 {
			_, wErr := adjustEstablishmentWallet(payment.EstablishmentID, credit, abacatepayID, payment.OrderID, now)
			switch {
			case errors.Is(wErr, models.ErrDuplicateCredit):
				log.Printf("[WALLET] Crédito já aplicado para %s — replay idempotente ignorado", abacatepayID)
				setFields["establishment_credited_at"] = now
			case wErr != nil:
				log.Printf("[WALLET] WARNING: falha ao creditar carteira do estabelecimento %d: %v", payment.EstablishmentID, wErr)
			default:
				setFields["establishment_credited_at"] = now
				log.Printf("[WALLET] Carteira do estabelecimento %d creditada em %.2f (payment=%s)", payment.EstablishmentID, credit, abacatepayID)
			}
		}
	}

	if err := models.DB.Model(payment).Updates(setFields).Error; err != nil {
		log.Printf("Failed to save split rules for AbacatePay ID %s: %v", abacatepayID, err)
		return
	}
	if OnPaymentApproved != nil {
		if err := OnPaymentApproved(payment.CustomerPhone, payment.OrderID, payment.Amount); err != nil {
			log.Printf("[LOYALTY] Failed to award points for order %s: %v", payment.OrderID, err)
		}
	}
}

// adjustEstablishmentWallet credita a carteira do estabelecimento de forma
// atômica (AdjustWalletBalance), semeando antes o saldo legado do Mongo se
// for a primeira movimentação da carteira pós-corte.
func adjustEstablishmentWallet(establishmentID int64, credit float64, abacatepayID, orderID string, now time.Time) (*models.Wallet, error) {
	if _, err := ensureWalletSeeded(models.DB, establishmentID, "establishment"); err != nil {
		return nil, err
	}
	return models.AdjustWalletBalance(
		models.DB,
		establishmentID,
		"establishment",
		"credit",
		"",
		credit,
		abacatepayID,
		"Credito do split do pedido "+orderID,
		"",
	)
}

// ValidateWebhookSignature verifica a assinatura HMAC-SHA256 do header
// x-abacatepay-signature contra o body do webhook usando a secret configurada.
// Retorna true se a assinatura for válida. Em produção (GO_ENV=production),
// secret ausente = REJEITA (fail-closed): sem isso, um erro de configuração
// desligaria a defesa em profundidade silenciosamente. O handler continua
// revalidando o status contra a API da AbacatePay de qualquer forma.
func ValidateWebhookSignature(body []byte, signature string) bool {
	secret := os.Getenv("ABACATE_PAY_WEBHOOK_SECRET")
	if secret == "" {
		if os.Getenv("GO_ENV") == "production" {
			log.Println("[WEBHOOK] ABACATE_PAY_WEBHOOK_SECRET ausente em produção — rejeitando (fail-closed)")
			return false
		}
		// Fora de produção, permite rodar sem secret para desenvolvimento local.
		log.Println("[WEBHOOK] ABACATE_PAY_WEBHOOK_SECRET not set — skipping HMAC validation (dev)")
		return true
	}

	if signature == "" {
		log.Println("[WEBHOOK] Missing x-abacatepay-signature header")
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		log.Println("[WEBHOOK] HMAC signature mismatch")
		return false
	}
	return true
}

func HandlePaymentWebhook(c *fiber.Ctx) error {
	body := c.Body()

	// ═══ DETECÇÃO MULTI-GATEWAY ═══
	// O gateway é identificado pelos headers da requisição.
	// Cada gateway usa um header de assinatura diferente.
	gatewayName := detectGatewayFromHeaders(c)

	if gatewayName != "" && services.IsGatewayEnabled() {
		// Caminho novo: usar o adapter multi-gateway
		return handleGatewayWebhook(c, body, gatewayName)
	}

	// ═══ CAMINHO LEGADO: AbacatePay ═══
	signature := c.Get("x-abacatepay-signature")
	if !ValidateWebhookSignature(body, signature) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid webhook signature"})
	}

	var webhookData struct {
		Event  string `json:"event"`
		ID     string `json:"id"`
		Charge struct {
			ID     string  `json:"id"`
			Status string  `json:"status"`
			Amount float64 `json:"amount"`
		} `json:"charge"`
	}

	if err := c.BodyParser(&webhookData); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid webhook payload"})
	}

	if webhookData.Event == "" {
		return c.Status(400).JSON(fiber.Map{"error": "event is required"})
	}

	chargeID := webhookData.Charge.ID
	if chargeID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "charge.id is required"})
	}

	// Verify charge status with AbacatePay API (don't trust webhook body)
	client := services.NewAbacatePayClient()
	apiCharge, err := client.GetCharge(chargeID)
	if err != nil {
		log.Printf("Failed to verify charge %s with AbacatePay: %v", chargeID, err)
		return c.Status(502).JSON(fiber.Map{"error": "Failed to verify charge"})
	}

	apiStatus, _ := apiCharge["status"].(string)
	abacatepayStatus := ""
	switch apiStatus {
	// API v2 usa status em maiúsculas: PAID, APPROVED, EXPIRED, REFUNDED, CANCELLED
	case "paid", "PAID", "CONFIRMED", "APPROVED":
		abacatepayStatus = "CONFIRMED"
	case "expired", "EXPIRED":
		abacatepayStatus = "EXPIRED"
	case "refunded", "REFUNDED":
		abacatepayStatus = "REFUNDED"
	case "cancelled", "CANCELLED":
		abacatepayStatus = "CANCELLED"
	default:
		abacatepayStatus = apiStatus
	}

	if abacatepayStatus == "REFUNDED" {
		// Estorno/chargeback: reverte créditos da carteira, notifica via fila
		// e marca o pagamento como REFUNDED.
		processPaymentRefund(chargeID)
	} else {
		updateLocalPaymentStatus(chargeID, abacatepayStatus)
	}

	if abacatepayStatus == "CONFIRMED" {
		publishPaymentApproved(chargeID)
	}

	return c.Status(200).JSON(fiber.Map{
		"status":  "processed",
		"message": "Webhook processed successfully",
	})
}

// defaultSplitRules calcula as regras de split de pagamento com base
// nos percentuais configurados para a zona do estabelecimento.
// Se platformPct + establishmentPct nao somarem 100%, o excedente
// vai para customerCredit (cashback).
func defaultSplitRules(payment *models.Payment, platformPct, establishmentPct float64) []models.SplitRule {
	total := payment.Amount
	platformFee := total * (platformPct / 100.0)
	establishmentAmount := total * (establishmentPct / 100.0)
	deliveryAmount := payment.DeliveryAmount
	customerCredit := total - platformFee - establishmentAmount - deliveryAmount

	if customerCredit < 0 {
		overage := -customerCredit
		customerCredit = 0
		establishmentAmount -= overage
		if establishmentAmount < 0 {
			overage = -establishmentAmount
			establishmentAmount = 0
			platformFee -= overage
			if platformFee < 0 {
				platformFee = 0
			}
		}
		log.Printf("[SPLIT] Warning: deliveryAmount=%.2f exceeds available%%, adjusted establishment=%.2f platform=%.2f", deliveryAmount, establishmentAmount, platformFee)
	}

	rules := []models.SplitRule{
		{
			ReceiverID:   0,
			ReceiverType: "platform",
			Amount:       platformFee,
			Percentage:   platformPct,
		},
		{
			ReceiverID:   payment.EstablishmentID,
			ReceiverType: "establishment",
			Amount:       establishmentAmount,
			Percentage:   establishmentPct,
		},
	}

	if deliveryAmount > 0 {
		rules = append(rules, models.SplitRule{
			ReceiverID:   0,
			ReceiverType: "deliveryman",
			Amount:       deliveryAmount,
			Percentage:   0,
		})
	}

	if customerCredit > 0 {
		rules = append(rules, models.SplitRule{
			ReceiverID:   payment.CustomerID,
			ReceiverType: "customer",
			Amount:       customerCredit,
			Percentage:   0,
		})
	}

	return rules
}

// ═══════════════════════════════════════════════════════════════
// MULTI-GATEWAY WEBHOOK HANDLERS
// ═══════════════════════════════════════════════════════════════

// detectGatewayFromHeaders identifica o gateway pelos headers HTTP.
// Cada gateway usa um header de assinatura diferente:
//   - AbacatePay: x-abacatepay-signature
//   - Pagar.me:   x-pagarme-signature
//   - Asaas:      access-token
//   - MercadoPago: x-signature
func detectGatewayFromHeaders(c *fiber.Ctx) string {
	if c.Get("x-pagarme-signature") != "" {
		return "pagarme"
	}
	if c.Get("access-token") != "" {
		return "asaas"
	}
	if c.Get("x-signature") != "" {
		return "mercadopago"
	}
	if c.Get("x-abacatepay-signature") != "" {
		return "abacatepay"
	}
	return ""
}

// handleGatewayWebhook processa webhooks de qualquer gateway registrado.
// O fluxo é:
//  1. Validar assinatura via gateway adapter
//  2. Parsear payload para formato unificado WebhookEvent
//  3. Buscar pagamento pelo gateway_transaction_id
//  4. Processar conforme o status (confirm, refund, etc)
func handleGatewayWebhook(c *fiber.Ctx, body []byte, gatewayName string) error {
	log.Printf("[WEBHOOK] Recebido de gateway: %s", gatewayName)

	// 1. Validar assinatura via gateway adapter
	headers := map[string]string{
		"x-pagarme-signature":    c.Get("x-pagarme-signature"),
		"access-token":           c.Get("access-token"),
		"x-signature":            c.Get("x-signature"),
		"x-abacatepay-signature": c.Get("x-abacatepay-signature"),
	}

	validatedName, valid := services.ValidateWebhookByGateway(body, headers)
	if !valid {
		log.Printf("[WEBHOOK] Assinatura inválida para gateway %s", gatewayName)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid webhook signature"})
	}
	if validatedName != "" {
		gatewayName = validatedName
	}

	// 2. Parsear payload para formato unificado
	event, err := services.ParseGatewayWebhook(c.Context(), gatewayName, body)
	if err != nil {
		log.Printf("[WEBHOOK] Erro ao parsear webhook %s: %v", gatewayName, err)
		return c.Status(400).JSON(fiber.Map{"error": "Failed to parse webhook"})
	}

	log.Printf("[WEBHOOK] Evento: gateway=%s type=%s txID=%s status=%s",
		event.Gateway, event.EventType, event.TransactionID, event.Status)

	// 3. Processar conforme o status
	internalStatus := services.MapGatewayStatus(string(event.Status))

	// Buscar pagamento pelo transaction ID do gateway
	payment, err := findPaymentByGatewayTransactionID(gatewayName, event.TransactionID)
	if err != nil {
		// Fallback: buscar pelo abacatepay_id (legado)
		payment, err = findPaymentByAbacatePayID(event.TransactionID)
		if err != nil {
			log.Printf("[WEBHOOK] Pagamento não encontrado: gateway=%s txID=%s: %v",
				gatewayName, event.TransactionID, err)
			return c.Status(200).JSON(fiber.Map{"status": "ignored", "message": "Payment not found"})
		}
	}

	// 4. Processar conforme o status
	switch event.Status {
	case gateway.StatusPaid, gateway.StatusCaptured:
		// Confirmar pagamento + split + credito carteira
		if err := confirmGatewayPayment(payment, event); err != nil {
			log.Printf("[WEBHOOK] Erro ao confirmar pagamento %d: %v", payment.ID, err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to confirm payment"})
		}

	case gateway.StatusRefunded, gateway.StatusChargeback:
		// Estorno/chargeback
		processPaymentRefund(event.TransactionID)

	case gateway.StatusFailed, gateway.StatusExpired, gateway.StatusVoided:
		updateLocalPaymentStatus(event.TransactionID, internalStatus)

	default:
		log.Printf("[WEBHOOK] Status ignorado: %s", event.Status)
	}

	return c.Status(200).JSON(fiber.Map{
		"status":  "processed",
		"gateway": gatewayName,
		"message": "Webhook processed successfully",
	})
}

// findPaymentByGatewayTransactionID busca pagamento pelo gateway + txID.
func findPaymentByGatewayTransactionID(gatewayName, txID string) (*models.Payment, error) {
	var payment models.Payment
	err := models.DB.Where("gateway = ? AND gateway_transaction_id = ?",
		gatewayName, txID).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// confirmGatewayPayment marca pagamento como CONFIRMED e processa split.
func confirmGatewayPayment(payment *models.Payment, event *gateway.WebhookEvent) error {
	now := time.Now()

	// Atualizar status
	updates := map[string]interface{}{
		"status":       "CONFIRMED",
		"confirmed_at": now,
		"captured_at":  now,
	}
	if err := models.DB.Model(payment).Updates(updates).Error; err != nil {
		return fmt.Errorf("falha ao atualizar pagamento: %w", err)
	}

	// Processar split se habilitado
	if os.Getenv("PAYMENT_SPLIT_ENABLED") == "true" {
		rules := defaultSplitRules(payment, 5.0, 85.0)
		if err := models.DB.Model(payment).Updates(map[string]interface{}{
			"split_rules": models.SplitRules(rules),
			"status":      "SPLIT",
		}).Error; err != nil {
			log.Printf("[WEBHOOK] Falha ao salvar split rules: %v", err)
		}

		// Creditar carteira do estabelecimento
		credito := establishmentShare(rules)
		if credito > 0 {
			log.Printf("[WEBHOOK] Credito estabelecimento %d: R$%.2f (payment=%d)",
				payment.EstablishmentID, credito, payment.ID)
		}
	}

	// Notificar pagamento confirmado
	publishPaymentApproved(event.TransactionID)

	log.Printf("[WEBHOOK] Pagamento %d confirmado via %s (tx=%s)",
		payment.ID, event.Gateway, event.TransactionID)
	return nil
}
