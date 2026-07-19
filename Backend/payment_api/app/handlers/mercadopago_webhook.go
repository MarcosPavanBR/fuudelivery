package handlers

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/carloshomar/vercardapio/app/services"
	"github.com/gofiber/fiber/v2"
)

func MercadoPagoWebhook(c *fiber.Ctx) error {
	body := c.Body()

	var notification struct {
		Action string `json:"action"`
		API    string `json:"api"`
		Data   struct {
			ID string `json:"id"`
		} `json:"data"`
		Type string `json:"type"`
	}

	if err := json.Unmarshal(body, &notification); err != nil {
		log.Printf("Error parsing MP webhook: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
	}

	log.Printf("Mercado Pago webhook received: action=%s, type=%s, data_id=%s",
		notification.Action, notification.Type, notification.Data.ID)

	if notification.Type == "payment" {
		var paymentID int64
		fmt.Sscanf(notification.Data.ID, "%d", &paymentID)

		paymentStatus, err := services.GetPaymentStatus(paymentID)
		if err != nil {
			log.Printf("Error getting payment status from MP: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to get payment status"})
		}

		updateLocalPaymentStatusByID(paymentID, paymentStatus.Status)

		if paymentStatus.Status == "approved" {
			publishPaymentApproved(paymentID)
		}
	}

	return c.JSON(fiber.Map{"message": "Webhook received"})
}
