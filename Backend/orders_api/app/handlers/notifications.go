package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

type PushTicket struct {
	Status string `json:"status"`
	ID     string `json:"id,omitempty"`
}

type RegisterPushTokenRequest struct {
	UserID    int64  `json:"user_id"`
	UserType  string `json:"user_type"`
	PushToken string `json:"push_token"`
}

func RegisterPushToken(c *fiber.Ctx) error {
	var req RegisterPushTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.PushToken == "" {
		return c.Status(400).JSON(fiber.Map{"error": "push_token is required"})
	}

	db := models.DB
	if db == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Database not available"})
	}

	pushToken := models.PushToken{
		UserID:    req.UserID,
		UserType:  req.UserType,
		PushToken: req.PushToken,
	}
	if err := db.Where(models.PushToken{UserID: req.UserID, UserType: req.UserType}).
		Assign(models.PushToken{PushToken: req.PushToken, UpdatedAt: time.Now()}).
		FirstOrCreate(&pushToken).Error; err != nil {
		log.Printf("[PUSH_TOKEN] Falha ao salvar token no Postgres (user=%d): %v", req.UserID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to register token"})
	}

	return c.JSON(fiber.Map{"message": "Token registered"})
}

func SendPushNotification(userID int64, userType, title, body string, data map[string]interface{}) error {
	db := models.DB
	if db == nil {
		return nil
	}

	var pushToken models.PushToken
	if err := db.Where("user_id = ? AND user_type = ?", userID, userType).First(&pushToken).Error; err != nil {
		return nil // token not found, silently skip
	}

	payload := map[string]interface{}{
		"to":    pushToken.PushToken,
		"title": title,
		"body":  body,
		"data":  data,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post("https://exp.host/--/api/v2/push/send", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
