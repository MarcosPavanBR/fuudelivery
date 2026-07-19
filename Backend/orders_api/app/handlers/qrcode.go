package handlers

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
)

func GenerateTableQRCode(c *fiber.Ctx) error {
	establishmentID := c.Params("establishmentId")
	tableNumber := c.Query("table", "01")

	baseURL := os.Getenv("APP_URL")
	if baseURL == "" {
		baseURL = "http://localhost:19002"
	}

	qrData := fmt.Sprintf("%s/establishment?table=%s&id=%s", baseURL, tableNumber, establishmentID)

	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost"
	}

	return c.JSON(fiber.Map{
		"qr_data": qrData,
		"table":   tableNumber,
		"url":     fmt.Sprintf("%s/api/order/qrcode/%s/generate?table=%s", apiBaseURL, establishmentID, tableNumber),
	})
}
