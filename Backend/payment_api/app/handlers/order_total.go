package handlers

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// OrderAuthData contém os dados autorizativos de um pedido extraídos de
// order_documents. Usado para validar que o usuário autenticado pode pagar
// o pedido e para obter os IDs canônicos de cliente/estabelecimento.
type OrderAuthData struct {
	OrderTotal      float64
	EstablishmentID int64
	CustomerPhone   string
	CustomerID      int64
	DeliveryAmount  float64
	Status          string
}

// ErrOrderNotFound indica que o pedido não existe em order_documents.
var ErrOrderNotFound = errors.New("order not found")

// ErrOrderAlreadyPaid indica que o pedido já possui um pagamento confirmado.
var ErrOrderAlreadyPaid = errors.New("order already paid")

// ErrUnauthorizedPayment indica que o usuário autenticado não é o dono do pedido.
var ErrUnauthorizedPayment = errors.New("unauthorized: user does not own this order")

// lookupOrderTotal devolve o total recalculado pelo servidor no momento da
// criação do pedido (campo order_total do JSONB em order_documents, escrito
// por orders_api/computeOrderTotal). Retorna false se o pedido não existir
// ou ainda não tiver total válido (pedidos anteriores ao corte de valores
// server-side) — nesses casos a cobrança é rejeitada em vez de confiar no
// amount enviado pelo cliente.
func lookupOrderTotal(orderID string) (float64, bool) {
	if models.DB == nil {
		return 0, false
	}
	var row struct {
		Total *float64
	}
	err := models.DB.Raw(
		`SELECT NULLIF(payload->>'order_total', '')::float8 AS total
		 FROM order_documents
		 WHERE legacy_id = ?
		 LIMIT 1`, orderID).Scan(&row).Error
	if err != nil {
		log.Printf("[PAYMENT] lookupOrderTotal(%s): %v", orderID, err)
		return 0, false
	}
	if row.Total == nil || *row.Total <= 0 {
		return 0, false
	}
	return *row.Total, true
}

// authorizeAndLoadOrder valida que o usuário autenticado pode pagar o pedido
// e retorna os dados autorizativos (establishment_id, customer_id, phone, etc.)
// extraídos de order_documents. Defesas implementadas:
//
//  1. Verifica que o pedido existe em order_documents.
//  2. Verifica que o telefone do JWT corresponde ao user_phone do pedido
//     (o usuário autenticado é o dono do pedido).
//  3. Verifica que o pedido não possui pagamento CONFIRMED (evita dupla cobrança).
//  4. Extrai establishment_id, customer_id e delivery_amount do payload JSONB
//     (fonte da verdade — nunca confia nos valores do request body).
//
// Retorna ErrOrderNotFound, ErrUnauthorizedPayment ou ErrOrderAlreadyPaid
// conforme apropriado.
func authorizeAndLoadOrder(c *fiber.Ctx, orderID string) (*OrderAuthData, error) {
	if models.DB == nil {
		return nil, errors.New("database not available")
	}

	// Extrai o telefone do usuário autenticado do JWT.
	authenticatedPhone, err := middlewares.GetUserPhoneFromToken(c)
	if err != nil {
		log.Printf("[PAYMENT] Failed to extract phone from JWT for order %s: %v", orderID, err)
		return nil, ErrUnauthorizedPayment
	}

	// Busca o pedido em order_documents com as colunas tipadas e o payload JSONB.
	var row struct {
		EstablishmentID int64
		UserPhone       string
		Status          string
		Payload         []byte
	}
	err = models.DB.Raw(
		`SELECT establishment_id, user_phone, status, payload
		 FROM order_documents
		 WHERE legacy_id = ?
		 LIMIT 1`, orderID).Scan(&row).Error
	if err != nil {
		log.Printf("[PAYMENT] Order %s not found in order_documents: %v", orderID, err)
		return nil, ErrOrderNotFound
	}

	// Verifica que o telefone do JWT corresponde ao user_phone do pedido.
	if row.UserPhone != authenticatedPhone {
		log.Printf("[PAYMENT] Authorization failed: JWT phone %s does not match order %s phone %s",
			authenticatedPhone, orderID, row.UserPhone)
		return nil, ErrUnauthorizedPayment
	}

	// Verifica se o pedido já possui um pagamento confirmado (evita dupla cobrança).
	var confirmedCount int64
	err = models.DB.Model(&models.Payment{}).
		Where("order_id = ? AND status = ?", orderID, "CONFIRMED").
		Count(&confirmedCount).Error
	if err != nil {
		log.Printf("[PAYMENT] Failed to check existing payments for order %s: %v", orderID, err)
		return nil, errors.New("failed to verify payment status")
	}
	if confirmedCount > 0 {
		log.Printf("[PAYMENT] Order %s already has a confirmed payment", orderID)
		return nil, ErrOrderAlreadyPaid
	}

	// Desserializa o payload JSONB para extrair order_total, customer_id e delivery_amount.
	var payload map[string]interface{}
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		log.Printf("[PAYMENT] Failed to parse payload for order %s: %v", orderID, err)
		return nil, errors.New("invalid order payload")
	}

	// Extrai order_total (obrigatório).
	orderTotal, ok := payload["order_total"].(float64)
	if !ok || orderTotal <= 0 {
		log.Printf("[PAYMENT] Order %s has invalid or missing order_total", orderID)
		return nil, errors.New("order has no valid total")
	}

	// Extrai customer_id (obrigatório).
	customerID, ok := payload["customer_id"].(float64)
	if !ok || customerID <= 0 {
		log.Printf("[PAYMENT] Order %s has invalid or missing customer_id", orderID)
		return nil, errors.New("order has no valid customer_id")
	}

	// Extrai delivery_amount (opcional, default 0).
	deliveryAmount, _ := payload["delivery_amount"].(float64)

	return &OrderAuthData{
		OrderTotal:      orderTotal,
		EstablishmentID: row.EstablishmentID,
		CustomerPhone:   row.UserPhone,
		CustomerID:      int64(customerID),
		DeliveryAmount:  deliveryAmount,
		Status:          row.Status,
	}, nil
}

// GetPaymentByOrder devolve o status da cobrança mais recente de um pedido.
// Usado pelo app do cliente para confirmar o pagamento do PIX (polling) sem
// confiar num botão "já paguei".
// GET /payments/order/:order_id (protegido)
//
// Autorização: verifica que o usuário autenticado é o dono do pedido antes
// de retornar o status do pagamento (defesa contra IDOR).
func GetPaymentByOrder(c *fiber.Ctx) error {
	orderID := c.Params("order_id")
	if orderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id obrigatório"})
	}

	// Valida que o usuário autenticado é o dono do pedido.
	orderData, err := authorizeAndLoadOrder(c, orderID)
	if err != nil {
		if err == ErrUnauthorizedPayment {
			return c.Status(403).JSON(fiber.Map{"error": "Acesso negado: você não é o dono deste pedido"})
		}
		if err == ErrOrderNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "Pedido não encontrado"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao validar pedido"})
	}

	var payment models.Payment
	if err := models.DB.Where("order_id = ?", orderID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Nenhuma cobrança encontrada para este pedido"})
	}

	// Verifica que o pagamento pertence ao mesmo cliente (defesa em profundidade).
	if payment.CustomerID != orderData.CustomerID {
		log.Printf("[PAYMENT] Payment customer_id mismatch for order %s: payment has %d, order has %d",
			orderID, payment.CustomerID, orderData.CustomerID)
		return c.Status(403).JSON(fiber.Map{"error": "Acesso negado"})
	}

	return c.JSON(fiber.Map{
		"order_id":       payment.OrderID,
		"status":         payment.Status,
		"amount":         payment.Amount,
		"payment_method": payment.Method,
	})
}
