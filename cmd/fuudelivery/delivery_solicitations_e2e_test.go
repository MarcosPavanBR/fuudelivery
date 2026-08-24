//go:build integration

package main

// Teste E2E do delivery_api com dual-write: Postgres (delivery_solicitations)
// é o primário e Mongo (collection "solicitations") o espelho durante o corte.
// Prova que:
//   1. CreateSolicitation grava nos DOIS bancos
//   2. Handshake atribui o entregador nos DOIS
//   3. Atualização de status (FINISHED) reflete nos DOIS
//   4. Atualização do status do pedido preserva o deliveryman (upsert)
//   5. Leituras voltam do Postgres (primário) e o extrato funciona
//
// Rodar com:
//   docker ps
//   go test -tags=integration -v -run TestDeliverySolicitationsDualWrite ./cmd/fuudelivery/

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
	deliveryHandlers "github.com/carloshomar/fuudelivery/delivery_api/app/handlers"
	deliveryModels "github.com/carloshomar/fuudelivery/delivery_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func noopWSE2E(_ int64, _ []byte) error { return nil }

func TestDeliverySolicitationsDualWrite(t *testing.T) {
	ctx := context.Background()

	// ---- Mongo + Postgres reais via testcontainers ----
	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err, "subir MongoDB")
	defer mongoContainer.Terminate(ctx)

	mongoURI, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)
	require.NoError(t, mongoClient.Ping(ctx, nil))
	defer mongoClient.Disconnect(ctx)

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("fuudelivery_delivery"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err, "subir Postgres")
	defer pgContainer.Terminate(ctx)

	pgDSN, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Container pode demorar a aceitar conexões (race clássico em CI)
	var pgDB *gorm.DB
	for attempt := 0; attempt < 10; attempt++ {
		pgDB, err = gorm.Open(postgresdriver.Open(pgDSN), &gorm.Config{})
		if err == nil {
			var ping int
			if pingErr := pgDB.Raw("SELECT 1").Scan(&ping).Error; pingErr == nil && ping == 1 {
				break
			}
			err = fmt.Errorf("postgres nao respondeu ao ping")
		}
		time.Sleep(2 * time.Second)
	}
	require.NoError(t, err, "conectar ao Postgres do testcontainer")

	// Cria a tabela do delivery (espelho do sql/02_dominio_entrega.sql)
	require.NoError(t, pgDB.AutoMigrate(&deliveryModels.DeliverySolicitation{}))

	// ---- Injeta nos globais do delivery_api (restaura no cleanup) ----
	oldMongoClient, oldMongoDB, oldPG := deliveryModels.MongoClient, deliveryModels.MongoDabase, deliveryModels.DB
	deliveryModels.MongoClient = mongoClient
	deliveryModels.MongoDabase = mongoClient.Database("delivery_e2e")
	deliveryModels.InitPostgres(pgDB)
	t.Cleanup(func() {
		deliveryModels.MongoClient = oldMongoClient
		deliveryModels.MongoDabase = oldMongoDB
		deliveryModels.InitPostgres(oldPG)
	})

	orderID := "order-dual-write"
	order := dto.OrderDTO{
		OrderId: orderID,
		Status:  "APPROVED",
		Establishment: dto.EstablishmentDTO{
			Id: 7, Name: "Restaurante E2E", Lat: -23.5505, Long: -46.6333,
		},
		User:     dto.UserDTO{ID: 3, Name: "Cliente E2E", Phone: "+5511999999999"},
		Products: []dto.ProductDTO{{Id: 1, Name: "X-Tudo", Quantity: 2, Price: 24.9}},
		Total:    49.8,
		Payment:  dto.PaymentDTO{Method: "PIX"},
	}
	body, _ := json.Marshal(order)
	require.NoError(t, deliveryHandlers.CreateSolicitation(string(body), noopWSE2E))

	coll := deliveryModels.MongoDabase.Collection("solicitations")

	// 1) CreateSolicitation gravou nos DOIS bancos
	var pgRec deliveryModels.DeliverySolicitation
	require.NoError(t, pgDB.Where("order_id = ?", orderID).First(&pgRec).Error)
	require.Equal(t, "APPROVED", pgRec.Status)
	require.Equal(t, int64(7), pgRec.EstablishmentID)
	require.Equal(t, "Restaurante E2E", pgRec.EstablishmentName)
	require.Equal(t, 49.8, pgRec.Total)
	require.Equal(t, "PIX", pgRec.PaymentMethod)
	require.Nil(t, pgRec.DeliveryManID, "recém-criada não tem entregador")

	var mongoDoc dto.OrderDTO
	require.NoError(t, coll.FindOne(ctx, bson.M{"orderid": orderID}).Decode(&mongoDoc))
	require.Equal(t, "APPROVED", mongoDoc.Status)

	// 2) Handshake → entrega o pedido nos DOIS
	app := fiber.New()
	app.Post("/handshake", func(c *fiber.Ctx) error { return deliveryHandlers.HandShakeDeliveryman(c) })
	app.Post("/status", func(c *fiber.Ctx) error { return deliveryHandlers.UpdateOrderStatusByDeliverymanID(c, noopWSE2E) })
	app.Get("/deliveryman/:id/extrato", deliveryHandlers.GetExtrato)

	handshake := map[string]interface{}{
		"order_id": orderID,
		"deliveryman": map[string]interface{}{
			"id": int64(42), "name": "Entregador E2E", "status": "IN_ROUTE_COLECT",
		},
	}
	hsBody, _ := json.Marshal(handshake)
	hsReq := httptest.NewRequest(http.MethodPost, "/handshake", bytes.NewReader(hsBody))
	hsReq.Header.Set("Content-Type", "application/json")
	hsResp, err := app.Test(hsReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, hsResp.StatusCode)

	require.NoError(t, pgDB.Where("order_id = ?", orderID).First(&pgRec).Error)
	require.NotNil(t, pgRec.DeliveryManID)
	require.Equal(t, int64(42), *pgRec.DeliveryManID)
	require.Equal(t, "IN_ROUTE_COLECT", pgRec.DeliveryManStatus)

	require.NoError(t, coll.FindOne(ctx, bson.M{"orderid": orderID}).Decode(&mongoDoc))
	require.Equal(t, int64(42), mongoDoc.DeliveryMan.Id)
	require.Equal(t, "IN_ROUTE_COLECT", mongoDoc.DeliveryMan.Status)

	// 3) Status do entregador → FINISHED nos DOIS
	status := map[string]interface{}{
		"order_id":    orderID,
		"deliveryman": map[string]interface{}{"id": int64(42), "status": "FINISHED"},
	}
	stBody, _ := json.Marshal(status)
	stReq := httptest.NewRequest(http.MethodPost, "/status", bytes.NewReader(stBody))
	stReq.Header.Set("Content-Type", "application/json")
	stResp, err := app.Test(stReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, stResp.StatusCode)

	require.NoError(t, pgDB.Where("order_id = ?", orderID).First(&pgRec).Error)
	require.Equal(t, "FINISHED", pgRec.DeliveryManStatus)
	require.NoError(t, coll.FindOne(ctx, bson.M{"orderid": orderID}).Decode(&mongoDoc))
	require.Equal(t, "FINISHED", mongoDoc.DeliveryMan.Status)

	// 4) Upsert do status do PEDIDO (CreateSolicitation de novo) preserva o
	//    deliveryman e atualiza o status nos DOIS
	finishedOrder := order
	finishedOrder.Status = "FINISHED"
	fBody, _ := json.Marshal(finishedOrder)
	require.NoError(t, deliveryHandlers.CreateSolicitation(string(fBody), noopWSE2E))

	require.NoError(t, pgDB.Where("order_id = ?", orderID).First(&pgRec).Error)
	require.Equal(t, "FINISHED", pgRec.Status, "status do pedido atualizado no PG")
	require.NotNil(t, pgRec.DeliveryManID)
	require.Equal(t, int64(42), *pgRec.DeliveryManID, "deliveryman preservado no PG")
	require.NoError(t, coll.FindOne(ctx, bson.M{"orderid": orderID}).Decode(&mongoDoc))
	require.Equal(t, "FINISHED", mongoDoc.Status, "status do pedido atualizado no Mongo")
	require.Equal(t, int64(42), mongoDoc.DeliveryMan.Id, "deliveryman preservado no Mongo")

	// 5) Leitura primária do Postgres: extrato do entregador
	exReq := httptest.NewRequest(http.MethodGet, "/deliveryman/42/extrato", nil)
	exResp, err := app.Test(exReq, -1)
	require.NoError(t, err)
	var extrato []dto.OrderDTO
	require.NoError(t, json.NewDecoder(exResp.Body).Decode(&extrato))
	require.Len(t, extrato, 1, "pedido FINISHED deve aparecer no extrato")
	require.Equal(t, orderID, extrato[0].OrderId)

	// 6) GetDeliveryManIDByOrderID (autorização do chat) lê do PG
	dmID, err := deliveryModels.GetDeliveryManIDByOrderID(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, int64(42), dmID)
}
