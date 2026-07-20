package handlers

import (
	"context"

	"github.com/carloshomar/vercardapio/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ListAllPayments(c *fiber.Ctx) error {
	collection := models.MongoDabase.Collection("payments")

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(500)
	cursor, err := collection.Find(context.Background(), bson.M{}, opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao buscar pagamentos"})
	}
	defer cursor.Close(context.Background())

	var payments []map[string]interface{}
	if err := cursor.All(context.Background(), &payments); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao decodificar resultados"})
	}

	if payments == nil {
		payments = []map[string]interface{}{}
	}

	return c.JSON(payments)
}
