// Package handlers - approval_handler.go
// Handlers HTTP para operacoes de aprovacao de pagamentos.
package handlers

import (
	"strconv"

	"github.com/carloshomar/fuudelivery/payment/models"
	"github.com/carloshomar/fuudelivery/payment/repository"
	"github.com/carloshomar/fuudelivery/payment/services"
	"github.com/gofiber/fiber/v2"
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
// Le do MongoDB; se nao existir, retorna as regras padrao.
// GET /api/approvals/rules
func (ah *ApprovalHandler) GetRules(c *fiber.Ctx) error {
	rules, err := repository.GetApprovalRules()
	if err != nil {
		defaultRules := models.DefaultApprovalRules()
		return c.JSON(defaultRules)
	}
	return c.JSON(rules)
}

// UpdateRules atualiza as regras de aprovacao e persiste no MongoDB.
// Aceita um JSON com os campos das regras e faz upsert no banco.
// ANTES: carrega as regras existentes para NAO zerar campos nao enviados.
// PUT /api/approvals/rules
func (ah *ApprovalHandler) UpdateRules(c *fiber.Ctx) error {
	// Carrega regras existentes primeiro — se nao houver, usa defaults
	rules, err := repository.GetApprovalRules()
	if err != nil {
		defaultRules := models.DefaultApprovalRules()
		rules = &defaultRules
	}

	// BodyParser aplica apenas os campos enviados no JSON sobre o struct existente.
	// Campos NAO enviados mantem seus valores atuais — nao sao zerados.
	if err := c.BodyParser(rules); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := repository.SaveApprovalRules(rules); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save rules"})
	}

	return c.JSON(fiber.Map{"message": "Rules updated", "rules": rules})
}
