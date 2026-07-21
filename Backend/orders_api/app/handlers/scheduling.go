package handlers

import (
	"time"

	"github.com/carloshomar/vercardapio/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)



type ScheduleRequest struct {
	OrderID     string `json:"order_id"`
	ScheduledAt string `json:"scheduled_at"`
}

func ScheduleOrder(c *fiber.Ctx) error {
	var req ScheduleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	scheduledTime, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid date format"})
	}

	orderID, err := primitive.ObjectIDFromHex(req.OrderID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid order ID"})
	}

	collection := models.MongoDabase.Collection("orders")
	filter := bson.M{"_id": orderID}
	update := bson.M{"$set": bson.M{"scheduled_at": scheduledTime, "is_scheduled": true}}

	_, err = collection.UpdateOne(mongoCtx(), filter, update)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to schedule"})
	}

	return c.JSON(fiber.Map{"message": "Order scheduled", "scheduled_at": scheduledTime})
}
