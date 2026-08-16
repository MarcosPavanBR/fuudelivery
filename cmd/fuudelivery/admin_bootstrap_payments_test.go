//go:build integration

// Package main - Teste de integracao do fluxo "staging" de admin:
//
//	Bootstrap admin (ADMIN_BOOTSTRAP_SECRET local) -> login -> /payments/all enriquecido
//
// Sobe MongoDB + Postgres reais via testcontainers, conecta os models GLOBAIS
// do monolith (authModels.DB + paymentModels.MongoDabase) e exercita as ROTAS
// REAIS registradas pelo main.go (setupAuthRoutes + setupPaymentRoutes) — o
// mesmo codigo que roda em producao, sem tocar no ambiente real.
//
// Rodar com:
//
//	docker ps                                  (verificar se Docker esta de pe)
//	go test -tags=integration -v -run TestAdminBootstrap ./cmd/fuudelivery/
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	authModels "github.com/carloshomar/fuudelivery/auth_api/app/models"
	paymentModels "github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestAdminBootstrapPaymentsAll valida o fluxo completo de admin em ambiente
// isolado ("staging"):
//
//  1. Bootstrap: /admin/bootstrap promove um usuario comum a admin usando o
//     ADMIN_BOOTSTRAP_SECRET definido localmente (variavel de ambiente).
//  2. Auth: /users/login gera o token JWT do admin promovido.
//  3. Enriquecimento: /payments/all (admin) retorna user.nome lido do Postgres
//     (tabela users) para o customer_id do pagamento.
//
// Cenarios negativos: secret errado -> 403; sem token -> 401; token de
// usuario comum -> 403; usuario inexistente no Postgres -> campo user omitido.
func TestAdminBootstrapPaymentsAll(t *testing.T) {
	ctx := context.Background()

	// ---- Setup: MongoDB + Postgres reais (staging) ----
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
		postgres.WithDatabase("fuudelivery_staging"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	require.NoError(t, err, "subir Postgres")
	defer pgContainer.Terminate(ctx)

	pgDSN, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Backoff classico de testcontainers em CI (o container pode nao aceitar
	// conexoes imediatamente apos o start).
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

	// ---- Setup: variaveis de ambiente (secret LOCAL de bootstrap) ----
	os.Setenv("JWT_SECRET", "staging-secret-para-teste")
	os.Setenv("GO_ENV", "test")
	os.Setenv("ADMIN_BOOTSTRAP_SECRET", "local-dev-bootstrap-secret")
	os.Setenv("MONGO_URI", mongoURI)
	os.Setenv("PAYMENT_MONGO_DATABASE", "fuudelivery_staging_payments")
	os.Setenv("DB_CONNECTION_STRING", pgDSN)
	defer os.Unsetenv("ADMIN_BOOTSTRAP_SECRET")

	// Conecta os models GLOBAIS usados pelos handlers reais.
	// ConnectDatabase() faz AutoMigrate + cria a zona padrao (5/85).
	require.NotPanics(t, func() { authModels.ConnectDatabase() }, "conectar authModels.DB (Postgres)")
	require.NotPanics(t, func() { paymentModels.ConnectMongoDatabase() }, "conectar paymentModels (Mongo)")
	require.NotNil(t, authModels.DB, "authModels.DB deve estar conectado")
	require.NotNil(t, paymentModels.MongoDabase, "paymentModels.MongoDabase deve estar conectado")

	// ---- Setup: app Fiber com as rotas REAIS do monolith ----
	app := fiber.New()
	setupAuthRoutes(app)
	setupPaymentRoutes(app)

	// ---- 1. Criar usuario comum direto no Postgres (GORM) ----
	// NOTA: nao usamos /users/register aqui: o handler CreateUser insere com
	// colunas do schema legado ("createdAt"/"updatedAt" camelCase) que o
	// AutoMigrate do model atual nao cria — em um Postgres limpo o registro
	// retornaria 500. Criar via GORM mantem o foco no fluxo sob teste
	// (bootstrap + login + /payments/all enriquecido).
	var userID uint
	var customerEmail = "cliente.staging@fuudelivery.com"
	t.Run("CreateCustomer", func(t *testing.T) {
		hash, err := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.DefaultCost)
		require.NoError(t, err)

		u := authModels.User{
			Name:     "Cliente Staging",
			Email:    customerEmail,
			Password: string(hash),
			Role:     "user",
		}
		require.NoError(t, authModels.DB.Create(&u).Error, "criar usuario no Postgres")
		userID = u.ID
		require.NotZero(t, userID)
	})

	// ---- 2. Bootstrap com secret ERRADO -> 403 ----
	t.Run("BootstrapWrongSecret", func(t *testing.T) {
		payload := map[string]string{
			"email":  customerEmail,
			"secret": "secret-errado",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/admin/bootstrap", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 403, resp.StatusCode, "secret errado deve ser rejeitado")
	})

	// ---- 3. Bootstrap com secret LOCAL correto -> promove a admin ----
	t.Run("BootstrapAdmin", func(t *testing.T) {
		payload := map[string]string{
			"email":  customerEmail,
			"secret": "local-dev-bootstrap-secret",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/admin/bootstrap", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode, "bootstrap com secret local")

		var u authModels.User
		require.NoError(t, authModels.DB.Where("email = ?", customerEmail).First(&u).Error)
		require.Equal(t, "admin", u.Role, "usuario promovido a admin no Postgres")
	})

	// ---- 4. Login real -> token JWT do admin ----
	var adminToken string
	t.Run("LoginAdmin", func(t *testing.T) {
		payload := map[string]string{
			"email":    customerEmail,
			"password": "senha123",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/users/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode, "login do admin")

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		adminToken, _ = result["token"].(string)
		require.NotEmpty(t, adminToken, "token JWT")
	})

	// ---- 5. Seed de pagamentos no MongoDB (banco de payments) ----
	// Um pagamento com customer_id existente no Postgres (deve virar user.nome)
	// e outro com customer_id inexistente (deve ficar sem user).
	paymentsCol := paymentModels.MongoDabase.Collection("payments")
	t.Run("SeedPayments", func(t *testing.T) {
		now := time.Now()
		_, err := paymentsCol.InsertOne(ctx, bson.M{
			"_id":            primitive.NewObjectID(),
			"order_id":       "order-staging-001",
			"customer_id":    int64(userID),
			"customer_phone": "11999990001",
			"amount":         8500,
			"status":         "CONFIRMED",
			"method":         "pix",
			"created_at":     now,
		})
		require.NoError(t, err, "inserir pagamento com customer conhecido")

		_, err = paymentsCol.InsertOne(ctx, bson.M{
			"_id":            primitive.NewObjectID(),
			"order_id":       "order-staging-002",
			"customer_id":    int64(999999),
			"customer_phone": "11999990002",
			"amount":         2500,
			"status":         "PENDING",
			"method":         "pix",
			"created_at":     now.Add(time.Second),
		})
		require.NoError(t, err, "inserir pagamento com customer inexistente")
	})

	// ---- 6. /payments/all sem token -> 401 ----
	t.Run("PaymentsAllNoToken", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payments/all", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 401, resp.StatusCode, "adminRequired sem token")
	})

	// ---- 7. /payments/all com token de admin -> user.nome enriquecido ----
	t.Run("PaymentsAllEnriched", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payments/all", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode, "listagem admin")

		var payments []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&payments)
		require.Len(t, payments, 2, "dois pagamentos semeados")

		var withUser, withoutUser map[string]interface{}
		for _, p := range payments {
			if p["order_id"] == "order-staging-001" {
				withUser = p
			} else {
				withoutUser = p
			}
		}
		require.NotNil(t, withUser, "pagamento com customer conhecido presente")

		u, ok := withUser["user"].(map[string]interface{})
		require.True(t, ok, "campo user presente para customer_id existente no Postgres")
		require.Equal(t, "Cliente Staging", u["nome"], "user.nome lido do Postgres")
		require.Equal(t, float64(userID), u["id"], "user.id = id do Postgres")

		// O pagamento com customer_id inexistente NÃO deve ter user (fallback).
		require.NotNil(t, withoutUser, "pagamento com customer inexistente presente")
		_, hasUser := withoutUser["user"]
		require.False(t, hasUser, "campo user omitido quando o usuario nao existe no Postgres")
	})

	// ---- 8. /payments/all com token de usuario COMUM -> 403 ----
	t.Run("PaymentsAllNonAdmin", func(t *testing.T) {
		// Cria um segundo usuario comum (sem bootstrap) e loga.
		commonEmail := "comum.staging@fuudelivery.com"
		hash, err := bcrypt.GenerateFromPassword([]byte("senha456"), bcrypt.DefaultCost)
		require.NoError(t, err)
		u := authModels.User{
			Name:     "Usuario Comum",
			Email:    commonEmail,
			Password: string(hash),
			Role:     "user",
		}
		require.NoError(t, authModels.DB.Create(&u).Error, "criar usuario comum")

		loginBody, _ := json.Marshal(map[string]string{
			"email":    commonEmail,
			"password": "senha456",
		})
		loginReq := httptest.NewRequest(http.MethodPost, "/users/login", bytes.NewReader(loginBody))
		loginReq.Header.Set("Content-Type", "application/json")
		loginResp, err := app.Test(loginReq, -1)
		require.NoError(t, err)
		require.Equal(t, 200, loginResp.StatusCode)

		var result map[string]interface{}
		json.NewDecoder(loginResp.Body).Decode(&result)
		commonToken, _ := result["token"].(string)
		require.NotEmpty(t, commonToken)

		allReq := httptest.NewRequest(http.MethodGet, "/payments/all", nil)
		allReq.Header.Set("Authorization", "Bearer "+commonToken)
		allResp, err := app.Test(allReq, -1)
		require.NoError(t, err)
		require.Equal(t, 403, allResp.StatusCode, "usuario comum nao acessa /payments/all")
	})

	t.Log("=== Fluxo de admin (bootstrap local + /payments/all enriquecido) validado no staging ===")
}
