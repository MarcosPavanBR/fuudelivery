package handlers

import (
	"log"
	"strconv"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// ProcessSplit grava regras de split customizadas num pagamento (Postgres,
// corte 4) com dual-write best-effort no Mongo legado.
//
// payment_id é o ID NUMÉRICO do Postgres desde o corte 4 (antes era o hex
// do ObjectID do Mongo). IDs hex legados são rejeitados com 400.
func ProcessSplit(c *fiber.Ctx) error {
	var req dto.SplitPaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	paymentID, err := strconv.ParseInt(req.PaymentID, 10, 64)
	if err != nil || paymentID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payment_id format"})
	}

	var payment models.Payment
	if err := models.DB.First(&payment, paymentID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Payment not found"})
	}

	rules := req.Rules
	if len(rules) == 0 {
		rules = defaultSplitRules(&payment, 5.0, 85.0)
	}

	var totalSplit float64
	for i, rule := range rules {
		if rule.Amount == 0 && rule.Percentage > 0 {
			rules[i].Amount = (rule.Percentage / 100.0) * payment.Amount
		}
		totalSplit += rules[i].Amount
	}

	if err := models.DB.Model(&payment).Updates(map[string]interface{}{
		"split_rules": models.SplitRules(rules),
		"status":      "SPLIT",
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save split rules"})
	}

	// NOTA: RabbitMQ removido. Notificacao de split e feita via Redis pelo monolito.
	log.Printf("[SPLIT] Split processado: payment=%s order=%s rules=%d", req.PaymentID, payment.OrderID, len(rules))

	return c.Status(200).JSON(fiber.Map{
		"payment_id":  req.PaymentID,
		"status":      "SPLIT",
		"split_rules": rules,
		"total":       payment.Amount,
		"message":     "Payment split processed successfully",
	})
}

// notifySplitToOrderQueue — stub mantido para compatibilidade.
// RabbitMQ foi removido. O monolito gerencia filas via Redis.
func notifySplitToOrderQueue(orderID, paymentID string, rules []models.SplitRule) {
	log.Printf("[SPLIT] Notificacao de split: order=%s payment=%s (RabbitMQ removido, ignora)", orderID, paymentID)
}
