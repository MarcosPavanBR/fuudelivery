package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/carloshomar/vercardapio/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func randomString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[n.Int64()]
	}
	return string(result)
}

func publishToOrderQueue(body []byte) error {
	dsn := os.Getenv("RABBIT_CONNECTION")
	if dsn == "" {
		panic("RABBIT_CONNECTION nao configurado")
	}

	queueName := os.Getenv("RABBIT_ORDER_QUEUE")
	if queueName == "" {
		panic("RABBIT_ORDER_QUEUE nao configurado")
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

func updateLocalPaymentStatusByID(paymentID int64, status string) {
	filter := bson.M{"mp_payment_id": paymentID}
	update := bson.M{"$set": bson.M{"mp_status": status, "status": status}}
	_, err := models.MongoDabase.Collection("payments").UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Printf("Error updating local payment status for MP ID %d: %v", paymentID, err)
	}
}

func updateLocalPaymentStatus(paymentID string, status string) {
	objID, err := primitive.ObjectIDFromHex(paymentID)
	if err != nil {
		log.Printf("Invalid payment ID format: %s", paymentID)
		return
	}
	now := time.Now()
	updateFields := bson.M{
		"status": status,
	}
	if status == "approved" || status == "CONFIRMED" {
		updateFields["confirmed_at"] = now
	}

	_, err = models.MongoDabase.Collection("payments").UpdateOne(
		context.Background(),
		bson.M{"_id": objID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		log.Printf("Failed to update payment %s: %v", paymentID, err)
	}
}

func publishPaymentApproved(paymentID int64) {
	var payment models.Payment
	err := models.MongoDabase.Collection("payments").FindOne(
		context.Background(),
		bson.M{"mp_payment_id": paymentID},
	).Decode(&payment)
	if err != nil {
		log.Printf("Payment not found for MP ID %d: %v", paymentID, err)
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
		bson.M{"mp_payment_id": paymentID},
		bson.M{"$set": bson.M{
			"status":      "CONFIRMED",
			"split_rules": splitRules,
			"confirmed_at": now,
		}},
	)
	if err != nil {
		log.Printf("Failed to save split rules for MP ID %d: %v", paymentID, err)
	}
}

func HandlePaymentWebhook(c *fiber.Ctx) error {
	var webhookData struct {
		PaymentID string `json:"payment_id"`
		Status    string `json:"status"`
		Message   string `json:"message,omitempty"`
	}

	if err := c.BodyParser(&webhookData); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid webhook payload"})
	}

	if webhookData.PaymentID == "" || webhookData.Status == "" {
		return c.Status(400).JSON(fiber.Map{"error": "payment_id and status are required"})
	}

	objID, err := primitive.ObjectIDFromHex(webhookData.PaymentID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payment_id format"})
	}

	var payment models.Payment
	err = models.MongoDabase.Collection("payments").FindOne(context.Background(), bson.M{"_id": objID}).Decode(&payment)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Payment not found"})
	}

	updateLocalPaymentStatus(webhookData.PaymentID, webhookData.Status)

	if webhookData.Status == "CONFIRMED" || webhookData.Status == "approved" {
		publishPaymentApproved(payment.MPPaymentID)
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
	deliveryAmount := 0.0
	customerCredit := total - platformFee - establishmentAmount - deliveryAmount

	if customerCredit < 0 {
		customerCredit = 0
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
