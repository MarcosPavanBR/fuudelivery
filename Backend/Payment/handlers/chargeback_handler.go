// Package handlers - chargeback_handler.go
// Handlers HTTP para operacoes de estornos (chargebacks).
package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/carloshomar/vercardapio/payment/services"
)

// ChargebackHandler e responsavel pelas rotas de estornos.
type ChargebackHandler struct {
	Service *services.ChargebackService
}

// NewChargebackHandler cria uma nova instancia do handler.
func NewChargebackHandler() *ChargebackHandler {
	return &ChargebackHandler{
		Service: services.NewChargebackService(),
	}
}

// CreateChargeback cria um novo estorno.
// POST /api/chargebacks
func (ch *ChargebackHandler) CreateChargeback(c *fiber.Ctx) error {
	var chargeback models.Chargeback
	if err := c.BodyParser(&chargeback); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Define status inicial como pendente
	chargeback.Status = models.ChargebackPending
	if err := ch.Service.CreateChargeback(&chargeback); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create chargeback"})
	}

	return c.Status(201).JSON(chargeback)
}

// GetChargeback busca um estorno pelo ID.
// GET /api/chargebacks/:id
func (ch *ChargebackHandler) GetChargeback(c *fiber.Ctx) error {
	id := c.Params("id")
	chargeback, err := ch.Service.GetChargeback(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Chargeback not found"})
	}
	return c.JSON(chargeback)
}

// ListChargebacks lista estornos com filtro e paginacao.
// GET /api/chargebacks?status=pending&page=1&limit=20
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

// ApproveChargeback aprova um estorno.
// POST /api/chargebacks/:id/approve
func (ch *ChargebackHandler) ApproveChargeback(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := ch.Service.ApproveChargeback(id, userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to approve chargeback"})
	}

	return c.JSON(fiber.Map{"message": "Chargeback approved"})
}

// RejectChargeback rejeita um estorno.
// POST /api/chargebacks/:id/reject
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

// AddEvidence adiciona uma evidencia a um estorno.
// POST /api/chargebacks/:id/evidence
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

	// Associa a evidencia ao estorno e registra quem enviou
	evidence.ChargebackID = objID
	evidence.UploadedBy = c.Locals("user_id").(string)

	if err := repository.CreateEvidence(&evidence); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to add evidence"})
	}

	return c.Status(201).JSON(evidence)
}

// GetEvidences retorna todas as evidencias de um estorno.
// GET /api/chargebacks/:id/evidence
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

// GetStats retorna estatisticas dos estornos.
// GET /api/chargebacks/stats
func (ch *ChargebackHandler) GetStats(c *fiber.Ctx) error {
	ctx := repository.MongoCtx()

	stats := map[string]interface{}{}

	// Conta por status
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
