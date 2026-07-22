package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/carloshomar/vercardapio/payment/services"
)

type ApprovalHandler struct {
	Engine *services.ApprovalEngine
}

func NewApprovalHandler() *ApprovalHandler {
	return &ApprovalHandler{
		Engine: services.NewApprovalEngine(),
	}
}

func (ah *ApprovalHandler) GetQueue(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	filter := models.PaymentFilter{
		Status: "pending",
		Page:   page,
		Limit:  limit,
	}

	payments, total, err := repository.ListPayments(filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get approval queue"})
	}

	return c.JSON(fiber.Map{
		"payments": payments,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (ah *ApprovalHandler) GetAutoApproved(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	filter := models.PaymentFilter{
		Status: "approved",
		Page:   page,
		Limit:  limit,
	}

	payments, total, err := repository.ListPayments(filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get auto-approved payments"})
	}

	return c.JSON(fiber.Map{
		"payments": payments,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (ah *ApprovalHandler) GetRules(c *fiber.Ctx) error {
	rules := map[string]interface{}{
		"auto_approve_max_amount":    1000,
		"auto_approve_max_risk":      20,
		"manual_review_min_amount":   5000,
		"manual_review_min_risk":     60,
		"compliance_min_risk":        80,
		"block_chargeback_active":    true,
		"block_max_daily_withdrawals": 3,
	}
	return c.JSON(rules)
}

func (ah *ApprovalHandler) UpdateRules(c *fiber.Ctx) error {
	var rules map[string]interface{}
	if err := c.BodyParser(&rules); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	return c.JSON(fiber.Map{"message": "Rules updated", "rules": rules})
}
