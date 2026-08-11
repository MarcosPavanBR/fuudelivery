package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/queue"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
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

func updateLocalPaymentStatus(abacatepayID string, status string) {
	now := time.Now()
	updateFields := bson.M{
		"status": status,
	}
	if status == "paid" || status == "CONFIRMED" {
		updateFields["confirmed_at"] = now
	}

	_, err := models.MongoDabase.Collection("payments").UpdateOne(
		mongoCtx(),
		bson.M{"abacatepay_id": abacatepayID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		log.Printf("Failed to update payment %s: %v", abacatepayID, err)
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

func publishPaymentApproved(abacatepayID string) {
	var payment models.Payment
	err := models.MongoDabase.Collection("payments").FindOne(
		mongoCtx(),
		bson.M{"abacatepay_id": abacatepayID},
	).Decode(&payment)
	if err != nil {
		log.Printf("Payment not found for AbacatePay ID %s: %v", abacatepayID, err)
		return
	}

	now := time.Now()
	orderMsg := map[string]interface{}{
		"order_id":     payment.OrderID,
		"payment_id":   payment.ID.Hex(),
		"status":       "PAYMENT_CONFIRMED",
		"amount":       payment.Amount,
		"method":       payment.Method,
		"confirmed_at": now.Format(time.RFC3339),
	}

	msgBody, _ := json.Marshal(orderMsg)
	if err := publishToOrderQueue(msgBody); err != nil {
		log.Printf("Failed to publish payment confirmation to order queue: %v", err)
	}

	// Publica na fila de pagamentos do Payment Service (microsserviço)
	// para que ele credite o valor na carteira do restaurante
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

	publishedAt := now
	payment.Status = "CONFIRMED"
	payment.ConfirmedAt = &publishedAt
	// Determina os percentuais de split com base na zona do estabelecimento
	platformPct, establishmentPct := 5.0, 85.0
	if GetSplitConfigForEstablishment != nil {
		platformPct, establishmentPct = GetSplitConfigForEstablishment(payment.EstablishmentID)
	}

	splitRules := defaultSplitRules(&payment, platformPct, establishmentPct)
	payment.SplitRules = splitRules

	_, err = models.MongoDabase.Collection("payments").UpdateOne(
		mongoCtx(),
		bson.M{"abacatepay_id": abacatepayID},
		bson.M{"$set": bson.M{
			"status":       "CONFIRMED",
			"split_rules":  splitRules,
			"confirmed_at": now,
		}},
	)
	if err != nil {
		log.Printf("Failed to save split rules for AbacatePay ID %s: %v", abacatepayID, err)
	}

	if OnPaymentApproved != nil {
		if err := OnPaymentApproved(payment.CustomerPhone, payment.OrderID, payment.Amount); err != nil {
			log.Printf("[LOYALTY] Failed to award points for order %s: %v", payment.OrderID, err)
		}
	}
}

// ValidateWebhookSignature verifica a assinatura HMAC-SHA256 do header
// x-abacatepay-signature contra o body do webhook usando a secret configurada.
// Retorna true se a assinatura for válida ou se a secret não estiver configurada
// (fallback: a verificação via API AbacatePay continua sendo o check primário).
func ValidateWebhookSignature(body []byte, signature string) bool {
	secret := os.Getenv("ABACATE_PAY_WEBHOOK_SECRET")
	if secret == "" {
		// Secret não configurada —skip HMAC (a verificação via API é o check primário)
		log.Println("[WEBHOOK] ABACATE_PAY_WEBHOOK_SECRET not set — skipping HMAC validation")
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
		log.Printf("[WEBHOOK] HMAC mismatch: expected=%s got=%s", expected[:16]+"...", signature[:min(16, len(signature))]+"...")
		return false
	}
	return true
}

func HandlePaymentWebhook(c *fiber.Ctx) error {
	// --- HMAC signature validation (defense-in-depth) ---
	body := c.Body()
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
	case "paid", "CONFIRMED":
		abacatepayStatus = "CONFIRMED"
	case "expired":
		abacatepayStatus = "EXPIRED"
	case "refunded":
		abacatepayStatus = "REFUNDED"
	case "cancelled":
		abacatepayStatus = "CANCELLED"
	default:
		abacatepayStatus = apiStatus
	}

	updateLocalPaymentStatus(chargeID, abacatepayStatus)

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
