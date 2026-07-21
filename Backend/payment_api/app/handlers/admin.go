package handlers

import (
	"context"
	"time"

	"github.com/carloshomar/vercardapio/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
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
