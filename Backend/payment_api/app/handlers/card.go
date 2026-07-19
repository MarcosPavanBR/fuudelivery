package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/carloshomar/vercardapio/app/dto"
	"github.com/carloshomar/vercardapio/app/models"
	"github.com/carloshomar/vercardapio/app/services"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func randomDigits(length int) string {
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		result[i] = byte('0') + byte(n.Int64())
	}
	return string(result)
}

func TokenizeCard(c *fiber.Ctx) error {
	var req dto.CardTokenizeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if len(req.CardNumber) < 13 || len(req.CardNumber) > 19 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid card number"})
	}

	lastDigits := req.CardNumber[len(req.CardNumber)-4:]
	token := fmt.Sprintf("ct_%s_%s", randomDigits(16), randomString(8))

	return c.Status(200).JSON(fiber.Map{
		"card_token":   token,
		"last_digits":  lastDigits,
		"brand":        detectCardBrand(req.CardNumber),
		"exp_month":    req.ExpMonth,
		"exp_year":     req.ExpYear,
		"message":      "Card tokenized successfully",
	})
}

func detectCardBrand(cardNumber string) string {
	if len(cardNumber) == 0 {
		return "unknown"
	}
	switch cardNumber[0] {
	case '4':
		return "visa"
	case '5':
		return "mastercard"
	case '3':
		return "amex"
	case '6':
		return "discover"
	default:
		return "unknown"
	}
}

func ChargeCard(c *fiber.Ctx) error {
	var req struct {
		CardToken     string  `json:"card_token"`
		Amount        float64 `json:"amount"`
		Installments  int     `json:"installments"`
		Email         string  `json:"email"`
		PaymentMethodID string `json:"payment_method_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.CardToken == "" {
		return c.Status(400).JSON(fiber.Map{"error": "card_token is required"})
	}

	email := req.Email
	if email == "" {
		email = "cliente@email.com"
	}

	paymentMethodID := req.PaymentMethodID
	if paymentMethodID == "" {
		paymentMethodID = "visa"
	}
	installments := req.Installments
	if installments <= 0 {
		installments = 1
	}

	mpResp, err := services.CreateCardPayment(req.Amount, "Pagamento cartao", req.CardToken, email, installments, paymentMethodID)
	if err != nil {
		log.Printf("Error creating card payment via MP: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Card payment failed"})
	}

	return c.Status(200).JSON(fiber.Map{
		"charge_id":    fmt.Sprintf("ch_%d", mpResp.ID),
		"status":       mpResp.Status,
		"status_detail": mpResp.StatusDetail,
		"mp_payment_id": mpResp.ID,
		"installments": installments,
		"message":      "Card payment processed via Mercado Pago",
	})
}

func ProcessPayment(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	description := fmt.Sprintf("Pedido %s", req.OrderID)
	email := req.CustomerEmail
	if email == "" {
		email = "cliente@email.com"
	}

	if req.Method == "credit" || req.Method == "debit" {
		paymentMethodID := "visa"
		if req.Method == "debit" {
			paymentMethodID = "debit"
		}
		installments := req.Installments
		if installments <= 0 {
			installments = 1
		}

		mpResp, err := services.CreateCardPayment(req.Amount, description, req.CardToken, email, installments, paymentMethodID)
		if err != nil {
			log.Printf("Error processing card payment via MP: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Payment processing failed"})
		}

		payment := models.Payment{
			ID:             primitive.NewObjectID(),
			OrderID:        req.OrderID,
			CustomerID:     req.CustomerID,
			EstablishmentID: req.EstablishmentID,
			Amount:         req.Amount,
			Method:         req.Method,
			Status:         mpResp.Status,
			CardToken:      req.CardToken,
			Installments:   installments,
			MPPaymentID:    mpResp.ID,
			MPStatus:       mpResp.Status,
			CreatedAt:      time.Now(),
		}

		if mpResp.Status == "approved" {
			now := time.Now()
			payment.ConfirmedAt = &now
			payment.Status = "CONFIRMED"
		}

		_, err = models.MongoDabase.Collection("payments").InsertOne(context.Background(), payment)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		response := dto.PaymentResponse{
			PaymentID:   payment.ID.Hex(),
			Status:      payment.Status,
			MPPaymentID: mpResp.ID,
			Message:     "Payment processed via Mercado Pago",
		}

		return c.Status(201).JSON(response)
	}

	if req.Method == "pix" {
		resp, err := services.CreatePIXPayment(req.Amount, description, email, req.CustomerName)
		if err != nil {
			log.Printf("Error processing PIX payment via MP: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "PIX payment failed"})
		}

		var qrCodeBase64, copyPaste string
		if resp.PointOfInteraction != nil && resp.PointOfInteraction.TransactionData != nil {
			qrCodeBase64 = resp.PointOfInteraction.TransactionData.QRCodeBase64
			copyPaste = resp.PointOfInteraction.TransactionData.CopyPaste
		}

		payment := models.Payment{
			ID:             primitive.NewObjectID(),
			OrderID:        req.OrderID,
			CustomerID:     req.CustomerID,
			EstablishmentID: req.EstablishmentID,
			Amount:         req.Amount,
			Method:         "pix",
			Status:         resp.Status,
			PixCopyPaste:   copyPaste,
			QRCodeBase64:   qrCodeBase64,
			MPPaymentID:    resp.ID,
			MPStatus:       resp.Status,
			CreatedAt:      time.Now(),
		}

		_, err = models.MongoDabase.Collection("payments").InsertOne(context.Background(), payment)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
		}

		response := dto.PaymentResponse{
			PaymentID:    payment.ID.Hex(),
			Status:       resp.Status,
			PixCopyPaste: copyPaste,
			QRCodeBase64: qrCodeBase64,
			MPPaymentID:  resp.ID,
			Message:      "PIX payment created via Mercado Pago",
		}

		return c.Status(201).JSON(response)
	}

	return c.Status(400).JSON(fiber.Map{"error": "Invalid payment method"})
}
