package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/payment_api/app/services"
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	"github.com/gofiber/fiber/v2"
)

// CreateRecipientRequest é o body da requisição para criar um recebedor.
type CreateRecipientRequest struct {
	Name       string `json:"name"`
	Document   string `json:"cpf_cnpj"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	PersonType string `json:"person_type"` // "JURIDICA" ou "FISICA"
}

// CreateRecipient cria uma sub-conta no gateway de pagamento para recebimento
// de splits. O restaurante ou entregador precisa ter dados bancários para
// receber transferências.
//
// POST /wallets/create-recipient
func CreateRecipient(c *fiber.Ctx) error {
	var req CreateRecipientRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Valida campos obrigatórios
	if req.Name == "" || req.Document == "" || req.Email == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name, cpf_cnpj and email are required"})
	}

	// Obtém o user_id do JWT
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Limpa documento: remove caracteres não numéricos
	req.Document = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, req.Document)

	// Verifica se já existe um recipient para este usuário
	var existing models.Recipient
	if err := models.DB.Where("user_id = ? AND user_type = ?", userID, "establishment").First(&existing).Error; err == nil {
		// Já existe — retorna o existente
		return c.Status(200).JSON(fiber.Map{
			"wallet_id":      existing.GatewayRecipientID,
			"gateway":        existing.Gateway,
			"status":         existing.Status,
			"already_exists": true,
		})
	}

	// Cria o recipient via gateway
	gwReq := &gateway.RecipientRequest{
		UserType: "restaurant",
		UserID:   userID,
		Name:     req.Name,
		Document: req.Document,
		Email:    req.Email,
		Phone:    req.Phone,
	}

	resp, ok := services.CreateRecipientViaGateway(context.Background(), gwReq)
	if !ok {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to create recipient in payment gateway"})
	}

	// Salva no banco de dados
	recipient := models.Recipient{
		UserType:           "establishment",
		UserID:             int(userID),
		Gateway:            resp.Gateway,
		GatewayRecipientID: resp.RecipientID,
		Status:             string(resp.Status),
		TransferInterval:   "daily",
	}

	if err := models.DB.Create(&recipient).Error; err != nil {
		log.Printf("[RECIPIENT] Falha ao salvar recipient: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save recipient locally"})
	}

	log.Printf("[RECIPIENT] Criado: user=%d gateway=%s recipient_id=%s", userID, resp.Gateway, resp.RecipientID)

	return c.Status(201).JSON(fiber.Map{
		"wallet_id": resp.RecipientID,
		"gateway":   resp.Gateway,
		"status":    resp.Status,
		"message":   "Conta de recebimento criada com sucesso",
	})
}

// GetRecipientStatus retorna o status do recebedor do restaurante logado.
//
// GET /wallets/recipient-status
func GetRecipientStatus(c *fiber.Ctx) error {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var recipient models.Recipient
	if err := models.DB.Where("user_id = ? AND user_type = ?", userID, "establishment").First(&recipient).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Recipient not found"})
	}

	// Busca saldo do gateway se disponível
	available, pending, ok := services.GetRecipientBalanceViaGateway(context.Background(), recipient.GatewayRecipientID)
	balanceAvailable := int64(0)
	balancePending := int64(0)
	if ok {
		balanceAvailable = available
		balancePending = pending
	}

	return c.JSON(fiber.Map{
		"wallet_id":         recipient.GatewayRecipientID,
		"gateway":           recipient.Gateway,
		"status":            recipient.Status,
		"balance_available": balanceAvailable,
		"balance_pending":   balancePending,
		"created_at":        recipient.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// getUserIDFromContext extrai o user_id do JWT no contexto Fiber.
func getUserIDFromContext(c *fiber.Ctx) (int64, error) {
	// O middleware de auth coloca o user_id no locals
	userID, ok := c.Locals("userID").(int64)
	if !ok {
		// Tenta converter de float64 (JSON number)
		if f, ok := c.Locals("userID").(float64); ok {
			return int64(f), nil
		}
		return 0, fmt.Errorf("userID not found in context")
	}
	return userID, nil
}
