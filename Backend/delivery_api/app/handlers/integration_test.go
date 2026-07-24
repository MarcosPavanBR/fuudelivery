//go:build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/carloshomar/vercardapio/delivery_api/app/dto"
	"github.com/carloshomar/vercardapio/delivery_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupDeliveryIntegrationEnv(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()

	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err, "subir container do MongoDB")

	uri, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.NoError(t, err)
	require.NoError(t, client.Ping(ctx, nil))

	models.MongoClient = client
	models.MongoDabase = client.Database("delivery_test")

	oldRabbitConn := os.Getenv("RABBIT_CONNECTION")
	os.Unsetenv("RABBIT_CONNECTION")

	return func() {
		if oldRabbitConn != "" {
			os.Setenv("RABBIT_CONNECTION", oldRabbitConn)
		}
		_ = client.Disconnect(ctx)
		_ = mongoContainer.Terminate(ctx)
	}
}

func noopWS(_ int64, _ []byte) error { return nil }

func sampleOrder(orderID string) (dto.OrderDTO, []byte) {
	order := dto.OrderDTO{
		OrderId: orderID,
		Status:  "APPROVED",
		Establishment: dto.Establishment{
			ID:   1,
			Name: "Restaurante Teste",
			Lat:  -23.5505,
			Long: -46.6333,
		},
		DeliveryValue: 8.50,
		User:          dto.User{Phone: "+5511999999999", Nome: "Cliente Teste"},
	}
	body, _ := json.Marshal(order)
	return order, body
}

func TestDeliveryLifecycle(t *testing.T) {
	cleanup := setupDeliveryIntegrationEnv(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/handshake", func(c *fiber.Ctx) error { return HandShakeDeliveryman(c) })
	app.Post("/status", func(c *fiber.Ctx) error { return UpdateOrderStatusByDeliverymanID(c, noopWS) })
	app.Get("/solicitations", GetApprovedSolicitations)

	_, orderBody := sampleOrder("order-1")
	require.NoError(t, CreateSolicitation(string(orderBody), noopWS))

	var stored dto.OrderDTO
	collection := models.MongoDabase.Collection("solicitations")
	require.NoError(t, collection.FindOne(context.Background(), bson.M{"orderid": "order-1"}).Decode(&stored))
	require.Equal(t, "APPROVED", stored.Status)
	require.Equal(t, int64(0), stored.DeliveryMan.Id, "solicitação recém-criada não tem entregador")

	req := httptest.NewRequest(http.MethodGet,
		"/solicitations?latitude=-23.5605&longitude=-46.6333&limitDistance=5", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var available []dto.OrderDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&available))
	require.Len(t, available, 1, "a solicitação deveria aparecer pro entregador dentro do raio")
	require.Equal(t, "order-1", available[0].OrderId)

	handshakePayload := map[string]interface{}{
		"order_id": "order-1",
		"deliveryman": map[string]interface{}{
			"id":     int64(42),
			"name":   "João Entregador",
			"email":  "joao@example.com",
			"status": "IN_ROUTE_COLECT",
		},
	}
	handshakeBody, _ := json.Marshal(handshakePayload)
	hReq := httptest.NewRequest(http.MethodPost, "/handshake", bytes.NewReader(handshakeBody))
	hReq.Header.Set("Content-Type", "application/json")
	hResp, err := app.Test(hReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, hResp.StatusCode)

	var afterHandshake dto.OrderDTO
	require.NoError(t, collection.FindOne(context.Background(), bson.M{"orderid": "order-1"}).Decode(&afterHandshake))
	require.Equal(t, int64(42), afterHandshake.DeliveryMan.Id)
	require.Equal(t, "IN_ROUTE_COLECT", afterHandshake.DeliveryMan.Status)

	req2 := httptest.NewRequest(http.MethodGet,
		"/solicitations?latitude=-23.5605&longitude=-46.6333&limitDistance=5", nil)
	resp2, err := app.Test(req2, -1)
	require.NoError(t, err)
	var availableAfter []dto.OrderDTO
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&availableAfter))
	require.Empty(t, availableAfter, "solicitação já atribuída não deveria aparecer mais")

	statusPayload := map[string]interface{}{
		"order_id": "order-1",
		"deliveryman": map[string]interface{}{
			"id":     int64(42),
			"status": "FINISHED",
		},
	}
	statusBody, _ := json.Marshal(statusPayload)
	sReq := httptest.NewRequest(http.MethodPost, "/status", bytes.NewReader(statusBody))
	sReq.Header.Set("Content-Type", "application/json")
	sResp, err := app.Test(sReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, sResp.StatusCode)

	var finalState dto.OrderDTO
	require.NoError(t, collection.FindOne(context.Background(), bson.M{"orderid": "order-1"}).Decode(&finalState))
	require.Equal(t, "FINISHED", finalState.DeliveryMan.Status)

	listReq := httptest.NewRequest(http.MethodGet, "/deliveryman/42/orders", nil)
	listApp := fiber.New()
	listApp.Get("/deliveryman/:id/orders", GetOrdersByDeliverymanID)
	listResp, err := listApp.Test(listReq, -1)
	require.NoError(t, err)
	var remaining []dto.OrderDTO
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&remaining))
	require.Empty(t, remaining, "pedido FINISHED não deveria aparecer na lista ativa do entregador")
}

func TestHandShakeDeliveryman_AlreadyAssigned(t *testing.T) {
	cleanup := setupDeliveryIntegrationEnv(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/handshake", func(c *fiber.Ctx) error { return HandShakeDeliveryman(c) })

	_, orderBody := sampleOrder("order-2")
	require.NoError(t, CreateSolicitation(string(orderBody), noopWS))

	firstPayload := map[string]interface{}{
		"order_id":    "order-2",
		"deliveryman": map[string]interface{}{"id": int64(1), "name": "Primeiro"},
	}
	firstBody, _ := json.Marshal(firstPayload)
	firstReq := httptest.NewRequest(http.MethodPost, "/handshake", bytes.NewReader(firstBody))
	firstReq.Header.Set("Content-Type", "application/json")
	firstResp, err := app.Test(firstReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, firstResp.StatusCode)

	secondPayload := map[string]interface{}{
		"order_id":    "order-2",
		"deliveryman": map[string]interface{}{"id": int64(2), "name": "Segundo"},
	}
	secondBody, _ := json.Marshal(secondPayload)
	secondReq := httptest.NewRequest(http.MethodPost, "/handshake", bytes.NewReader(secondBody))
	secondReq.Header.Set("Content-Type", "application/json")
	secondResp, err := app.Test(secondReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, secondResp.StatusCode,
		"segundo entregador não deveria conseguir assumir um pedido já atribuído")
}

func TestCreateSolicitation_UpdatesExistingInsteadOfDuplicating(t *testing.T) {
	cleanup := setupDeliveryIntegrationEnv(t)
	defer cleanup()

	_, orderBody := sampleOrder("order-3")
	require.NoError(t, CreateSolicitation(string(orderBody), noopWS))

	updatedOrder, updatedBody := sampleOrder("order-3")
	updatedOrder.Status = "CANCELLED"
	updatedBody, _ = json.Marshal(updatedOrder)
	require.NoError(t, CreateSolicitation(string(updatedBody), noopWS))

	collection := models.MongoDabase.Collection("solicitations")
	count, err := collection.CountDocuments(context.Background(), bson.M{"orderid": "order-3"})
	require.NoError(t, err)
	require.Equal(t, int64(1), count, "não deveria duplicar o documento pro mesmo order_id")

	var final dto.OrderDTO
	require.NoError(t, collection.FindOne(context.Background(), bson.M{"orderid": "order-3"}).Decode(&final))
	require.Equal(t, "CANCELLED", final.Status)
}

func TestUpdateOrderStatusByDeliverymanID_WrongDeliverymanID(t *testing.T) {
	cleanup := setupDeliveryIntegrationEnv(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/handshake", func(c *fiber.Ctx) error { return HandShakeDeliveryman(c) })
	app.Post("/status", func(c *fiber.Ctx) error { return UpdateOrderStatusByDeliverymanID(c, noopWS) })

	_, orderBody := sampleOrder("order-4")
	require.NoError(t, CreateSolicitation(string(orderBody), noopWS))

	handshakePayload := map[string]interface{}{
		"order_id":    "order-4",
		"deliveryman": map[string]interface{}{"id": int64(42), "status": "IN_ROUTE_COLECT"},
	}
	hBody, _ := json.Marshal(handshakePayload)
	hReq := httptest.NewRequest(http.MethodPost, "/handshake", bytes.NewReader(hBody))
	hReq.Header.Set("Content-Type", "application/json")
	_, err := app.Test(hReq, -1)
	require.NoError(t, err)

	wrongPayload := map[string]interface{}{
		"order_id":    "order-4",
		"deliveryman": map[string]interface{}{"id": int64(99), "status": "FINISHED"},
	}
	wrongBody, _ := json.Marshal(wrongPayload)
	wrongReq := httptest.NewRequest(http.MethodPost, "/status", bytes.NewReader(wrongBody))
	wrongReq.Header.Set("Content-Type", "application/json")
	wrongResp, err := app.Test(wrongReq, -1)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, wrongResp.StatusCode)

	collection := models.MongoDabase.Collection("solicitations")
	var stillAssigned dto.OrderDTO
	require.NoError(t, collection.FindOne(context.Background(), bson.M{"orderid": "order-4"}).Decode(&stillAssigned))
	require.Equal(t, "IN_ROUTE_COLECT", stillAssigned.DeliveryMan.Status,
		"status não deveria ter mudado, já que o ID do entregador não bateu")
}

func TestGetApprovedSolicitations_RespectsDistanceLimit(t *testing.T) {
	cleanup := setupDeliveryIntegrationEnv(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/solicitations", GetApprovedSolicitations)

	near, nearBody := sampleOrder("order-near")
	near.Establishment.Lat = -23.5505
	near.Establishment.Long = -46.6333
	nearBody, _ = json.Marshal(near)
	require.NoError(t, CreateSolicitation(string(nearBody), noopWS))

	far, farBody := sampleOrder("order-far")
	far.Establishment.Lat = -22.9068
	far.Establishment.Long = -43.1729
	farBody, _ = json.Marshal(far)
	require.NoError(t, CreateSolicitation(string(farBody), noopWS))

	req := httptest.NewRequest(http.MethodGet,
		"/solicitations?latitude=-23.5505&longitude=-46.6333&limitDistance=10", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	var results []dto.OrderDTO
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&results))
	require.Len(t, results, 1, "só a solicitação dentro do raio de 10km deveria voltar")
	require.Equal(t, "order-near", results[0].OrderId)
}
