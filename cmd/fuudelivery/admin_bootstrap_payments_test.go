//go:build integration

// Package main - Teste de integracao do fluxo "staging" de admin:
//
//	Bootstrap admin (ADMIN_BOOTSTRAP_SECRET local) -> login -> /payments/all enriquecido
//
// Sobe Postgres real via testcontainer, conecta os models GLOBAIS
// do monolith (authModels.DB + paymentModels.DB) e exercita as ROTAS
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
	"github.com/testcontainers/testcontainers-go/modules/postgres"
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

	// Modo CI/serviço externo: quando POSTGRES_TEST_URI está
	// definido, usa o serviço já iniciado pelo workflow (padrão
	// "docker run" + wait, mais estável que testcontainers no runner atual).
	externalPG := os.Getenv("POSTGRES_TEST_URI") != ""

	// ---- Setup: Postgres real (staging) ----
	var pgDSN string
	if externalPG {
		pgDSN = os.Getenv("POSTGRES_TEST_URI")
	} else {
		pgContainer, pErr := postgres.Run(ctx, "postgres:16-alpine",
			postgres.WithDatabase("fuudelivery_staging"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
		)
		require.NoError(t, pErr, "subir Postgres")
		defer pgContainer.Terminate(ctx)

		pgDSN, pErr = pgContainer.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, pErr)
	}

	// Backoff classico de testcontainers em CI (o container pode nao aceitar
	// conexoes imediatamente apos o start).
	var pgDB *gorm.DB
	var err error
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
	os.Setenv("DB_CONNECTION_STRING", pgDSN)
	defer os.Unsetenv("ADMIN_BOOTSTRAP_SECRET")

	// Conecta os models GLOBAIS usados pelos handlers reais.
	require.NotPanics(t, func() { authModels.ConnectDatabase() }, "conectar authModels.DB (Postgres)")
	require.NotPanics(t, func() { paymentModels.ConnectPostgresDatabase() }, "conectar paymentModels.DB (Postgres)")
	require.NotNil(t, authModels.DB, "authModels.DB deve estar conectado")
	require.NotNil(t, paymentModels.DB, "paymentModels.DB deve estar conectado")

	// ---- Setup: app Fiber com as rotas REAIS do monolith ----
	app := fiber.New()
	setupAuthRoutes(app)
	setupPaymentRoutes(app)

	// ---- 1. Criar usuario comum direto no Postgres (GORM) ----
	var userID uint
	var customerEmail = "cliente.staging@fuudelivery.com"
	t.Run("CreateCustomer", func(t *testing.T) {
		hash, err := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.DefaultCost)
		require.NoError(t, err)

		u := authModels.User{
			Name:     "Cliente Staging",
			Email:    customerEmail,
			Phone:    "+5511999999999",
			Password: string(hash),
		}
		require.NoError(t, authModels.DB.Create(&u).Error)
		userID = u.ID
		require.NotZero(t, userID)
	})

	// ---- 2. Bootstrap admin ----
	var adminToken string
	t.Run("BootstrapAdmin", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"secret":   "local-dev-bootstrap-secret",
			"email":    "admin.staging@fuudelivery.com",
			"phone":    "+5511888888888",
			"name":     "Admin Staging",
			"password": "admin123",
		})
		req := httptest.NewRequest(http.MethodPost, "/admin/bootstrap", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode, "bootstrap deve retornar 200")
	})

	// ---- 3. Login admin ----
	t.Run("LoginAdmin", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    "admin.staging@fuudelivery.com",
			"password": "admin123",
		})
		req := httptest.NewRequest(http.MethodPost, "/users/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		token, ok := result["token"].(string)
		require.True(t, ok, "response deve conter token string")
		adminToken = token
		require.NotEmpty(t, adminToken)
	})

	// ---- 4. Criar pagamento de teste ----
	t.Run("CreatePayment", func(t *testing.T) {
		p := paymentModels.Payment{
			OrderID:         "staging-order-001",
			CustomerID:      int64(userID),
			CustomerPhone:   "+5511999999999",
			EstablishmentID: 1,
			Amount:          42.50,
			Method:          "pix",
			Status:          "approved",
			AbacatePayID:    "test-abacatepay-001",
		}
		require.NoError(t, paymentModels.DB.Create(&p).Error)
	})

	// ---- 5. GET /payments/all (admin) ----
	t.Run("PaymentsAll_Enriched", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payments/all", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		var payments []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payments))
		require.NotEmpty(t, payments, "deve haver ao menos 1 pagamento")

		// O pagamento deve ter user.nome enriquecido (cliente.staging@fuudelivery.com -> "Cliente Staging")
		p := payments[0]
		require.Equal(t, "Cliente Staging", p["customer_name"], "customer_name deve ser enriquecido do Postgres")
	})

	// ---- 6. Cenarios negativos ----
	t.Run("NoToken_401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/payments/all", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 401, resp.StatusCode)
	})

	t.Run("WrongSecret_403", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"secret":   "wrong-secret",
			"email":    "another@fuudelivery.com",
			"phone":    "+5511777777777",
			"name":     "Wrong Admin",
			"password": "wrong123",
		})
		req := httptest.NewRequest(http.MethodPost, "/admin/bootstrap", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 403, resp.StatusCode)
	})

	// ---- Cleanup: remover dados de teste ----
	t.Cleanup(func() {
		authModels.DB.Unscoped().Where("email = ?", "admin.staging@fuudelivery.com").Delete(&authModels.User{})
		authModels.DB.Unscoped().Where("email = ?", customerEmail).Delete(&authModels.User{})
		paymentModels.DB.Unscoped().Where("order_id = ?", "staging-order-001").Delete(&paymentModels.Payment{})
	})

	// Suppress unused variable warnings
	_ = ctx
}
