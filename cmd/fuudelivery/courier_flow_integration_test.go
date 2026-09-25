//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	chatModels "github.com/carloshomar/fuudelivery/chat_api/app/models"
	deliveryModels "github.com/carloshomar/fuudelivery/delivery_api/app/models"
	ordersHandlers "github.com/carloshomar/fuudelivery/orders_api/app/handlers"
	ordersModels "github.com/carloshomar/fuudelivery/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const courierFlowSecret = "segredo-de-teste-do-fluxo-do-entregador"

// TestCourierFlow percorre o fluxo do entregador de ponta a ponta, que estava
// desligado em quatro pontos: nada alimentava delivery_solicitations desde a
// saída do RabbitMQ, a lista dava 403 para todo entregador, o aceite lia o
// campo errado (404) e o avanço de etapa gravava status vazio.
//
// loja aprova → entregador vê (sem dado do cliente) → aceita → avança →
// loja marca pronto → entregador valida código, sai e entrega → pedido
// FINISHED → extrato. Em cada etapa, o cliente e o entregador de MESMO id
// numérico tentam agir no lugar um do outro.
func TestCourierFlow(t *testing.T) {
	uri := os.Getenv("POSTGRES_TEST_URI")
	if uri == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("POSTGRES_TEST_URI ausente em CI")
		}
		t.Skip("POSTGRES_TEST_URI não definida")
	}
	db, err := gorm.Open(postgresdriver.Open(uri), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	tables := []interface{}{&authModels.Establishment{}, &authModels.Client{}, &authModels.User{},
		&authModels.DeliveryMan{}, &ordersModels.OrderDocument{}, &deliveryModels.DeliverySolicitation{},
		&chatModels.ChatMessage{}}
	_ = db.Migrator().DropTable(tables...)
	require.NoError(t, db.AutoMigrate(tables...))

	prevAuth, prevOrders, prevDelivery, prevChat := authModels.DB, ordersModels.DB, deliveryModels.DB, chatModels.DB
	authModels.DB, ordersModels.DB, deliveryModels.DB, chatModels.DB = db, db, db, db
	t.Cleanup(func() {
		authModels.DB, ordersModels.DB, deliveryModels.DB, chatModels.DB = prevAuth, prevOrders, prevDelivery, prevChat
	})
	t.Setenv("JWT_SECRET", courierFlowSecret)
	wireCourierFlow()

	require.NoError(t, db.Create(&authModels.Establishment{
		ID: 7, Name: "Pizzaria do Cadastro", Lat: -23.5505, Long: -46.6333, LocationString: "Rua A, 1",
	}).Error)
	require.NoError(t, db.Create(&authModels.Client{ID: 5, Name: "Ana", Phone: "+5511911112222", Password: "x"}).Error)
	require.NoError(t, db.Create(&authModels.DeliveryMan{ID: 5, Name: "Beto"}).Error)
	require.NoError(t, db.Create(&authModels.User{ID: 5, Name: "Usuário 5", Role: "user"}).Error)

	payload := func(id string) []byte {
		b, _ := json.Marshal(map[string]interface{}{
			"order_id": id, "status": "AWAIT_APPROVE", "establishmentId": 7,
			// O app do cliente manda o que quiser aqui; a fila usa o cadastro.
			"establishment": map[string]interface{}{"name": "Nome forjado", "lat": 0, "long": 0},
			"user":          map[string]interface{}{"nome": "Ana", "phone": "+5511911112222"},
			"location": map[string]interface{}{
				"logradouro": "Rua B", "numero": "10", "bairro": "Centro",
				"coords": map[string]interface{}{"latitude": -23.5605, "longitude": -46.6433},
			},
			"deliveryValue": 7.5, "order_total": 57.5, "distance": 0,
			"paymentMethod": map[string]interface{}{"type": "pix"},
			"cart":          []interface{}{map[string]interface{}{"quantity": 1, "item": map[string]interface{}{"name": "Pizza"}}},
		})
		return b
	}
	for _, id := range []string{"ord-flow", "ord-cancel"} {
		require.NoError(t, db.Create(&ordersModels.OrderDocument{
			LegacyID: id, EstablishmentID: 7, UserPhone: "+5511911112222", Status: "AWAIT_APPROVE", Payload: payload(id),
		}).Error)
	}

	app := fiber.New()
	setupDeliveryRoutes(app)
	app.Put("/orders/status", protectedRoute, func(c *fiber.Ctx) error {
		return ordersHandlers.UpdateOrderStatus(c, sendToEstablishment)
	})
	app.Post("/orders/pickup-code/validate", protectedRoute, ordersHandlers.ValidatePickupCode)
	setupChatRoutes(app)
	setupWebSocketRoutes(app)

	sign := func(claims jwt.MapClaims) string {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(courierFlowSecret))
		require.NoError(t, err)
		return s
	}
	loja := sign(jwt.MapClaims{"id": 1, "role": "user", "account_type": "user", "establishment_id": 7})
	entregador := sign(jwt.MapClaims{"id": 5, "name": "Beto", "account_type": "deliveryman"})
	outroEntregador := sign(jwt.MapClaims{"id": 6, "name": "Caio", "account_type": "deliveryman"})
	cliente := sign(jwt.MapClaims{"id": 5, "role": "client", "account_type": "client", "phone": "+5511911112222"})
	clienteLegado := sign(jwt.MapClaims{"id": 5, "role": "client", "phone": "+5511911112222"})

	call := func(method, path, token string, body interface{}) (int, []byte) {
		t.Helper()
		var r io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			r = bytes.NewReader(b)
		}
		req := httptest.NewRequest(method, path, r)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, 5000)
		require.NoError(t, err)
		out, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, out
	}
	list := func(token string) []map[string]interface{} {
		t.Helper()
		code, body := call("GET", "/solicitation-orders?latitude=-23.55&longitude=-46.63&limitDistance=10", token, nil)
		require.Equal(t, 200, code, string(body))
		var out []map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &out))
		return out
	}
	setStatus := func(id, status string) {
		t.Helper()
		code, body := call("PUT", "/orders/status", loja, map[string]string{"id": id, "status": status})
		require.Equal(t, 200, code, "loja %s→%s: %s", id, status, body)
	}
	courierStep := func(token, status string) (int, string) {
		code, body := call("POST", "/deliveryman/status", token, map[string]interface{}{
			"order_id": "ord-flow", "deliveryman": map[string]interface{}{"id": 5, "status": status},
		})
		return code, string(body)
	}
	orderDoc := func(id string) (ordersModels.OrderDocument, map[string]interface{}) {
		var d ordersModels.OrderDocument
		require.NoError(t, db.Where("legacy_id = ?", id).First(&d).Error)
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal(d.Payload, &p))
		return d, p
	}

	// Antes de a loja aprovar, nada na fila.
	require.Empty(t, list(entregador))

	setStatus("ord-flow", "APPROVED")
	setStatus("ord-cancel", "APPROVED")

	// Cliente (token novo ou legado) não vê a fila de entregas.
	for _, tok := range []string{cliente, clienteLegado} {
		code, _ := call("GET", "/solicitation-orders?latitude=-23.55&longitude=-46.63&limitDistance=10", tok, nil)
		require.Equal(t, 403, code)
	}

	disponiveis := list(entregador)
	require.Len(t, disponiveis, 2)
	var o map[string]interface{}
	for _, d := range disponiveis {
		if d["order_id"] == "ord-flow" {
			o = d
		}
	}
	require.NotNil(t, o, "ord-flow na lista: %v", disponiveis)
	require.Equal(t, 7.5, o["deliveryValue"])
	require.EqualValues(t, 7, o["establishmentId"])
	est := o["establishment"].(map[string]interface{})
	require.Equal(t, "Pizzaria do Cadastro", est["name"], "a loja vem do cadastro, não do payload")
	require.Equal(t, -23.5505, est["lat"])
	require.Greater(t, o["distance"].(float64), 0.5, "distância loja→cliente calculada no servidor")
	require.NotContains(t, o, "user", "a lista não expõe o cliente")
	require.NotContains(t, o, "location", "a lista não expõe o endereço")

	// Cancelado sai da fila.
	setStatus("ord-cancel", "CANCELLED")
	require.Len(t, list(entregador), 1)

	// Aceito e depois cancelado pela loja: o entregador não avança mais.
	require.NoError(t, db.Create(&ordersModels.OrderDocument{
		LegacyID: "ord-late-cancel", EstablishmentID: 7, UserPhone: "+5511911112222", Status: "AWAIT_APPROVE", Payload: payload("ord-late-cancel"),
	}).Error)
	setStatus("ord-late-cancel", "APPROVED")
	code, _ := call("PUT", "/solicitation-orders/hand-shake", outroEntregador, map[string]interface{}{"order_id": "ord-late-cancel"})
	require.Equal(t, 200, code)
	setStatus("ord-late-cancel", "CANCELLED")
	code, _ = call("POST", "/deliveryman/status", outroEntregador, map[string]interface{}{
		"order_id": "ord-late-cancel", "deliveryman": map[string]interface{}{"status": "AWAIT_COLECT"},
	})
	require.Equal(t, 409, code, "pedido cancelado não avança")
	code, body := call("GET", "/deliveryman/has-active/6", outroEntregador, nil)
	require.Equal(t, 200, code)
	require.JSONEq(t, "[]", string(body), "cancelado sai das entregas ativas")

	// Cliente com o mesmo id do entregador não aceita no lugar dele.
	code, _ = call("PUT", "/solicitation-orders/hand-shake", cliente, map[string]interface{}{"order_id": "ord-flow"})
	require.Equal(t, 403, code)
	// Pedido cancelado não se aceita.
	code, _ = call("PUT", "/solicitation-orders/hand-shake", entregador, map[string]interface{}{"order_id": "ord-cancel"})
	require.Equal(t, 409, code)

	// Aceite no formato do app: order_id + deliveryman (id do body é ignorado).
	code, body = call("PUT", "/solicitation-orders/hand-shake", entregador, map[string]interface{}{
		"order_id": "ord-flow", "deliveryman": map[string]interface{}{"id": 999, "name": "Beto"},
	})
	require.Equal(t, 200, code, string(body))
	code, _ = call("PUT", "/solicitation-orders/hand-shake", outroEntregador, map[string]interface{}{"order_id": "ord-flow"})
	require.Equal(t, 409, code, "segundo entregador não pega o mesmo pedido")
	require.Empty(t, list(outroEntregador))

	_, p := orderDoc("ord-flow")
	dm := p["deliveryman"].(map[string]interface{})
	require.EqualValues(t, 5, dm["id"], "o entregador chega ao pedido")
	require.Equal(t, "IN_ROUTE_COLECT", dm["status"])

	// Chat do pedido: remetente, tipo e nome vêm do token e da tabela do tipo
	// da conta — o entregador não se apresenta como "restaurant"/"Suporte" e
	// não aparece com o nome do usuário 5.
	code, body = call("POST", "/chat/message", entregador, map[string]interface{}{
		"order_id": "ord-flow", "message": "cheguei", "sender_type": "restaurant", "sender_name": "Suporte", "sender_id": 1,
	})
	require.Equal(t, 200, code, string(body))
	code, body = call("POST", "/chat/message", cliente, map[string]interface{}{"order_id": "ord-flow", "message": "ok"})
	require.Equal(t, 200, code, string(body))
	var msgs []chatModels.ChatMessage
	require.NoError(t, db.Where("order_id = ?", "ord-flow").Order("id").Find(&msgs).Error)
	require.Len(t, msgs, 2)
	require.Equal(t, "deliveryman", msgs[0].SenderType)
	require.Equal(t, "Beto", msgs[0].SenderName)
	require.EqualValues(t, 5, msgs[0].SenderID)
	require.Equal(t, "client", msgs[1].SenderType)
	require.Equal(t, "Ana", msgs[1].SenderName)

	for nome, tok := range map[string]string{"cliente": cliente, "entregador": entregador, "loja": loja} {
		code, _ = call("GET", "/chat/messages/ord-flow", tok, nil)
		require.Equal(t, 200, code, "%s lê o chat do próprio pedido", nome)
	}
	code, _ = call("GET", "/chat/messages/ord-flow", outroEntregador, nil)
	require.Equal(t, 403, code, "entregador de fora não lê o chat")
	// O :orderId ia cru para o GORM (First(&order, orderID)) — condição SQL.
	code, _ = call("GET", "/chat/messages/1=1", cliente, nil)
	require.Equal(t, 403, code)

	// Cliente 5 marca como lido: a mensagem do ENTREGADOR 5 conta como dele
	// (tipo diferente); a própria mensagem do cliente, não.
	code, body = call("PUT", "/chat/read/ord-flow/5", cliente, nil)
	require.Equal(t, 200, code, string(body))
	require.NoError(t, db.Where("order_id = ?", "ord-flow").Order("id").Find(&msgs).Error)
	require.NotNil(t, msgs[0].ReadAt, "mensagem do entregador 5 lida pelo cliente 5")
	require.Nil(t, msgs[1].ReadAt, "a própria mensagem não se marca como lida")

	// Posição do entregador: só o entregador atribuído publica.
	code, _ = call("POST", "/delivery/location", cliente, map[string]interface{}{"order_id": "ord-flow", "lat": -23.5, "lng": -46.6})
	require.Equal(t, 403, code, "cliente 5 não publica a posição do entregador 5")
	code, body = call("POST", "/delivery/location", entregador, map[string]interface{}{"order_id": "ord-flow", "lat": -23.5, "lng": -46.6})
	require.Equal(t, 200, code, string(body))

	// Entrega em andamento: só o próprio entregador vê, com o cliente.
	code, _ = call("GET", "/deliveryman/has-active/5", cliente, nil)
	require.Equal(t, 403, code, "cliente 5 não lê as entregas do entregador 5")
	code, body = call("GET", "/deliveryman/has-active/5", entregador, nil)
	require.Equal(t, 200, code)
	var ativos []map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &ativos))
	require.Len(t, ativos, 1)
	require.Equal(t, "Ana", ativos[0]["user"].(map[string]interface{})["nome"])
	require.Equal(t, "+5511911112222", ativos[0]["user"].(map[string]interface{})["phone"])
	require.NotNil(t, ativos[0]["location"])
	require.Equal(t, "IN_ROUTE_COLECT", ativos[0]["deliveryman"].(map[string]interface{})["status"])

	// Etapas do entregador, no formato do app ({deliveryman: {status}}).
	code, _ = courierStep(cliente, "AWAIT_COLECT")
	require.Equal(t, 403, code)
	code, body2 := courierStep(entregador, "AWAIT_COLECT")
	require.Equal(t, 200, code, body2)
	code, _ = courierStep(entregador, "AWAIT_COLECT")
	require.Equal(t, 200, code, "repetir a mesma etapa é idempotente")
	code, _ = courierStep(entregador, "FINISHED")
	require.Equal(t, 409, code, "não pula da coleta para entregue")
	code, _ = courierStep(entregador, "IN_ROUTE_DELIVERY")
	require.Equal(t, 409, code, "não sai com o pedido ainda não pronto")

	setStatus("ord-flow", "PREPARING")
	setStatus("ord-flow", "DONE")
	d, _ := orderDoc("ord-flow")
	require.Len(t, d.PickupCode, 6)

	// Código de retirada: só o entregador do pedido valida — o cliente de
	// mesmo id, não.
	code, _ = call("POST", "/orders/pickup-code/validate", cliente, map[string]string{"order_id": "ord-flow", "pickup_code": d.PickupCode})
	require.Equal(t, 403, code)
	code, _ = call("POST", "/orders/pickup-code/validate", entregador, map[string]string{"order_id": "ord-flow", "pickup_code": d.PickupCode})
	require.Equal(t, 200, code)

	code, body2 = courierStep(entregador, "IN_ROUTE_DELIVERY")
	require.Equal(t, 200, code, body2)
	d, _ = orderDoc("ord-flow")
	require.Equal(t, "IN_ROUTE_DELIVERY", d.Status, "saída do entregador leva o pedido a caminho")

	code, body2 = courierStep(entregador, "FINISHED")
	require.Equal(t, 200, code, body2)
	d, p = orderDoc("ord-flow")
	require.Equal(t, "FINISHED", d.Status, "entrega do entregador finaliza o pedido")
	require.Equal(t, "FINISHED", p["deliveryman"].(map[string]interface{})["status"])
	require.Len(t, d.PickupCode, 6, "o código não se perde nas gravações do entregador")

	var s deliveryModels.DeliverySolicitation
	require.NoError(t, db.Where("order_id = ?", "ord-flow").First(&s).Error)
	require.Equal(t, "FINISHED", s.Status)
	require.EqualValues(t, 5, s.UserID, "user_id é o id do cliente")

	code, body = call("GET", "/deliveryman/has-active/5", entregador, nil)
	require.Equal(t, 200, code)
	require.JSONEq(t, "[]", string(body))

	code, _ = call("GET", "/deliveryman/extrato/5", cliente, nil)
	require.Equal(t, 403, code)
	code, body = call("GET", "/deliveryman/extrato/5", entregador, nil)
	require.Equal(t, 200, code)
	var extrato []map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &extrato))
	require.Len(t, extrato, 1)
	require.Equal(t, 7.5, extrato[0]["deliveryValue"])
	require.Equal(t, map[string]interface{}{"nome": "Ana"}, extrato[0]["user"], "extrato guarda só o nome do cliente")
	require.NotContains(t, extrato[0], "location")
	require.NotEmpty(t, extrato[0]["operationDate"])
}
