package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"github.com/carloshomar/vercardapio/orders_api/app/models"
)

func RepeatOrder(c *fiber.Ctx) error {
	orderID := c.Params("orderId")

	oid, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid order ID"})
	}

	collection := models.MongoDabase.Collection("orders")
	filter := bson.M{"_id": oid}

	var order bson.M
	if err := collection.FindOne(context.Background(), filter).Decode(&order); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Order not found"})
	}

	response := fiber.Map{
		"cart":          order["cart"],
		"establishment": order["establishment"],
	}

	return c.JSON(response)
}
