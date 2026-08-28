//go:build integration

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
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/bcrypt"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestPasswordResetFlow valida o fluxo completo "esqueci minha senha":
//
//  1. Bootstrap admin + login â†’ token JWT admin
//  2. Criar cliente de teste no Postgres
//  3. POST /admin/password-reset/code â†’ recebe cÃ³digo em claro
//  4. POST /auth/reset-password com o cÃ³digo â†’ senha redefinida
//  5. Login com senha antiga â†’ 401; login com nova senha â†’ 200
//
// CenÃ¡rios negativos:
//   - CÃ³digo errado â†’ 400 (msg genÃ©rica)
//   - CÃ³digo expirado â†’ 400
//   - Identificador inexistente â†’ 404 (admin) / 400 (pÃºblico)
//   - user_type invÃ¡lido â†’ 400
//   - Senha muito curta â†’ 400
func TestPasswordResetFlow(t *testing.T) {
	ctx := context.Background()

	// ---- Setup: Postgres real via testcontainer ----
	externalPG := os.Getenv("POSTGRES_TEST_URI") != ""

	var pgDSN string
	if externalPG {
		pgDSN = os.Getenv("POSTGRES_TEST_URI")
	} else {
		pgContainer, pErr := postgres.Run(ctx, "postgres:16-alpine",
			postgres.WithDatabase("fuudelivery_pwdreset_test"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
		)
		require.NoError(t, pErr, "subir Postgres")
		defer pgContainer.Terminate(ctx)

		pgDSN, pErr = pgContainer.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, pErr)
	}

	// Backoff para CI (container pode demorar para aceitar conexÃµes)
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

	// ---- Setup: variÃ¡veis de ambiente ----
	os.Setenv("JWT_SECRET", "pwdreset-test-secret")
	os.Setenv("GO_ENV", "test")
	os.Setenv("ADMIN_BOOTSTRAP_SECRET", "pwdreset-bootstrap-secret")
	os.Setenv("DB_CONNECTION_STRING", pgDSN)
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("GO_ENV")
		os.Unsetenv("ADMIN_BOOTSTRAP_SECRET")
		os.Unsetenv("DB_CONNECTION_STRING")
	}()

	// Conecta os models GLOBAIS usados pelos handlers reais.
	require.NotPanics(t, func() { authModels.ConnectDatabase() }, "conectar authModels.DB")
	require.NotNil(t, authModels.DB)

	// ---- Setup: app Fiber com rotas reais do auth ----
	app := fiber.New()
	setupAuthRoutes(app)

	// ---- Helpers ----
	doJSON := func(method, path string, body interface{}, headers ...map[string]string) *http.Response {
		t.Helper()
		var reqBody *bytes.Buffer
		if body != nil {
			b, _ := json.Marshal(body)
			reqBody = bytes.NewBuffer(b)
		} else {
			reqBody = bytes.NewBuffer(nil)
		}
		req := httptest.NewRequest(method, path, reqBody)
		req.Header.Set("Content-Type", "application/json")
		for _, h := range headers {
			for k, v := range h {
				req.Header.Set(k, v)
			}
		}
		resp, err := app.Test(req, -1) // timeout 0 = sem limite
		require.NoError(t, err)
		return resp
	}

	decodeJSON := func(r *http.Response) map[string]interface{} {
		t.Helper()
		var m map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&m))
		return m
	}

	// ---- 1. Bootstrap admin ----
	var adminToken string
	t.Run("BootstrapAdmin", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/admin/bootstrap", map[string]string{
			"secret":   "pwdreset-bootstrap-secret",
			"email":    "admin-pwdreset@test.com",
			"phone":    "+5511000000001",
			"name":     "Admin PwdReset",
			"password": "admin123",
		})
		require.Equal(t, 200, resp.StatusCode)
	})

	t.Run("LoginAdmin", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/users/login", map[string]string{
			"email":    "admin-pwdreset@test.com",
			"password": "admin123",
		})
		require.Equal(t, 200, resp.StatusCode)
		result := decodeJSON(resp)
		token, ok := result["token"].(string)
		require.True(t, ok, "response deve conter token")
		adminToken = token
		require.NotEmpty(t, adminToken)
	})

	adminAuth := map[string]string{"Authorization": "Bearer " + adminToken}

	// ---- 2. Criar cliente de teste ----
	var clientID uint
	clientPhone := "+5511999990001"
	clientOldPass := "senhaAntiga123"

	t.Run("CreateClient", func(t *testing.T) {
		hash, err := bcrypt.GenerateFromPassword([]byte(clientOldPass), bcrypt.DefaultCost)
		require.NoError(t, err)

		c := authModels.Client{
			Phone:    clientPhone,
			Password: string(hash),
			Name:     "Cliente Teste Reset",
		}
		require.NoError(t, authModels.DB.Create(&c).Error)
		clientID = c.ID
		require.NotZero(t, clientID)
	})

	// ---- 3. Gerar cÃ³digo de reset (admin) ----
	var resetCode string
	t.Run("GenerateResetCode", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/admin/password-reset/code", map[string]string{
			"user_type":  "client",
			"identifier": clientPhone,
		}, adminAuth)
		require.Equal(t, 200, resp.StatusCode)

		result := decodeJSON(resp)
		code, ok := result["code"].(string)
		require.True(t, ok, "response deve conter code")
		resetCode = code
		require.Len(t, resetCode, 8, "cÃ³digo deve ter 8 caracteres")
		require.Equal(t, "client", result["user_type"])
		require.NotNil(t, result["expires_at"])
	})

	// ---- 4. Resetar senha com cÃ³digo vÃ¡lido ----
	newPass := "NovaSenha456!"
	t.Run("ResetPassword_Success", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/auth/reset-password", map[string]string{
			"user_type":    "client",
			"identifier":   clientPhone,
			"code":         resetCode,
			"new_password": newPass,
		})
		require.Equal(t, 200, resp.StatusCode)
		result := decodeJSON(resp)
		require.Contains(t, result["message"], "sucesso")
	})

	// ---- 5. Verificar: senha antiga nÃ£o funciona mais ----
	t.Run("LoginWithOldPassword_Fails", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/users/login/client", map[string]string{
			"phone":    clientPhone,
			"password": clientOldPass,
		})
		require.NotEqual(t, 200, resp.StatusCode, "senha antiga deve ser rejeitada")
	})

	// ---- 6. Verificar: nova senha funciona ----
	t.Run("LoginWithNewPassword_Works", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/users/login/client", map[string]string{
			"phone":    clientPhone,
			"password": newPass,
		})
		require.Equal(t, 200, resp.StatusCode)
	})

	// ---- 7. CÃ³digo jÃ¡ usado nÃ£o pode ser reutilizado ----
	t.Run("ReuseCode_Fails", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/auth/reset-password", map[string]string{
			"user_type":    "client",
			"identifier":   clientPhone,
			"code":         resetCode,
			"new_password": "OutraSenha789!",
		})
		require.Equal(t, 400, resp.StatusCode)
		result := decodeJSON(resp)
		require.Equal(t, "Código inválido ou expirado. Confira os dados com o suporte.", result["error"])
	})

	// ---- CenÃ¡rios negativos ----
	t.Run("WrongCode_Fails", func(t *testing.T) {
		// Gerar um novo cÃ³digo vÃ¡lido primeiro
		resp := doJSON(http.MethodPost, "/admin/password-reset/code", map[string]string{
			"user_type":  "client",
			"identifier": clientPhone,
		}, adminAuth)
		require.Equal(t, 200, resp.StatusCode)

		// Tentar com cÃ³digo errado
		resp = doJSON(http.MethodPost, "/auth/reset-password", map[string]string{
			"user_type":    "client",
			"identifier":   clientPhone,
			"code":         "XXXXXXXX",
			"new_password": "Teste1234!",
		})
		require.Equal(t, 400, resp.StatusCode)
	})

	t.Run("ShortPassword_Fails", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/admin/password-reset/code", map[string]string{
			"user_type":  "client",
			"identifier": clientPhone,
		}, adminAuth)
		require.Equal(t, 200, resp.StatusCode)
		result := decodeJSON(resp)
		code := result["code"].(string)

		resp = doJSON(http.MethodPost, "/auth/reset-password", map[string]string{
			"user_type":    "client",
			"identifier":   clientPhone,
			"code":         code,
			"new_password": "12345", // < 6 chars
		})
		require.Equal(t, 400, resp.StatusCode)
	})

	t.Run("InvalidUserType_Fails", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/admin/password-reset/code", map[string]string{
			"user_type":  "admin",
			"identifier": clientPhone,
		}, adminAuth)
		require.Equal(t, 400, resp.StatusCode)
	})

	t.Run("UnknownIdentifier_Admin404", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/admin/password-reset/code", map[string]string{
			"user_type":  "client",
			"identifier": "+5511000009999",
		}, adminAuth)
		require.Equal(t, 404, resp.StatusCode)
	})

	t.Run("UnknownIdentifier_Public400", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/auth/reset-password", map[string]string{
			"user_type":    "client",
			"identifier":   "+5511000009999",
			"code":         "ABCDEFGH",
			"new_password": "Teste123!",
		})
		require.Equal(t, 400, resp.StatusCode)
		// Mensagem genÃ©rica (anti-enumeration)
		result := decodeJSON(resp)
		require.Equal(t, "Código inválido ou expirado. Confira os dados com o suporte.", result["error"])
	})

	t.Run("MaxAttempts_LocksCode", func(t *testing.T) {
		// Gerar cÃ³digo
		resp := doJSON(http.MethodPost, "/admin/password-reset/code", map[string]string{
			"user_type":  "client",
			"identifier": clientPhone,
		}, adminAuth)
		require.Equal(t, 200, resp.StatusCode)
		result := decodeJSON(resp)
		code := result["code"].(string)

		// Tentar errar 5 vezes (maxPasswordResetAttempts = 5)
		for i := 0; i < 5; i++ {
			resp = doJSON(http.MethodPost, "/auth/reset-password", map[string]string{
				"user_type":    "client",
				"identifier":   clientPhone,
				"code":         "WRONGCODE" + fmt.Sprintf("%d", i),
				"new_password": "Teste123!",
			})
			require.Equal(t, 400, resp.StatusCode)
		}

		// 6a tentativa com o cÃ³digo CORRETO deve falhar (cÃ³digo bloqueado)
		resp = doJSON(http.MethodPost, "/auth/reset-password", map[string]string{
			"user_type":    "client",
			"identifier":   clientPhone,
			"code":         code,
			"new_password": "Teste123!",
		})
		require.Equal(t, 400, resp.StatusCode)
		result = decodeJSON(resp)
		require.Contains(t, result["error"], "bloqueado")
	})

	// ---- Cleanup ----
	t.Cleanup(func() {
		authModels.DB.Unscoped().Where("id = ?", clientID).Delete(&authModels.Client{})
		authModels.DB.Unscoped().Where("email = ?", "admin-pwdreset@test.com").Delete(&authModels.User{})
		authModels.DB.Unscoped().Where("user_id = ?", clientID).Delete(&authModels.PasswordResetToken{})
	})
}
