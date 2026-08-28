package handlers

// orders.go — CRUD de pedidos.
// CORTE 5 (banco-único): o POSTGRES é a fonte da verdade (tabela
// order_documents). O Mongo legado é usado apenas em:
//   - dual-write best-effort na escrita (orders_pg.go);
//   - fallback de leitura em listagens enquanto o ETL não roda;
//   - lazy import em buscas pontuais por ID.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
)

func CreateOrder(c *fiber.Ctx, sendMessageToClient func(clientID int64, message []byte) error) error {
	var request dto.RequestPayload

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}

	if request.ScheduledAt != nil && !request.ScheduledAt.IsZero() {
		request.Status = "SCHEDULED"
		request.IsScheduled = true
	} else {
		request.Status = "AWAIT_APPROVE"
		now := time.Now()
		request.ScheduledAt = &now
	}

	// O total é SEMPRE recalculado no servidor a partir dos preços do banco.
	// O valor enviado pelo cliente (itens do carrinho) nunca é usado — evita
	// pedido de R$100,00 criado com payload de R$0,01.
	serverTotal, totalErr := computeOrderTotal(request.Cart, request.DeliveryValue, request.EstablishmentId)
	if totalErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": totalErr.Error(),
		})
	}
	request.OrderTotal = serverTotal

	if !request.IsScheduled {
		isOpen, err := checkEstablishmentOpen(request.EstablishmentId)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Erro ao verificar horário do estabelecimento",
			})
		}
		if !isOpen {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Estabelecimento fechado neste horário",
			})
		}
	}

	establishment, err := GetEstablishment(request.EstablishmentId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao obter detalhes do estabelecimento",
		})
	}

	request.Establishment = *establishment

	// Corte 5: ID público gerado no formato legado (ObjectID hex) para não
	// quebrar nenhum consumidor; Postgres é a fonte primária, Mongo espelhado.
	orderID := newLegacyOrderID()
	request.OrderId = orderID

	doc, err := payloadToDoc(orderID, &request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao serializar a ordem",
		})
	}
	if err := saveOrderPrimary(doc); err != nil {
		log.Printf("[ORDER] Falha ao persistir pedido: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao inserir a ordem no banco de dados",
		})
	}

	jsonData, _ := json.Marshal(request)
	if err := sendMessageToClient(request.EstablishmentId, jsonData); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"message": "Ordem criada com sucesso",
		"orderId": orderID,
	})
}

// computeOrderTotal recalcula o total do pedido no servidor: preço de cada
// produto e adicional vem da tabela do banco (não do payload do cliente),
// multiplicado pela quantidade, somado ao valor de entrega informado.
func computeOrderTotal(cart []dto.CartItem, deliveryValue float64, establishmentID int64) (float64, error) {
	if deliveryValue < 0 {
		return 0, fmt.Errorf("valor de entrega inválido")
	}
	if len(cart) == 0 {
		return 0, fmt.Errorf("carrinho vazio")
	}
	if authModels.DB == nil {
		return 0, fmt.Errorf("postgres indisponível")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	total := deliveryValue
	for _, ci := range cart {
		if ci.Quantity <= 0 {
			return 0, fmt.Errorf("quantidade inválida para o produto %s", ci.Item.Name)
		}
		var p models.Product
		if err := authModels.DB.WithContext(ctx).First(&p, ci.Item.ID).Error; err != nil {
			return 0, fmt.Errorf("produto %d não encontrado", ci.Item.ID)
		}
		if p.EstablishmentID != uint(establishmentID) {
			return 0, fmt.Errorf("produto %d não pertence a este estabelecimento", p.ID)
		}
		total += p.Price * float64(ci.Quantity)

		for _, addID := range ci.Additionals {
			var a models.Additional
			if err := authModels.DB.WithContext(ctx).First(&a, addID).Error; err != nil {
				return 0, fmt.Errorf("adicional %d não encontrado", addID)
			}
			if a.EstablishmentID != uint(establishmentID) {
				return 0, fmt.Errorf("adicional %d não pertence a este estabelecimento", a.ID)
			}
			total += a.Price
		}
	}
	return total, nil
}

// canActOnEstablishment verifica se o chamador pode agir sobre recursos do
// estabelecimento informado: admin sempre pode; role de estabelecimento só se
// o establishment_id do token for o dono.
func canActOnEstablishment(c *fiber.Ctx, establishmentID int64) bool {
	role, rErr := middlewares.GetUserRoleFromToken(c)
	if rErr != nil {
		return false
	}
	if role == "admin" {
		return true
	}
	tokenEstID, eErr := middlewares.GetEstablishmentIDFromToken(c)
	if eErr != nil {
		return false
	}
	return tokenEstID == establishmentID
}

// isValidOrderTransition limita os status aceitos em PUT /orders/status.
// (REQUEST_APPROVE é o pedido do cliente reabrindo aprovação; os demais são
// transições do restaurante.)
func isValidOrderTransition(status string) bool {
	switch status {
	case "REQUEST_APPROVE", "APPROVED", "DENIED", "PREPARING", "DONE", "CANCELLED":
		return true
	}
	return false
}

// GetEstablishment busca direto no Postgres (mesma base do auth_api, pool já
// importado neste arquivo). Substitui a antiga self-call HTTP via
// URL_GET_ESTABLISHMENT_ID, que pânica sem a env e adicionava um salto de rede
// no caminho crítico da criação de pedidos.
func GetEstablishment(establishmentID int64) (*dto.Establishment, error) {
	if authModels.DB == nil {
		return nil, fmt.Errorf("postgres indisponível")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var est authModels.Establishment
	if err := authModels.DB.WithContext(ctx).First(&est, establishmentID).Error; err != nil {
		return nil, err
	}

	return &dto.Establishment{
		HorarioFuncionamento: est.HorarioFuncionamento,
		Id:                   int64(est.ID),
		Image:                est.Image,
		Latitude:             est.Lat,
		Longitude:            est.Long,
		MaxDistanceDelivery:  est.MaxDistanceDelivery,
		Name:                 est.Name,
		OwnerId:              int64(est.OwnerID),
		PrimaryCollor:        est.PrimaryColor,
		SecondaryCollor:      est.SecondaryColor,
		LocationString:       est.LocationString,
	}, nil
}

// checkEstablishmentOpen verifica se o estabelecimento está aberto antes de
// aceitar pedidos. Consulta diretamente o Postgres (mesmo banco do monolito)
// para evitar latência de HTTP loopback e falhas de porta.
func checkEstablishmentOpen(establishmentID int64) (bool, error) {
	var establishment models.Establishment
	if err := authModels.DB.First(&establishment, establishmentID).Error; err != nil {
		return false, fmt.Errorf("establishment not found: %w", err)
	}
	return establishment.OpenData != nil, nil
}

func UpdateOrderStatus(c *fiber.Ctx, sendMessageToClient func(clientID int64, message []byte) error) error {
	var requestBody dto.UpdateOrderStatusRequest
	if err := c.BodyParser(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Erro ao fazer parsing do corpo da requisição",
		})
	}

	// Corte 5: leitura Postgres-first com lazy import do Mongo (legado).
	doc, err := findOrderByLegacyID(requestBody.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Pedido não encontrado",
		})
	}

	// Autorização: quem muda status é o estabelecimento dono do pedido
	// (claim establishment_id) ou um admin. Sem isso qualquer autenticado
	// podia marcar pedidos alheios como DONE/CANCELLED (IDOR).
	if !canActOnEstablishment(c, doc.EstablishmentID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Sem permissão para alterar este pedido",
		})
	}

	if !isValidOrderTransition(requestBody.Status) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Status inválido",
		})
	}

	// Payload atual para WebSocket e push notification.
	var order dto.RequestPayload
	_ = json.Unmarshal(doc.Payload, &order)

	if requestBody.Status != "REQUEST_APPROVE" {
		order.OrderId = doc.LegacyID
		order.Status = requestBody.Status
		// RabbitMQ removido — fila gerenciada pelo monolito via Redis
		log.Printf("[ORDER] Order %s status update published", order.OrderId)
	}

	jsonData, _ := json.Marshal(requestBody)

	if err := sendMessageToClient(doc.EstablishmentID, jsonData); err != nil {
		return err
	}

	// Mutação única: status + código de retirada quando DONE. Grava no
	// Postgres e espelha no Mongo best-effort (orders_pg.go).
	err = patchOrderDoc(doc, func(p *dto.RequestPayload) {
		p.Status = requestBody.Status
		if requestBody.Status == "DONE" {
			// Código de retirada fica na coluna tipada (doc.PickupCode);
			// o payload não tem esse campo por design.
			doc.PickupCode = generateSecureCode()
		}
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Erro ao atualizar ordem no banco de dados",
		})
	}

	go sendStatusPushNotification(order, requestBody.Status)

	return c.JSON(fiber.Map{
		"message": "Status do pedido atualizado com sucesso",
	})
}

// sendStatusPushNotification envia push de mudança de status ao cliente.
// CORTE 1 (banco-único): a leitura dos tokens agora é 100% Postgres.
// O caminho antigo consultava o Mongo por user_phone — campo que a escrita
// (RegisterPushToken) nunca gravava, ou seja: provável caminho morto. Aqui
// resolvemos phone → user_id na tabela clients (auth_api) e buscamos os
// tokens por user_id, que é como eles são realmente indexados.
func sendStatusPushNotification(order dto.RequestPayload, status string) {
	statusMessages := map[string]string{
		"APPROVED":          "Seu pedido foi aprovado e está sendo preparado!",
		"DONE":              "Seu pedido está pronto e saiu para entrega!",
		"IN_ROUTE_DELIVERY": "Seu pedido está a caminho!",
		"FINISHED":          "Seu pedido foi entregue! Bom apetite!",
		"CANCELLED":         "Seu pedido foi cancelado.",
		"SCHEDULED":         "Seu pedido foi agendado com sucesso!",
	}

	msg, ok := statusMessages[status]
	if !ok {
		return
	}

	title := "Atualização do Pedido"
	if status == "FINISHED" {
		title = "Pedido Entregue"
	} else if status == "CANCELLED" {
		title = "Pedido Cancelado"
	}

	userPhone := order.User.Phone
	if userPhone == "" || models.DB == nil {
		return
	}

	// 1) Resolve o id do cliente pelo telefone (tabela clients, auth_api).
	var clientIDs []int64
	if err := models.DB.Table("clients").
		Where("phone = ?", userPhone).
		Pluck("id", &clientIDs).Error; err != nil {
		log.Printf("[PUSH] Erro ao buscar cliente por phone=%s: %v", userPhone, err)
		return
	}
	if len(clientIDs) == 0 {
		return // cliente sem cadastro/telefone — nada a notificar
	}

	// 2) Busca os tokens registrados para esses ids.
	var tokens []models.PushToken
	if err := models.DB.
		Where("user_id IN ? AND user_type = ?", clientIDs, "client").
		Find(&tokens).Error; err != nil {
		log.Printf("[PUSH] Erro ao buscar push tokens: %v", err)
		return
	}

	for _, t := range tokens {
		if err := SendPushNotification(t.UserID, t.UserType, title, msg, map[string]interface{}{
			"order_id": order.OrderId,
			"status":   status,
			"type":     "status_update",
		}); err != nil {
			log.Printf("Erro ao enviar push: %v", err)
		}
	}
}

func ListOrdersByEstablishmentID(c *fiber.Ctx) error {
	establishmentID := c.Params("establishmentId")

	if establishmentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID do estabelecimento não fornecido",
		})
	}

	establishmentIDInt, err := strconv.Atoi(establishmentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID do estabelecimento inválido",
		})
	}

	// Postgres primário (corte 5).
	var docs []models.OrderDocument
	if models.DB != nil {
		models.DB.Where("establishment_id = ?", establishmentIDInt).
			Order("created_at desc").Limit(500).Find(&docs)
	}

	var formattedOrders []map[string]interface{}
	for _, d := range docs {
		formattedOrders = append(formattedOrders, docToResponseMap(&d))
	}

	if formattedOrders == nil {
		formattedOrders = []map[string]interface{}{}
	}

	return c.JSON(formattedOrders)
}

func ListOrdersByEstablishmentIDAndPhone(c *fiber.Ctx) error {
	establishmentID := c.Params("establishmentId")
	phoneNumberEncoded := c.Params("phoneNumber")

	if establishmentID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID do estabelecimento não fornecido"})
	}

	establishmentIDInt, err := strconv.Atoi(establishmentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID do estabelecimento inválido"})
	}

	phoneNumber, err := url.QueryUnescape(phoneNumberEncoded)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Número de telefone inválido"})
	}

	var docs []models.OrderDocument
	if models.DB != nil {
		models.DB.Where("establishment_id = ? AND user_phone = ?", establishmentIDInt, phoneNumber).
			Order("created_at desc").Limit(500).Find(&docs)
	}

	var orders []map[string]interface{}
	for _, d := range docs {
		orders = append(orders, docToResponseMap(&d))
	}

	if orders == nil {
		orders = []map[string]interface{}{}
	}

	return c.JSON(orders)
}

func ListAllOrders(c *fiber.Ctx) error {
	var docs []models.OrderDocument
	if models.DB != nil {
		models.DB.Order("created_at desc").Limit(500).Find(&docs)
	}

	var orders []map[string]interface{}
	for _, d := range docs {
		orders = append(orders, docToResponseMap(&d))
	}

	if orders == nil {
		orders = []map[string]interface{}{}
	}

	// Backfill de user.nome: o pedido só carrega o nome se o app o enviou.
	// Completa nomes faltantes consultando a tabela users do Postgres pelo
	// phone (os pedidos nao guardam user_id). Batch, sem N+1, fallback silencioso.
	enrichOrdersWithUsers(orders)

	return c.JSON(orders)
}

// enrichOrdersWithUsers preenche user.nome dos pedidos que vieram sem nome,
// consultando a tabela users do Postgres por telefone. Nao sobrescreve nomes
// ja presentes e ignora silenciosamente quando o DB esta indisponivel ou o
// telefone nao casa com nenhum usuario.
func enrichOrdersWithUsers(orders []map[string]interface{}) {
	if models.DB == nil {
		return
	}

	// Coleta phones unicos dos pedidos que tem user.phone
	seen := make(map[string]bool)
	var phones []string
	for _, o := range orders {
		userMap, ok := o["user"].(map[string]interface{})
		if !ok {
			continue
		}
		phone, _ := userMap["phone"].(string)
		if phone == "" || seen[phone] {
			continue
		}
		seen[phone] = true
		phones = append(phones, phone)
	}
	if len(phones) == 0 {
		return
	}

	var users []struct {
		Phone string
		Name  string
	}
	if err := models.DB.Table("users").Select("phone, name").Where("phone IN ?", phones).Find(&users).Error; err != nil {
		log.Printf("[ORDERS] Falha ao buscar nomes dos clientes: %v", err)
		return
	}

	byPhone := make(map[string]string, len(users))
	for _, u := range users {
		if u.Name != "" {
			byPhone[u.Phone] = u.Name
		}
	}

	for _, o := range orders {
		userMap, ok := o["user"].(map[string]interface{})
		if !ok {
			continue
		}
		if nome, _ := userMap["nome"].(string); nome != "" {
			continue // ja tem nome — nao sobrescreve
		}
		phone, _ := userMap["phone"].(string)
		if name, ok := byPhone[phone]; ok {
			userMap["nome"] = name
		}
	}
}

func ListOrdersByPhone(c *fiber.Ctx) error {
	phoneNumberEncoded := c.Params("phone")

	phoneNumber, err := url.QueryUnescape(phoneNumberEncoded)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Erro ao decodificar número de telefone"})
	}

	tokenPhone, phoneErr := middlewares.GetUserPhoneFromToken(c)
	if phoneErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}
	role, roleErr := middlewares.GetUserRoleFromToken(c)
	if roleErr != nil || (role != "admin" && tokenPhone != phoneNumber) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	var docs []models.OrderDocument
	if models.DB != nil {
		models.DB.Where("user_phone = ?", phoneNumber).
			Order("created_at desc").Limit(500).Find(&docs)
	}

	var orders []map[string]interface{}
	for _, d := range docs {
		orders = append(orders, docToResponseMap(&d))
	}
	if orders == nil {
		orders = []map[string]interface{}{}
	}

	return c.JSON(orders)
}
