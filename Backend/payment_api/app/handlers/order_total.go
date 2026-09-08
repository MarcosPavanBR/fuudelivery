package handlers

import (
	"log"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/payment_api/app/dto"
	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// orderFacts são os campos do pedido que o servidor considera autoritativos
// numa cobrança. Nenhum deles pode vir do corpo da requisição.
type orderFacts struct {
	EstablishmentID int64
	UserPhone       string
}

// lookupOrderRecipient devolve QUEM recebe o dinheiro do pedido, lido das
// colunas tipadas de order_documents (não do payload).
//
// Isto fecha um desvio de dinheiro real. O `amount` e o `delivery_amount` já
// eram reconferidos contra o pedido, mas o DESTINATÁRIO não: `establishment_id`
// vinha do corpo da requisição e era exatamente ele que decidia qual carteira
// o webhook creditava (webhook.go -> adjustEstablishmentWallet). Como
// POST /establishments/register é aberto, dava para registrar um
// estabelecimento proprio, fazer um pedido de verdade num restaurante alheio
// e mandar `establishment_id` apontando para si: o restaurante entregava a
// comida e o split ia para a carteira do atacante, sacavel por
// EstablishmentWithdraw.
//
// customer_id/customer_phone entram na mesma categoria: alimentam a parcela de
// cashback do split e sao a base do ACL de leitura em canViewOrderPayment, ou
// seja, o chamador escolhia quem podia ler a propria cobranca.
func lookupOrderRecipient(orderID string) (orderFacts, bool) {
	if models.DB == nil {
		return orderFacts{}, false
	}
	var row struct {
		EstablishmentID *int64
		UserPhone       *string
	}
	err := models.DB.Raw(
		`SELECT establishment_id, user_phone
		 FROM order_documents
		 WHERE legacy_id = ?
		 LIMIT 1`, orderID).Scan(&row).Error
	if err != nil {
		log.Printf("[PAYMENT] lookupOrderRecipient(%s): %v", orderID, err)
		return orderFacts{}, false
	}
	if row.EstablishmentID == nil || *row.EstablishmentID <= 0 {
		return orderFacts{}, false
	}
	facts := orderFacts{EstablishmentID: *row.EstablishmentID}
	if row.UserPhone != nil {
		facts.UserPhone = *row.UserPhone
	}
	return facts, true
}

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

// validateChargeAmount garante que o valor cobrado é exatamente o total do
// pedido calculado no servidor. Tolerância de 1 centavo para ruído de float.
func validateChargeAmount(orderID string, clientAmount float64) (float64, bool) {
	serverTotal, ok := lookupOrderTotal(orderID)
	if !ok {
		return 0, false
	}
	diff := toCents(serverTotal) - toCents(clientAmount)
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		return serverTotal, false
	}
	return serverTotal, true
}

// lookupOrderDelivery devolve o frete registrado no pedido (campo
// deliveryValue do mesmo JSONB de order_documents, escrito por
// orders_api/CreateOrder junto com o resto do dto.RequestPayload).
//
// Diferente do total, ZERO é valor legítimo aqui — retirada no balcão e
// frete grátis existem. Por isso só a ausência do campo (pedido legado,
// anterior ao corte de valores server-side) devolve false; 0 devolve
// (0, true).
func lookupOrderDelivery(orderID string) (float64, bool) {
	if models.DB == nil {
		return 0, false
	}
	var row struct {
		Delivery *float64
	}
	err := models.DB.Raw(
		`SELECT NULLIF(payload->>'deliveryValue', '')::float8 AS delivery
		 FROM order_documents
		 WHERE legacy_id = ?
		 LIMIT 1`, orderID).Scan(&row).Error
	if err != nil {
		log.Printf("[PAYMENT] lookupOrderDelivery(%s): %v", orderID, err)
		return 0, false
	}
	if row.Delivery == nil || *row.Delivery < 0 {
		return 0, false
	}
	return *row.Delivery, true
}

// resolveDeliveryAmount decide qual frete gravar na cobrança.
//
// Por que existe: o Amount já era conferido contra o servidor
// (validateChargeAmount), mas o DeliveryAmount vinha CRU do corpo da
// requisição e seguia direto para services.CalculateSplitRules — onde
// `deliveryAmount >= total` zera plataforma E estabelecimento e manda o
// valor inteiro para a regra "deliveryman". Um cliente mandando
// delivery_amount igual ao amount pagava o pedido normalmente e desviava
// 100% do dinheiro do estabelecimento.
//
// Regra: o frete registrado no pedido manda, com a mesma tolerância de 1
// centavo usada no total. Só quando o pedido não tem o campo caímos no valor
// do cliente — e aí vale a guarda de sanidade 0 <= delivery <= total, que é
// justamente o que fechava o desvio acima.
func resolveDeliveryAmount(orderID string, clientDelivery, serverTotal float64) (float64, bool) {
	if clientDelivery < 0 {
		return 0, false
	}

	if serverDelivery, ok := lookupOrderDelivery(orderID); ok {
		diff := toCents(serverDelivery) - toCents(clientDelivery)
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			return serverDelivery, false
		}
		// A guarda de sanidade vale AQUI TAMBÉM, e não só no ramo legado.
		// O valor gravado no pedido não é um valor de servidor de verdade:
		// orders_api/computeOrderTotal aceita o deliveryValue que o cliente
		// mandou na criação do pedido, sem recalcular por zona. Então quem
		// controla o pedido controla este "servidor" — tratar o campo como
		// autoritativo sem teto só mudaria o momento do desvio, da cobrança
		// para a criação do pedido.
		if toCents(serverDelivery) >= toCents(serverTotal) {
			return serverDelivery, false
		}
		return serverDelivery, true
	}

	// `>=`, não `>`: o dano começa exatamente na igualdade, porque
	// CalculateSplitRules trata `deliveryAmount >= total` como "a entrega
	// consome tudo" e zera plataforma e estabelecimento. serverTotal aqui é
	// sempre > 0 (lookupOrderTotal rejeita total <= 0), então isto não
	// bloqueia pedido de valor zero — ele nem chega até aqui.
	if toCents(clientDelivery) >= toCents(serverTotal) {
		return 0, false
	}
	return clientDelivery, true
}

// bindRecipientToOrder sobrescreve os campos de DESTINATÁRIO da requisição com
// os do pedido e com a identidade do token. Sobrescreve em vez de rejeitar:
// nenhum cliente legítimo passa a tomar 400 por isso, e o valor do corpo deixa
// de ter qualquer efeito.
//
// Devolve false só quando o pedido não tem estabelecimento conhecido — aí não
// há para quem creditar e a cobrança não deve existir.
func bindRecipientToOrder(c *fiber.Ctx, req *dto.PaymentRequest) bool {
	facts, ok := lookupOrderRecipient(req.OrderID)
	if !ok {
		return false
	}

	if req.EstablishmentID != 0 && req.EstablishmentID != facts.EstablishmentID {
		// Não é erro de digitação: é a tentativa de redirecionar o split.
		log.Printf("[PAYMENT] establishment_id do corpo (%d) diverge do pedido %s (%d) — usando o do pedido",
			req.EstablishmentID, req.OrderID, facts.EstablishmentID)
	}
	req.EstablishmentID = facts.EstablishmentID

	if facts.UserPhone != "" {
		req.CustomerPhone = facts.UserPhone
	}

	// customer_id sai do token, não do corpo: ele endereça a parcela de
	// cashback do split.
	if tokenUserID, err := middlewares.GetUserIDFromToken(c); err == nil && tokenUserID > 0 {
		req.CustomerID = tokenUserID
	}

	return true
}

// canViewOrderPayment decide quem pode consultar o status/valor de uma
// cobrança (auditoria de segurança — achado #3: vazamento de dados de
// pagamento de pedidos alheios):
//   - admin: qualquer cobrança;
//   - cliente dono do pedido (phone do token == customer_phone);
//   - estabelecimento dono da cobrança (establishment_id do token).
//
// Entregador atribuído e demais papéis ficam de fora (sem vazamento).
func canViewOrderPayment(role string, tokenPhone string, phoneErr error, tokenEstID int64, estErr error, customerPhone string, establishmentID int64) bool {
	if role == "admin" {
		return true
	}
	isCustomer := phoneErr == nil && tokenPhone != "" && customerPhone == tokenPhone
	if isCustomer {
		return true
	}
	isEstablishment := estErr == nil && tokenEstID == establishmentID
	return isEstablishment
}

// GetPaymentByOrder devolve o status da cobrança mais recente de um pedido.
// Usado pelo app do cliente para confirmar o pagamento do PIX (polling) sem
// confiar num botão "já paguei".
// GET /payments/order/:order_id (protegido)
func GetPaymentByOrder(c *fiber.Ctx) error {
	orderID := c.Params("order_id")
	if orderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id obrigatório"})
	}

	var payment models.Payment
	if err := models.DB.Where("order_id = ?", orderID).
		Order("created_at DESC").First(&payment).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Nenhuma cobrança encontrada para este pedido"})
	}

	// Participant validation: only the customer, establishment owner, assigned
	// deliveryman, or admin can view payment status.
	role, rErr := middlewares.GetUserRoleFromToken(c)
	if rErr != nil {
		return c.Status(403).JSON(fiber.Map{"error": "Invalid token"})
	}
	tokenPhone, phoneErr := middlewares.GetUserPhoneFromToken(c)
	tokenEstID, estErr := middlewares.GetEstablishmentIDFromToken(c)
	if !canViewOrderPayment(role, tokenPhone, phoneErr, tokenEstID, estErr, payment.CustomerPhone, payment.EstablishmentID) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden"})
	}

	return c.JSON(fiber.Map{
		"order_id":       payment.OrderID,
		"status":         payment.Status,
		"amount":         payment.Amount,
		"payment_method": payment.Method,
	})
}
