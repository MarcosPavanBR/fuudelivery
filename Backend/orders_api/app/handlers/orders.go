package handlers

// orders.go — CRUD de pedidos.
// CORTE 5 (banco-único): o POSTGRES é a fonte da verdade (tabela
// order_documents). O Mongo legado é usado apenas em:
//   - dual-write best-effort na escrita (orders_pg.go);
//   - fallback de leitura em listagens enquanto o ETL não roda;
//   - lazy import em buscas pontuais por ID.
// Quando o Atlas for desligado, remover os ramos *_legacy* — nada mais muda.

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/orders_api/app/dto"
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
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

func GetEstablishment(establishmentID int64) (*dto.Establishment, error) {
	urlEnv := os.Getenv("URL_GET_ESTABLISHMENT_ID")
	if urlEnv == "" {
		panic("URL_GET_ESTABLISHMENT_ID não configurado.")
	}

	url := fmt.Sprintf(urlEnv, establishmentID)
	log.Println(url)
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API retornou status não OK: %d", response.StatusCode)
	}

	var establishmentDTO dto.Establishment
	if err := json.NewDecoder(response.Body).Decode(&establishmentDTO); err != nil {
		return nil, err
	}

	return &establishmentDTO, nil
}

// clientHTTPCheck é o client HTTP usado para checagens internas entre domínios.
// Timeout curto e obrigatório: sem ele, um endpoint lento travaria a criação
// de pedidos inteira (o handler é síncrono).
var clientHTTPCheck = &http.Client{Timeout: 3 * time.Second}

// checkEstablishmentOpen verifica se o estabelecimento está aberto antes de
// aceitar pedidos. A URL base é configurável via URL_CHECK_ESTABLISHMENT_OPEN
// (template com %d para o ID). O default aponta para o PRÓPRIO processo —
// desde 2026-08 auth e orders vivem no mesmo binário (monolito), então o
// antigo hostname de Docker Compose "auth-api" não resolve mais em produção.
func checkEstablishmentOpen(establishmentID int64) (bool, error) {
	urlEnv := os.Getenv("URL_CHECK_ESTABLISHMENT_OPEN")
	if urlEnv == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "3000"
		}
		urlEnv = fmt.Sprintf("http://localhost:%s/api/auth/establishments/%%d/is-open", port)
	}

	url := fmt.Sprintf(urlEnv, establishmentID)
	resp, err := clientHTTPCheck.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var result struct {
		IsOpen bool `json:"is_open"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	return result.IsOpen, nil
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
		if err := SendPushNotification(t.PushToken, title, msg, map[string]interface{}{
			"order_id": order.OrderId,
			"status":   status,
			"type":     "status_update",
		}); err != nil {
			log.Printf("Erro ao enviar push: %v", err)
		}
	}
}

// listOrdersFromMongoLegacy executa a mesma query que os handlers faziam antes
// do corte 5. Serve de FALLBACK enquanto o ETL Mongo→Postgres não rodou:
// se o Postgres ainda não tem pedidos daquele filtro, servimos do Atlas.
func listOrdersFromMongoLegacy(filter bson.M, sortField string, limit int64) []map[string]interface{} {
	if models.MongoDabase == nil {
		return nil
	}
	collection := models.MongoDabase.Collection("orders")

	findOpts := mongoFindOptions(sortField, limit)
	cursor, err := collection.Find(mongoCtx(), filter, findOpts)
	if err != nil {
		log.Printf("[ORDER-LEGACY] Falha na consulta fallback ao Mongo: %v", err)
		return nil
	}
	defer cursor.Close(mongoCtx())

	var orders []map[string]interface{}
	if err := cursor.All(mongoCtx(), &orders); err != nil {
		return nil
	}
	return orders
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

	// Fallback legado: nada migrado ainda para este estabelecimento.
	if len(formattedOrders) == 0 {
		legacy := listOrdersFromMongoLegacy(
			bson.M{"establishmentid": establishmentIDInt}, "", 0)
		if legacy != nil {
			formattedOrders = legacy
		}
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

	if len(orders) == 0 {
		legacy := listOrdersFromMongoLegacy(
			bson.M{"establishmentid": establishmentIDInt, "user.phone": phoneNumber}, "", 0)
		if legacy != nil {
			orders = legacy
		}
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

	// Fallback legado: banco Postgres ainda vazio (ETL pendente).
	if len(orders) == 0 {
		legacy := listOrdersFromMongoLegacy(bson.M{}, "created_at", 500)
		if legacy != nil {
			orders = legacy
		}
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
	if len(orders) == 0 {
		return
	}
	db := authModels.DB
	if db == nil {
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
	if err := db.Table("users").Select("phone, name").Where("phone IN ?", phones).Find(&users).Error; err != nil {
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

	var docs []models.OrderDocument
	if models.DB != nil {
		models.DB.Where("user_phone = ?", phoneNumber).
			Order("created_at desc").Limit(500).Find(&docs)
	}

	var orders []map[string]interface{}
	for _, d := range docs {
		orders = append(orders, docToResponseMap(&d))
	}

	if len(orders) == 0 {
		legacy := listOrdersFromMongoLegacy(bson.M{"user.phone": phoneNumber}, "lastModified", 0)
		if legacy != nil {
			orders = legacy
		}
	}
	if orders == nil {
		orders = []map[string]interface{}{}
	}

	return c.JSON(orders)
}
