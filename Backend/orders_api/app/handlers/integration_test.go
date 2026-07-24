//go:build integration

// Testes de integração do orders_api.
//
// Sobem containers reais (MongoDB + Postgres) via testcontainers-go e exercitam
// o fluxo completo: CreateOrder -> UpdateOrderStatus -> EarnPointsForOrder.
// Não usam mocks de banco — só a chamada HTTP para o auth_api (GetEstablishment /
// checkEstablishmentOpen) é substituída por um httptest.Server, porque esse é
// outro serviço, fora do escopo deste teste.
//
// Rodar com:
//
//	docker ps            (garantir que o Docker está de pé)
//	go test -tags=integration ./app/handlers/... -run TestOrderLifecycle -v
//
// Pré-requisito: adicionar ao go.mod (rodar com rede liberada p/ proxy.golang.org):
//
//	go get github.com/testcontainers/testcontainers-go
//	go get github.com/testcontainers/testcontainers-go/modules/mongodb
//	go get github.com/testcontainers/testcontainers-go/modules/postgres
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/carloshomar/vercardapio/orders_api/app/dto"
	"github.com/carloshomar/vercardapio/orders_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupIntegrationEnv sobe Mongo + Postgres reais em containers, popula
// models.MongoDabase e models.DB (as globais que os handlers usam hoje) e
// devolve uma func de cleanup. Chame no início de cada teste com `defer cleanup()`.
//
// TODO: os handlers usam variáveis globais (models.DB / models.MongoDabase).
// Isso funciona para testes sequenciais, mas testes de integração não devem
// rodar em paralelo (t.Parallel) enquanto isso não virar injeção de dependência.
func setupIntegrationEnv(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()

	// --- MongoDB (pedidos, push tokens) ---
	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err, "subir container do MongoDB")

	mongoURI, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)
	require.NoError(t, mongoClient.Ping(ctx, nil))

	models.MongoClient = mongoClient
	models.MongoDabase = mongoClient.Database("orders_test")

	// --- Postgres (fidelidade / cupons / produtos) ---
	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("orders_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err, "subir container do Postgres")

	pgDSN, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgresdriver.Open(pgDSN), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(
		&models.LoyaltyPoints{},
		&models.LoyaltyTransaction{},
		&models.Coupon{},
		&models.CouponUsage{},
		&models.Category{},
		&models.Product{},
		&models.Order{},
	))
	models.DB = gormDB

	return func() {
		_ = mongoClient.Disconnect(ctx)
		_ = mongoContainer.Terminate(ctx)
		_ = pgContainer.Terminate(ctx)
	}
}

// fakeAuthAPI substitui o auth_api real: responde sempre "estabelecimento
// aberto" e devolve um dto.Establishment fixo. CreateOrder chama esses dois
// endpoints via http.Get usando as env vars URL_CHECK_ESTABLISHMENT_OPEN e
// URL_GET_ESTABLISHMENT_ID (ambas com um "%d" para o ID).
func fakeAuthAPI(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/is-open", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"is_open": true})
	})
	mux.HandleFunc("/establishment", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dto.Establishment{
			Id:   1,
			Name: "Restaurante Teste",
		})
	})

	srv := httptest.NewServer(mux)
	os.Setenv("URL_CHECK_ESTABLISHMENT_OPEN", srv.URL+"/is-open?id=%d")
	os.Setenv("URL_GET_ESTABLISHMENT_ID", srv.URL+"/establishment?id=%d")
	return srv
}

// noopWS simula o callback de WebSocket que os handlers usam para notificar
// clientes conectados. Nos testes de integração não precisamos de socket de
// verdade, só que a função não quebre o fluxo.
func noopWS(clientID int64, message []byte) error { return nil }

// TestOrderLifecycle cobre o caminho feliz completo:
//  1. Cliente cria um pedido               -> CreateOrder
//  2. Estabelecimento aprova e finaliza     -> UpdateOrderStatus (APPROVED, depois DONE)
//  3. Pontos de fidelidade são creditados   -> EarnPointsForOrder
//
// TODO (casos que faltam além do caminho feliz):
//   - Pedido agendado (ScheduledAt no futuro) não deve checar se está aberto.
//   - Estabelecimento fechado deve retornar 400 e não deve gravar no Mongo.
//   - UpdateOrderStatus com ID inválido / inexistente -> 400 / 404.
//   - EarnPointsForOrder com orderValue <= 0 não deve criar transação (hoje retorna nil sem erro).
//   - Cliente no tier "ouro" deve ganhar pontos em dobro (getPointsMultiplier).
//   - DONE deve gerar pickup_code de 6 dígitos único (ver pickup_code_test.go já existente).
func TestOrderLifecycle(t *testing.T) {
	cleanup := setupIntegrationEnv(t)
	defer cleanup()

	authSrv := fakeAuthAPI(t)
	defer authSrv.Close()

	app := fiber.New()
	app.Post("/orders", func(c *fiber.Ctx) error { return CreateOrder(c, noopWS) })
	app.Put("/orders/status", func(c *fiber.Ctx) error { return UpdateOrderStatus(c, noopWS) })

	// 1. Cria o pedido
	orderPayload := dto.RequestPayload{
		EstablishmentId: 1,
		DeliveryValue:   5.90,
		User: dto.User{
			Phone: "+5511999999999",
		},
		Cart: []dto.CartItem{
			{ID: "1", Quantity: 2, Item: dto.Item{Name: "X-Burger", Price: 25.00}},
		},
	}
	body, _ := json.Marshal(orderPayload)

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "criação do pedido deveria retornar 200")

	var createResp struct {
		OrderId string `json:"orderId"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&createResp))
	require.NotEmpty(t, createResp.OrderId)

	// TODO: confirmar direto no Mongo que o documento foi persistido com
	// status "AWAIT_APPROVE" antes de seguir pro próximo passo.

	// 2. Atualiza status para DONE (dispara geração de pickup_code)
	updatePayload := map[string]string{"id": createResp.OrderId, "status": "DONE"}
	updateBody, _ := json.Marshal(updatePayload)

	updateReq := httptest.NewRequest(http.MethodPut, "/orders/status", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, updateResp.StatusCode)

	// TODO: confirmar no Mongo que pickup_code foi gravado e tem 6 chars.

	// 3. Credita pontos de fidelidade para o telefone do cliente
	err = EarnPointsForOrder(orderPayload.User.Phone, createResp.OrderId, 55.90)
	require.NoError(t, err)

	var loyalty models.LoyaltyPoints
	require.NoError(t, models.DB.Where("user_phone = ?", orderPayload.User.Phone).First(&loyalty).Error)
	require.Equal(t, 55, loyalty.Points, "55.90 arredondado pra baixo * multiplicador bronze (1x)")
	require.Equal(t, 1, loyalty.TotalOrders)
	require.Equal(t, "bronze", loyalty.Tier)

	var transactions []models.LoyaltyTransaction
	require.NoError(t, models.DB.Where("order_id = ?", createResp.OrderId).Find(&transactions).Error)
	require.Len(t, transactions, 1)
	require.Equal(t, "earn", transactions[0].Type)
}
