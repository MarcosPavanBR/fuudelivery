package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func mongoCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = cancel
	return ctx
}

func ListAllPayments(c *fiber.Ctx) error {
	collection := models.MongoDabase.Collection("payments")

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(500)
	cursor, err := collection.Find(mongoCtx(), bson.M{}, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao buscar pagamentos"})
	}
	defer cursor.Close(mongoCtx())

	var payments []map[string]interface{}
	if err := cursor.All(mongoCtx(), &payments); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao decodificar resultados"})
	}

	if payments == nil {
		payments = []map[string]interface{}{}
	}

	return c.JSON(payments)
}

// GetPaymentStats retorna estatísticas agregadas dos pagamentos (admin).
// GET /payments/stats
func GetPaymentStats(c *fiber.Ctx) error {
	collection := models.MongoDabase.Collection("payments")

	total, _ := collection.CountDocuments(mongoCtx(), bson.M{})
	pending, _ := collection.CountDocuments(mongoCtx(), bson.M{"status": "PENDING"})
	confirmed, _ := collection.CountDocuments(mongoCtx(), bson.M{"status": "CONFIRMED"})
	rejected, _ := collection.CountDocuments(mongoCtx(), bson.M{"status": "REJECTED"})

	// Soma de valores (armazenados em centavos)
	var totalAmount float64
	cursor, err := collection.Find(mongoCtx(), bson.M{})
	if err == nil {
		defer cursor.Close(mongoCtx())
		for cursor.Next(mongoCtx()) {
			var p struct {
				Amount float64 `bson:"amount"`
			}
			if err := cursor.Decode(&p); err == nil {
				totalAmount += p.Amount
			}
		}
	}

	return c.JSON(fiber.Map{
		"total":        total,
		"total_amount": totalAmount,
		"pending":      pending,
		"confirmed":    confirmed,
		"rejected":     rejected,
		"status_counts": fiber.Map{
			"PENDING":   pending,
			"CONFIRMED": confirmed,
			"REJECTED":  rejected,
		},
	})
}

// ApprovePayment aprova manualmente um pagamento pendente (admin).
// POST /payments/:id/approve
func ApprovePayment(c *fiber.Ctx) error {
	id := c.Params("id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payment ID"})
	}

	var payment models.Payment
	err = models.MongoDabase.Collection("payments").FindOne(mongoCtx(), bson.M{"_id": objID}).Decode(&payment)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payment not found"})
	}

	if payment.Status != "PENDING" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Payment is not pending", "status": payment.Status})
	}

	adminID := ""
	if uid, err := middlewares.GetUserIDFromToken(c); err == nil {
		adminID = fmt.Sprintf("%d", uid)
	}

	now := time.Now()
	_, err = models.MongoDabase.Collection("payments").UpdateOne(
		mongoCtx(),
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"status":       "CONFIRMED",
			"confirmed_at": now,
			"approved_by":  adminID,
		}},
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to approve payment"})
	}

	return c.JSON(fiber.Map{"message": "Payment approved", "payment_id": id, "status": "CONFIRMED"})
}

// RejectPayment rejeita manualmente um pagamento pendente (admin).
// POST /payments/:id/reject  body: {"reason": "..."}
func RejectPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payment ID"})
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var payment models.Payment
	err = models.MongoDabase.Collection("payments").FindOne(mongoCtx(), bson.M{"_id": objID}).Decode(&payment)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Payment not found"})
	}

	if payment.Status != "PENDING" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Payment is not pending", "status": payment.Status})
	}

	adminID := ""
	if uid, err := middlewares.GetUserIDFromToken(c); err == nil {
		adminID = fmt.Sprintf("%d", uid)
	}

	now := time.Now()
	_, err = models.MongoDabase.Collection("payments").UpdateOne(
		mongoCtx(),
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"status":           "REJECTED",
			"rejected_at":      now,
			"rejected_by":      adminID,
			"rejection_reason": body.Reason,
		}},
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to reject payment"})
	}

	return c.JSON(fiber.Map{"message": "Payment rejected", "payment_id": id, "status": "REJECTED"})
}

// ListWallets lista todas as carteiras (admin).
// GET /wallets
func ListWallets(c *fiber.Ctx) error {
	collection := models.MongoDabase.Collection("wallets")

	opts := options.Find().SetSort(bson.D{{Key: "last_updated", Value: -1}}).SetLimit(500)
	cursor, err := collection.Find(mongoCtx(), bson.M{}, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao listar carteiras"})
	}
	defer cursor.Close(mongoCtx())

	var wallets []map[string]interface{}
	if err := cursor.All(mongoCtx(), &wallets); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao decodificar carteiras"})
	}

	// Enriquece com owner_type derivado de user_type para o painel Financeiro
	for i, w := range wallets {
		if t, ok := w["user_type"].(string); ok {
			wallets[i]["owner_type"] = t
		}
	}

	if wallets == nil {
		wallets = []map[string]interface{}{}
	}

	return c.JSON(wallets)
}
