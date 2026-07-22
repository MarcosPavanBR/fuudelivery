package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/carloshomar/vercardapio/payment/services"
)

type ChargebackHandler struct {
	Service *services.ChargebackService
}

func NewChargebackHandler() *ChargebackHandler {
	return &ChargebackHandler{
		Service: services.NewChargebackService(),
	}
}

func (ch *ChargebackHandler) CreateChargeback(c *fiber.Ctx) error {
	var chargeback models.Chargeback
	if err := c.BodyParser(&chargeback); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	chargeback.Status = models.ChargebackPending
	if err := ch.Service.CreateChargeback(&chargeback); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create chargeback"})
	}

	return c.Status(201).JSON(chargeback)
}

func (ch *ChargebackHandler) GetChargeback(c *fiber.Ctx) error {
	id := c.Params("id")
	chargeback, err := ch.Service.GetChargeback(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Chargeback not found"})
	}
	return c.JSON(chargeback)
}

func (ch *ChargebackHandler) ListChargebacks(c *fiber.Ctx) error {
	status := c.Query("status")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	chargebacks, total, err := ch.Service.ListChargebacks(status, page, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list chargebacks"})
	}

	return c.JSON(fiber.Map{
		"chargebacks": chargebacks,
		"total":       total,
		"page":        page,
		"limit":       limit,
	})
}

func (ch *ChargebackHandler) ApproveChargeback(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := ch.Service.ApproveChargeback(id, userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to approve chargeback"})
	}

	return c.JSON(fiber.Map{"message": "Chargeback approved"})
}

func (ch *ChargebackHandler) RejectChargeback(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Locals("user_id").(string)

	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := ch.Service.RejectChargeback(id, userID, body.Reason); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to reject chargeback"})
	}

	return c.JSON(fiber.Map{"message": "Chargeback rejected"})
}

func (ch *ChargebackHandler) AddEvidence(c *fiber.Ctx) error {
	chargebackID := c.Params("id")
	objID, err := repository.HexToObjectID(chargebackID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid chargeback ID"})
	}

	var evidence models.Evidence
	if err := c.BodyParser(&evidence); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	evidence.ChargebackID = objID
	evidence.UploadedBy = c.Locals("user_id").(string)

	if err := repository.CreateEvidence(&evidence); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to add evidence"})
	}

	return c.Status(201).JSON(evidence)
}

func (ch *ChargebackHandler) GetEvidences(c *fiber.Ctx) error {
	chargebackID := c.Params("id")
	objID, err := repository.HexToObjectID(chargebackID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid chargeback ID"})
	}

	evidences, err := repository.GetEvidencesByChargeback(objID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get evidences"})
	}

	return c.JSON(evidences)
}

func (ch *ChargebackHandler) GetStats(c *fiber.Ctx) error {
	ctx := repository.MongoCtx()

	stats := map[string]interface{}{}

	total, _ := repository.Chargebacks.CountDocuments(ctx, map[string]interface{}{})
	stats["total"] = total

	pending, _ := repository.Chargebacks.CountDocuments(ctx, map[string]interface{}{"status": "pending"})
	stats["pending"] = pending

	approved, _ := repository.Chargebacks.CountDocuments(ctx, map[string]interface{}{"status": "approved"})
	stats["approved"] = approved

	rejected, _ := repository.Chargebacks.CountDocuments(ctx, map[string]interface{}{"status": "rejected"})
	stats["rejected"] = rejected

	escalated, _ := repository.Chargebacks.CountDocuments(ctx, map[string]interface{}{"status": "escalated"})
	stats["escalated"] = escalated

	return c.JSON(stats)
}
