// Package handlers - approval_handler.go
// Handlers HTTP para operacoes de aprovacao de pagamentos.
package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/carloshomar/vercardapio/payment/services"
)

// ApprovalHandler e responsavel pelas rotas de aprovacao.
type ApprovalHandler struct {
	Engine *services.ApprovalEngine // Motor de decisao
}

// NewApprovalHandler cria uma nova instancia do handler.
func NewApprovalHandler() *ApprovalHandler {
	return &ApprovalHandler{
		Engine: services.NewApprovalEngine(),
	}
}

// GetQueue retorna a fila de pagamentos pendentes de aprovacao.
// GET /api/approvals/queue?page=1&limit=50
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

// GetAutoApproved retorna pagamentos aprovados automaticamente pelo sistema.
// GET /api/approvals/auto-approved?page=1&limit=50
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

// GetRules retorna as regras de aprovacao atuais.
// GET /api/approvals/rules
func (ah *ApprovalHandler) GetRules(c *fiber.Ctx) error {
	// Regras hardcoded - em producao, viriam do banco de dados
	rules := map[string]interface{}{
		"auto_approve_max_amount":     1000,  // Maximo para auto-aprovacao: R$1000
		"auto_approve_max_risk":       20,    // Score maximo para auto-aprovacao
		"manual_review_min_amount":    5000,  // Minimo para revisao manual
		"manual_review_min_risk":      60,    // Score minimo para revisao
		"compliance_min_risk":         80,    // Score minimo para compliance
		"block_chargeback_active":     true,  // Bloquear se tiver chargebacks
		"block_max_daily_withdrawals": 3,     // Maximo de saques por dia
	}
	return c.JSON(rules)
}

// UpdateRules atualiza as regras de aprovacao.
// PUT /api/approvals/rules
func (ah *ApprovalHandler) UpdateRules(c *fiber.Ctx) error {
	var rules map[string]interface{}
	if err := c.BodyParser(&rules); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	// TODO: Salvar regras no banco de dados
	return c.JSON(fiber.Map{"message": "Rules updated", "rules": rules})
}
