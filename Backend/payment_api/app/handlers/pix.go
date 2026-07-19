package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/vercardapio/app/dto"
	"github.com/carloshomar/vercardapio/app/models"
	"github.com/carloshomar/vercardapio/app/services"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GeneratePIX(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	email := req.CustomerEmail
	if email == "" {
		email = "cliente@email.com"
	}

	name := req.CustomerName
	if name == "" {
		name = "Cliente"
	}

	description := fmt.Sprintf("Pedido %s", req.OrderID)

	mpResp, err := services.CreatePIXPayment(req.Amount, description, email, name)
	if err != nil {
		log.Printf("Error creating PIX payment via MP: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create PIX payment"})
	}

	var qrCodeBase64, copyPaste, ticketURL string
	if mpResp.PointOfInteraction != nil && mpResp.PointOfInteraction.TransactionData != nil {
		qrCodeBase64 = mpResp.PointOfInteraction.TransactionData.QRCodeBase64
		copyPaste = mpResp.PointOfInteraction.TransactionData.CopyPaste
		ticketURL = mpResp.PointOfInteraction.TransactionData.TicketURL
	}

	payment := models.Payment{
		ID:           primitive.NewObjectID(),
		OrderID:      req.OrderID,
		CustomerID:   req.CustomerID,
		EstablishmentID: req.EstablishmentID,
		Amount:       req.Amount,
		Method:       "pix",
		Status:       mpResp.Status,
		PixQRCode:    "",
		PixCopyPaste: copyPaste,
		QRCodeBase64: qrCodeBase64,
		TicketURL:    ticketURL,
		MPPaymentID:  mpResp.ID,
		MPStatus:     mpResp.Status,
		CreatedAt:    time.Now(),
	}

	_, err = models.MongoDabase.Collection("payments").InsertOne(context.Background(), payment)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
	}

	response := dto.PaymentResponse{
		PaymentID:    payment.ID.Hex(),
		Status:       mpResp.Status,
		PixQRCode:    "",
		PixCopyPaste: copyPaste,
		QRCodeBase64: qrCodeBase64,
		TicketURL:    ticketURL,
		MPPaymentID:  mpResp.ID,
		Message:      "PIX payment created via Mercado Pago",
	}

	return c.Status(201).JSON(response)
}
