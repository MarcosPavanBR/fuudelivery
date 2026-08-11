package handlers

import (
	"log"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ProcessSplit(c *fiber.Ctx) error {
	var req dto.SplitPaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	objID, err := primitive.ObjectIDFromHex(req.PaymentID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payment_id format"})
	}

	var payment models.Payment
	err = models.MongoDabase.Collection("payments").FindOne(mongoCtx(), bson.M{"_id": objID}).Decode(&payment)
	if err != nil {
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

	_, err = models.MongoDabase.Collection("payments").UpdateOne(
		mongoCtx(),
		bson.M{"_id": objID},
		bson.M{
			"$set": bson.M{
				"split_rules": rules,
				"status":      "SPLIT",
			},
		},
	)
	if err != nil {
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
