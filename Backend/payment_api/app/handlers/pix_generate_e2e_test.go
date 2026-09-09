//go:build integration

// Package handlers — pix_generate_e2e_test.go
//
// Teste E2E do GeneratePIX AGORA pelo router (item 1.7 do roadmap de split):
// POST /payments/pix/generate -> PaymentRouter -> adapter abacatepay ->
// API v2 do AbacatePay (mockada em httptest).
//
// Por que integração e não unitário: o handler consulta order_documents no
// Postgres (validateChargeAmount/resolveDeliveryAmount/bindRecipientToOrder),
// persiste o pagamento no Postgres e o caminho do adapter depende do envelope
// v2 da API real. É exatamente este teste que teria pegado o adapter apontando
// para o endpoint/contrato errado antes de virar regressão em produção.
//
// Como rodar:
//
//	go test -tags=integration -run 'TestGeneratePIX_RouterE2E' ./app/handlers/
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/carloshomar/fuudelivery/pkg/gateway"
	abacatepay "github.com/carloshomar/fuudelivery/pkg/gateway/abacatepay"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// mockAbacateV2 simula a API v2 do AbacatePay nos dois endpoints que o
// monolito usa: criação (/transparents/create) e consulta (/transparents/check).
// O handler de criação valida o formato EXATO do corpo (method=PIX, data
// aninhado, amount em centavos) — é a prova de que o adapter fala o contrato.
func mockAbacateV2(t *testing.T, createCalls, checkCalls *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/transparents/create":
			atomic.AddInt32(createCalls, 1)

			var body struct {
				Method string `json:"method"`
				Data   struct {
					Amount      int64  `json:"amount"`
					Description string `json:"description"`
					ExternalID  string `json:"externalId"`
				} `json:"data"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

			// Contrato v2: method=PIX, data aninhado, amount em CENTAVOS.
			require.Equal(t, "PIX", body.Method, "corpo v2 precisa method=PIX")
			require.Equal(t, int64(8990), body.Data.Amount,
				"amount precisa chegar em centavos (R$ 89,90 = 8990)")
			require.Equal(t, "order-e2e-router-001", body.Data.ExternalID)

			// Resposta v2 real: envelope + brCode/brCodeBase64.
			resp := fmt.Sprintf(`{"success":true,"data":{"id":"charge-router-e2e-001","amount":%d,"status":"waiting","brCode":"0002012658BR.GOV.BCB.PIXROUTERE2E","brCodeBase64":"data:image/png;base64,aVFST0ZJSk1RPT0=","expiresAt":"2026-12-01T12:00:00Z","createdAt":"2026-12-01T11:30:00Z"},"error":null}`, body.Data.Amount)
			_, _ = w.Write([]byte(resp))

		case "/transparents/check":
			atomic.AddInt32(checkCalls, 1)
			chargeID := r.URL.Query().Get("id")
			_, _ = fmt.Fprintf(w, `{"success":true,"data":{"id":%q,"status":"PAID","amount":8990,"expiresAt":"2026-12-01T12:00:00Z"},"error":null}`, chargeID)

		default:
			t.Errorf("path inesperado na API mockada: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":"not found"}`))
		}
	}))
}

// TestGeneratePIX_RouterE2E percorre o caminho completo do item 1.7:
// handler -> router -> adapter -> API v2 (mock) -> persistência -> resposta.
func TestGeneratePIX_RouterE2E(t *testing.T) {
	teardown := setupCheckoutE2EEnv(t)
	defer teardown()

	var createCalls, checkCalls int32
	mock := mockAbacateV2(t, &createCalls, &checkCalls)
	defer mock.Close()

	// O adapter lê a env na construção; aponta para o mock.
	t.Setenv("ABACATE_PAY_API_KEY", "e2e-api-key")
	t.Setenv("ABACATE_PAY_BASE_URL", mock.URL)

	// Pedido real no Postgres: total 89.90, frete 7.00, establishment 42,
	// cliente +5511988887777 — o que validateChargeAmount/
	// resolveDeliveryAmount/bindRecipientToOrder consultam.
	createOrderDocumentsTable(t)
	seedOrderDocument(t, "order-e2e-router-001", 89.90, 7.00)

	// Router real com o adapter real — é o fluxo de produção, só com a
	// URL da API apontando para o mock.
	abacateGW, err := abacatepay.NewGateway()
	require.NoError(t, err, "construir gateway abacatepay (com credencial fake)")
	router := gateway.NewRouter(abacateGW)
	router.SetStrategy(gateway.StrategyOrdered)

	app := fiber.New()
	app.Post("/payments/pix/generate", func(c *fiber.Ctx) error {
		c.Locals("payment_router", router)
		return GeneratePIX(c)
	})

	body := `{"order_id":"order-e2e-router-001","customer_id":777,"establishment_id":42,"amount":89.90,"delivery_amount":7.00,"method":"pix","customer_phone":"+5511988887777"}`
	req := httptest.NewRequest(http.MethodPost, "/payments/pix/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// bindRecipientToOrder lê customer_id do token; sem middleware de auth,
	// o token não existe e o customer_id do corpo passa (comportamento do
	// handler — o middleware real roda antes em produção).
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode, "GeneratePIX deve criar a cobrança via router")

	require.Equal(t, int32(1), atomic.LoadInt32(&createCalls),
		"a API do gateway deve ser chamada exatamente 1 vez (via router)")

	// Corpo da resposta: campos PIX no formato que apps/painéis consomem.
	var out struct {
		PaymentID    string `json:"payment_id"`
		Status       string `json:"status"`
		PixQRCode    string `json:"pix_qr_code"`
		PixCopyPaste string `json:"pix_copy_paste"`
		QRCodeBase64 string `json:"qr_code_base64"`
		AbacatePayID string `json:"abacatepay_id"`
		Message      string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.Equal(t, "PENDING", out.Status)
	require.Equal(t, "charge-router-e2e-001", out.AbacatePayID)
	require.Equal(t, "0002012658BR.GOV.BCB.PIXROUTERE2E", out.PixCopyPaste)
	require.Equal(t, "0002012658BR.GOV.BCB.PIXROUTERE2E", out.PixQRCode)
	// Base64 PURO (sem prefixo data:image) — o frontend renderiza direto.
	require.Equal(t, "aVFST0ZJSk1RPT0=", out.QRCodeBase64)
	require.NotContains(t, out.QRCodeBase64, "data:image", "base64 deve chegar puro ao frontend")
	require.Contains(t, out.Message, "abacatepay")

	// Persistência: pagamento gravado com os campos PIX + external ID.
	var stored models.Payment
	require.NoError(t, models.DB.Where("order_id = ?", "order-e2e-router-001").
		Order("created_at DESC").First(&stored).Error)
	require.Equal(t, "PENDING", stored.Status)
	require.Equal(t, "pix", stored.Method)
	require.Equal(t, 89.90, stored.Amount)
	require.Equal(t, 7.00, stored.DeliveryAmount)
	require.Equal(t, int64(42), stored.EstablishmentID)
	require.Equal(t, "+5511988887777", stored.CustomerPhone)
	require.Equal(t, "charge-router-e2e-001", stored.AbacatePayID,
		"webhook casa a cobrança por este ID — precisa ser o ID do gateway")
	require.Equal(t, "0002012658BR.GOV.BCB.PIXROUTERE2E", stored.PixCopyPaste)
	require.Equal(t, "aVFST0ZJSk1RPT0=", stored.QRCodeBase64)
	require.WithinDuration(t, time.Now(), stored.CreatedAt, time.Minute)
}

// TestGeneratePIX_SemPedidoProtege_contraAmountFalso garante que o caminho
// novo (router) NÃO afrouxou as proteções server-side de dinheiro.
func TestGeneratePIX_RouterE2E_AmountDivergente(t *testing.T) {
	teardown := setupCheckoutE2EEnv(t)
	defer teardown()

	var createCalls int32
	mock := mockAbacateV2(t, &createCalls, new(int32))
	defer mock.Close()

	t.Setenv("ABACATE_PAY_API_KEY", "e2e-api-key")
	t.Setenv("ABACATE_PAY_BASE_URL", mock.URL)

	createOrderDocumentsTable(t)
	seedOrderDocument(t, "order-e2e-router-002", 89.90, 7.00)

	abacateGW, err := abacatepay.NewGateway()
	require.NoError(t, err)
	router := gateway.NewRouter(abacateGW)

	app := fiber.New()
	app.Post("/payments/pix/generate", func(c *fiber.Ctx) error {
		c.Locals("payment_router", router)
		return GeneratePIX(c)
	})

	// Cliente tenta pagar R$ 0,01 num pedido de R$ 89,90.
	body := `{"order_id":"order-e2e-router-002","amount":0.01,"delivery_amount":7.00,"method":"pix"}`
	req := httptest.NewRequest(http.MethodPost, "/payments/pix/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
	require.Equal(t, int32(0), atomic.LoadInt32(&createCalls),
		"cobrança rejeitada NÃO pode chegar ao gateway")

	var stored []models.Payment
	require.NoError(t, models.DB.Where("order_id = ?", "order-e2e-router-002").Find(&stored).Error)
	require.Empty(t, stored, "nenhum pagamento deve ser persistido")
}
