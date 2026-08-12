package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GeneratePIX(c *fiber.Ctx) error {
	var req dto.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	description := fmt.Sprintf("Pedido %s", req.OrderID)

	client := services.NewAbacatePayClient()
	chargeReq := services.PIXChargeRequest{}
	chargeReq.Data.Amount = int64(req.Amount)
	chargeReq.Data.Description = description
	chargeReq.Data.ExternalID = req.OrderID
	// customer é opcional para PIX; se enviado, TODOS os campos (incl. taxId/CPF
	// válido) são obrigatórios. O monolito não coleta CPF → omitir para não
	// tomar 422 do gateway. (O AppComida poderá passar taxId no futuro.)

	apiResp, err := client.CreatePIXCharge(chargeReq)
	if err != nil {
		log.Printf("Error creating PIX payment via AbacatePay: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create PIX payment"})
	}

	payment := models.Payment{
		ID:              primitive.NewObjectID(),
		OrderID:         req.OrderID,
		CustomerID:      req.CustomerID,
		CustomerPhone:   req.CustomerPhone,
		EstablishmentID: req.EstablishmentID,
		Amount:          req.Amount,
		DeliveryAmount:  req.DeliveryAmount,
		Method:          "pix",
		Status:          "PENDING",
		PixQRCode:       apiResp.QRCode,
		PixCopyPaste:    apiResp.CopyPaste,
		QRCodeBase64:    apiResp.QRCodeBase64,
		AbacatePayID:    apiResp.ID,
		CreatedAt:       time.Now(),
	}

	_, err = models.MongoDabase.Collection("payments").InsertOne(mongoCtx(), payment)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save payment"})
	}

	response := dto.PaymentResponse{
		PaymentID:    payment.ID.Hex(),
		Status:       "PENDING",
		PixQRCode:    apiResp.QRCode,
		PixCopyPaste: apiResp.CopyPaste,
		QRCodeBase64: apiResp.QRCodeBase64,
		AbacatePayID: apiResp.ID,
		Message:      "PIX payment created via AbacatePay",
	}

	return c.Status(201).JSON(response)
}
