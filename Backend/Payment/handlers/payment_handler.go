package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/carloshomar/vercardapio/payment/services"
)

type PaymentHandler struct {
	ApprovalEngine *services.ApprovalEngine
	Gateway        *services.GatewayService
	WalletService  *services.WalletService
}

func NewPaymentHandler() *PaymentHandler {
	return &PaymentHandler{
		ApprovalEngine: services.NewApprovalEngine(),
		Gateway:        services.NewGatewayService(),
		WalletService:  services.NewWalletService(),
	}
}

func (ph *PaymentHandler) CreatePayment(c *fiber.Ctx) error {
	var payment models.Payment
	if err := c.BodyParser(&payment); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := ph.ApprovalEngine.ProcessPayment(&payment); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to process payment"})
	}

	return c.Status(201).JSON(payment)
}

func (ph *PaymentHandler) GetPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	objID, err := repository.HexToObjectID(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payment ID"})
	}

	payment, err := repository.GetPaymentByID(objID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Payment not found"})
	}

	return c.JSON(payment)
}

func (ph *PaymentHandler) ListPayments(c *fiber.Ctx) error {
	filter := models.PaymentFilter{
		Status:          c.Query("status"),
		RiskLevel:       c.Query("risk_level"),
		EstablishmentID: c.Query("establishment_id"),
		CustomerID:      c.Query("customer_id"),
		Method:          c.Query("method"),
		DateFrom:        c.Query("date_from"),
		DateTo:          c.Query("date_to"),
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	filter.Page = page
	filter.Limit = limit

	payments, total, err := repository.ListPayments(filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list payments"})
	}

	return c.JSON(fiber.Map{
		"payments": payments,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (ph *PaymentHandler) ApprovePayment(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := ph.ApprovalEngine.ApprovePayment(id, userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to approve payment"})
	}

	return c.JSON(fiber.Map{"message": "Payment approved"})
}

func (ph *PaymentHandler) RejectPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Locals("user_id").(string)

	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := ph.ApprovalEngine.RejectPayment(id, userID, body.Reason); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to reject payment"})
	}

	return c.JSON(fiber.Map{"message": "Payment rejected"})
}

func (ph *PaymentHandler) GetStats(c *fiber.Ctx) error {
	stats, err := repository.GetPaymentStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get stats"})
	}
	return c.JSON(stats)
}

func (ph *PaymentHandler) CreatePixPayment(c *fiber.Ctx) error {
	var req services.CreatePixRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	resp, err := ph.Gateway.CreatePixPayment(&req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(resp)
}

func (ph *PaymentHandler) GetPixStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, err := ph.Gateway.GetPixStatus(id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}
