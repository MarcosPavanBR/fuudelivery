package handlers

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/carloshomar/vercardapio/payment_api/app/models"
	"github.com/carloshomar/vercardapio/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson"
)

func publishToOrderQueue(body []byte) error {
	dsn := os.Getenv("RABBIT_CONNECTION")
	if dsn == "" {
		log.Println("[QUEUE] RabbitMQ não configurado, mensagem ignorada")
		return nil
	}

	queueName := os.Getenv("RABBIT_ORDER_QUEUE")
	if queueName == "" {
		log.Println("[QUEUE] RABBIT_ORDER_QUEUE não configurado, mensagem ignorada")
		return nil
	}

	conn, err := amqp.Dial(dsn)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	err = ch.Publish(
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		return err
	}

	log.Printf("Message published to queue %s: %s", queueName, string(body))
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
		context.Background(),
		bson.M{"abacatepay_id": abacatepayID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		log.Printf("Failed to update payment %s: %v", abacatepayID, err)
	}
}

func publishPaymentApproved(abacatepayID string) {
	var payment models.Payment
	err := models.MongoDabase.Collection("payments").FindOne(
		context.Background(),
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

	publishedAt := now
	payment.Status = "CONFIRMED"
	payment.ConfirmedAt = &publishedAt
	splitRules := defaultSplitRules(&payment)
	payment.SplitRules = splitRules

	_, err = models.MongoDabase.Collection("payments").UpdateOne(
		context.Background(),
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
}

func HandlePaymentWebhook(c *fiber.Ctx) error {
	var webhookData struct {
		Event   string `json:"event"`
		ID      string `json:"id"`
		Charge  struct {
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

func defaultSplitRules(payment *models.Payment) []models.SplitRule {
	total := payment.Amount
	platformFee := total * 0.05
	establishmentAmount := total * 0.85
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
			Percentage:   5.0,
		},
		{
			ReceiverID:   payment.EstablishmentID,
			ReceiverType: "establishment",
			Amount:       establishmentAmount,
			Percentage:   85.0,
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
