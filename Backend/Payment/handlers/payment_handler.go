// Package handlers - payment_handler.go
// Handlers HTTP para operacoes de pagamentos.
// Cada handler recebe uma requisicao HTTP, valida os dados,
// chama o servico correspondente e retorna a resposta.
package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
	"github.com/carloshomar/vercardapio/payment/services"
)

// PaymentHandler e responsavel pelas rotas de pagamentos.
type PaymentHandler struct {
	ApprovalEngine *services.ApprovalEngine // Motor de decisao
	Gateway        *services.GatewayService // Gateway AbacatePay
	WalletService  *services.WalletService  // Servico de carteiras
}

// NewPaymentHandler cria uma nova instancia do handler.
func NewPaymentHandler() *PaymentHandler {
	return &PaymentHandler{
		ApprovalEngine: services.NewApprovalEngine(),
		Gateway:        services.NewGatewayService(),
		WalletService:  services.NewWalletService(),
	}
}

// CreatePayment cria um novo pagamento e processa sua aprovacao.
// POST /api/payments
func (ph *PaymentHandler) CreatePayment(c *fiber.Ctx) error {
	// Parse do corpo da requisicao
	var payment models.Payment
	if err := c.BodyParser(&payment); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Processa o pagamento (calcula risco e decide aprovacao)
	if err := ph.ApprovalEngine.ProcessPayment(&payment); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to process payment"})
	}

	return c.Status(201).JSON(payment)
}

// GetPayment busca um pagamento pelo ID.
// GET /api/payments/:id
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

// ListPayments lista pagamentos com filtros e paginacao.
// GET /api/payments?status=pending&page=1&limit=20
func (ph *PaymentHandler) ListPayments(c *fiber.Ctx) error {
	// Extrai filtros da query string
	filter := models.PaymentFilter{
		Status:          c.Query("status"),
		RiskLevel:       c.Query("risk_level"),
		EstablishmentID: c.Query("establishment_id"),
		CustomerID:      c.Query("customer_id"),
		Method:          c.Query("method"),
		DateFrom:        c.Query("date_from"),
		DateTo:          c.Query("date_to"),
	}

	// Extrai paginacao (default: pagina 1, 20 itens)
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	filter.Page = page
	filter.Limit = limit

	// Busca pagamentos no banco
	payments, total, err := repository.ListPayments(filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list payments"})
	}

	// Retorna lista com metadados de paginacao
	return c.JSON(fiber.Map{
		"payments": payments,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// ApprovePayment aprova manualmente um pagamento pendente.
// POST /api/payments/:id/approve
func (ph *PaymentHandler) ApprovePayment(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := ph.ApprovalEngine.ApprovePayment(id, userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to approve payment"})
	}

	return c.JSON(fiber.Map{"message": "Payment approved"})
}

// RejectPayment rejeita manualmente um pagamento pendente.
// POST /api/payments/:id/reject
func (ph *PaymentHandler) RejectPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Locals("user_id").(string)

	// Parse do motivo da rejeicao
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

// GetStats retorna estatisticas gerais dos pagamentos.
// GET /api/payments/stats
func (ph *PaymentHandler) GetStats(c *fiber.Ctx) error {
	stats, err := repository.GetPaymentStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get stats"})
	}
	return c.JSON(stats)
}

// CreatePixPayment cria uma cobranca PIX via AbacatePay.
// POST /api/payments/pix
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

// GetPixStatus consulta o status de uma cobranca PIX.
// GET /api/payments/pix/:id/status
func (ph *PaymentHandler) GetPixStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, err := ph.Gateway.GetPixStatus(id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}
