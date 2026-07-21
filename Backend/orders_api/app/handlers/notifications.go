package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/carloshomar/vercardapio/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)


func mongoCtx() context.Context {
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
_ = cancel
return ctx
}


type PushTicket struct {
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
}

type PushMessage struct {
	To    string                 `json:"to"`
	Title string                 `json:"title"`
	Body  string                 `json:"body"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

func SendPushNotification(token string, title string, body string, data map[string]interface{}) error {
	message := PushMessage{
		To:    token,
		Title: title,
		Body:  body,
		Data:  data,
	}

	jsonData, _ := json.Marshal(message)

	resp, err := http.Post(
		"https://exp.host/--/api/v2/push/send",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func RegisterPushToken(c *fiber.Ctx) error {
	var req struct {
		UserID    int64  `json:"user_id"`
		UserType  string `json:"user_type"`
		PushToken string `json:"push_token"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	collection := models.MongoDabase.Collection("push_tokens")
	filter := bson.M{"user_id": req.UserID, "user_type": req.UserType}
	update := bson.M{"$set": bson.M{"push_token": req.PushToken, "updated_at": time.Now()}}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(mongoCtx(), filter, update, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to register token"})
	}

	return c.JSON(fiber.Map{"message": "Token registered"})
}
