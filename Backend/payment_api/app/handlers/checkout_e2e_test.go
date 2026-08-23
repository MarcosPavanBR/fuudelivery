//go:build integration

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ============================================================================
// Suíte E2E do checkout — CORTE 4 banco-único.
//
// Desde o corte 4 os handlers de pagamento usam APENAS Postgres (models.DB).
// O Mongo legado saiu da suíte: o dual-write é no-op quando models.MongoDabase
// é nil, então os testes sobem um container Postgres isolado por teste.
//
// Como rodar:
//
//	go test -tags=integration -v -run 'TestCheckoutE2E' ./app/handlers/
//
// Por padrão usa Docker + testcontainers (postgres:16-alpine). Com
// POSTGRES_TEST_URI definida conecta direto na instância informada (o schema
// é recriado a cada setup para garantir isolamento).
// ============================================================================

// ptrTime retorna um ponteiro para o time.Time informado — usado para
// preencher campos opcionais como WalletCreditedAt/RefundedAt.
func ptrTime(t time.Time) *time.Time {
	return &t
}

// setupCheckoutE2EEnv prepara o Postgres dos testes E2E:
//   - Padrão (sem POSTGRES_TEST_URI): Docker + testcontainers com a imagem
//     postgres:16-alpine — cada teste ganha um container isolado e descartável.
//   - Com POSTGRES_TEST_URI definida: conecta direto no Postgres informado
//     (ex.: instância local de dev). As tabelas são dropadas e recriadas
//     a cada setup para garantir isolamento.
func setupCheckoutE2EEnv(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()

	var pgContainer *postgres.PostgresContainer
	var dsn string

	if uri := os.Getenv("POSTGRES_TEST_URI"); uri != "" {
		dsn = uri
	} else {
		c, err := postgres.Run(ctx, "postgres:16-alpine",
			postgres.WithDatabase("payment_e2e_test"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
		)
		require.NoError(t, err, "subir container Postgres")
		pgContainer = c

		var dErr error
		dsn, dErr = c.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, dErr)
	}

	// Backoff clássico de testcontainers em CI: o container pode não aceitar
	// conexões imediatamente após o start (connection reset by peer).
	var gormDB *gorm.DB
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		gormDB, err = gorm.Open(postgresdriver.Open(dsn), &gorm.Config{})
		if err == nil {
			var ping int
			if pingErr := gormDB.Raw("SELECT 1").Scan(&ping).Error; pingErr == nil && ping == 1 {
				break
			}
			err = fmt.Errorf("postgres não respondeu ao ping")
		}
		time.Sleep(2 * time.Second)
	}
	require.NoError(t, err, "conectar no Postgres de teste")

	// Isolamento: dropa e recria as tabelas do domínio de pagamentos.
	for _, table := range []string{"wallet_transactions", "wallets", "payments"} {
		require.NoError(t, gormDB.Exec("DROP TABLE IF EXISTS "+table+" CASCADE").Error)
	}
	require.NoError(t, gormDB.AutoMigrate(&models.Payment{}, &models.Wallet{}, &models.WalletTxn{}))
	models.DB = gormDB

	// Mongo desativado nos testes: dual-write vira no-op (helpers checam nil).
	models.MongoClient = nil
	models.MongoDabase = nil

	return func() {
		if sqlDB, err := gormDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		if pgContainer != nil {
			_ = pgContainer.Terminate(ctx)
		}
	}
}

// bytesReader helper local (evita import direto de bytes em todo o arquivo).
func bytesReader(b []byte) *strings.Reader { return strings.NewReader(string(b)) }

// seedPayment insere um pagamento direto no Postgres (substitui o InsertOne
// do Mongo das versões anteriores da suíte).
func seedPayment(t *testing.T, p *models.Payment) {
	t.Helper()
	require.NoError(t, models.DB.Create(p).Error)
}

// findPaymentByAbacate recarrega o pagamento pelo ID externo do gateway.
func findPaymentByAbacate(t *testing.T, abacatepayID string) models.Payment {
	t.Helper()
	var p models.Payment
	require.NoError(t, models.DB.Where("abacatepay_id = ?", abacatepayID).First(&p).Error)
	return p
}

// seedWallet cria uma carteira com saldo inicial.
func seedWallet(t *testing.T, userID int64, userType string, balance float64) models.Wallet {
	t.Helper()
	w := models.Wallet{
		UserID:      userID,
		UserType:    userType,
		Balance:     balance,
		Currency:    "BRL",
		Status:      "active",
		LastUpdated: time.Now(),
	}
	require.NoError(t, models.DB.Create(&w).Error)
	return w
}

// seedLedger cria um lançamento no ledger vinculado à carteira do usuário.
func seedLedger(t *testing.T, walletID int64, txnType, kind string, amount, balanceAfter float64, refID, description string, createdAt time.Time) {
	t.Helper()
	entry := models.WalletTxn{
		WalletID:     walletID,
		Type:         txnType,
		Kind:         kind,
		Amount:       amount,
		BalanceAfter: balanceAfter,
		Description:  description,
		ReferenceID:  refID,
		CreatedAt:    createdAt,
	}
	require.NoError(t, models.DB.Create(&entry).Error)
}

// countLedger conta lançamentos do usuário (join com wallets), com filtros
// opcionais de tipo/kind/referência — substitui os CountDocuments do Mongo.
func countLedger(t *testing.T, userID int64, txnType, kind, refID string) int64 {
	t.Helper()
	q := models.DB.Model(&models.WalletTxn{}).
		Joins("JOIN wallets ON wallets.id = wallet_transactions.wallet_id").
		Where("wallets.user_id = ?", userID)
	if txnType != "" {
		q = q.Where("wallet_transactions.type = ?", txnType)
	}
	if kind != "" {
		q = q.Where("wallet_transactions.kind = ?", kind)
	}
	if refID != "" {
		q = q.Where("wallet_transactions.reference_id = ?", refID)
	}
	var count int64
	require.NoError(t, q.Count(&count).Error)
	return count
}

// getWalletByUser recarrega a carteira pelo user_id (qualquer tipo).
func getWalletByUser(t *testing.T, userID int64) models.Wallet {
	t.Helper()
	var w models.Wallet
	require.NoError(t, models.DB.Where("user_id = ?", userID).First(&w).Error)
	return w
}

// Teste 1: Fluxo completo pagamento -> split rules -> CONFIRMED
func TestCheckoutE2E_PaymentWebhookToSplit(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	payment := models.Payment{
		OrderID:         "order-e2e-001",
		CustomerID:      100,
		CustomerPhone:   "+5511999999999",
		EstablishmentID: 42,
		Amount:          89.90,
		DeliveryAmount:  7.00,
		Method:          "pix",
		Status:          "PENDING",
		AbacatePayID:    "charge-e2e-test-001",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &payment)

	stored := findPaymentByAbacate(t, "charge-e2e-test-001")
	require.Equal(t, "PENDING", stored.Status)

	splitRules := defaultSplitRules(&stored, 5.0, 85.0)
	// amount=89.90, delivery=7.00: 5%+85%+delivery = 87.91 -> sobra 1.99
	// de cashback -> regra "customer" adicionada -> 4 regras.
	require.Len(t, splitRules, 4)

	require.Equal(t, "platform", splitRules[0].ReceiverType)
	require.InDelta(t, 89.90*0.05, splitRules[0].Amount, 0.01)

	require.Equal(t, "establishment", splitRules[1].ReceiverType)
	require.InDelta(t, 89.90*0.85, splitRules[1].Amount, 0.01)

	require.Equal(t, "deliveryman", splitRules[2].ReceiverType)
	require.InDelta(t, 7.00, splitRules[2].Amount, 0.01)

	require.Equal(t, "customer", splitRules[3].ReceiverType)
	require.InDelta(t, 89.90-89.90*0.05-89.90*0.85-7.00, splitRules[3].Amount, 0.01)

	totalSplit := 0.0
	for _, r := range splitRules {
		totalSplit += r.Amount
	}
	require.LessOrEqual(t, totalSplit, 89.91)

	now := time.Now()
	require.NoError(t, models.DB.Model(&stored).Updates(map[string]interface{}{
		"status":       "CONFIRMED",
		"split_rules":  models.SplitRules(splitRules),
		"confirmed_at": now,
	}).Error)

	confirmed := findPaymentByAbacate(t, "charge-e2e-test-001")
	require.Equal(t, "CONFIRMED", confirmed.Status)
	require.Len(t, confirmed.SplitRules, 4)
	require.NotNil(t, confirmed.ConfirmedAt)

	orderMsg := map[string]interface{}{
		"order_id":     confirmed.OrderID,
		"payment_id":   confirmed.IDString(),
		"status":       "PAYMENT_CONFIRMED",
		"amount":       confirmed.Amount,
		"method":       confirmed.Method,
		"confirmed_at": now.Format(time.RFC3339),
	}
	msgBody, _ := json.Marshal(orderMsg)
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(msgBody, &parsed))
	require.Equal(t, "order-e2e-001", parsed["order_id"])
	require.Equal(t, "PAYMENT_CONFIRMED", parsed["status"])
}

// Teste 2: Idempotencia de split — reaplicar nao duplica regras
func TestCheckoutE2E_WebhookIdempotent(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	payment := models.Payment{
		OrderID:         "order-e2e-idempotent",
		CustomerID:      200,
		EstablishmentID: 55,
		Amount:          50.00,
		DeliveryAmount:  5.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-idempotent-001",
		SplitRules: models.SplitRules([]models.SplitRule{
			{ReceiverType: "platform", Amount: 2.25, Percentage: 4.5},
			{ReceiverType: "establishment", Amount: 40.25, Percentage: 80.5},
			{ReceiverType: "deliveryman", Amount: 5.00, Percentage: 0},
		}),
		CreatedAt: time.Now(),
	}
	seedPayment(t, &payment)

	stored := findPaymentByAbacate(t, "charge-idempotent-001")

	newSplit := defaultSplitRules(&stored, 5.0, 85.0)
	require.NoError(t, models.DB.Model(&stored).Update("split_rules", models.SplitRules(newSplit)).Error)

	result := findPaymentByAbacate(t, "charge-idempotent-001")
	require.Equal(t, "CONFIRMED", result.Status)
	require.Len(t, result.SplitRules, 3)

	totalSplit := 0.0
	for _, r := range result.SplitRules {
		totalSplit += r.Amount
	}
	require.InDelta(t, 50.00, totalSplit, 0.01)
}

// Teste 3: Pedido pequeno nao gera split negativo
func TestCheckoutE2E_SmallOrderNoNegativeSplit(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	payment := models.Payment{
		OrderID:         "order-e2e-small",
		CustomerID:      300,
		EstablishmentID: 77,
		Amount:          5.00,
		DeliveryAmount:  7.00,
		Method:          "pix",
		Status:          "PENDING",
		AbacatePayID:    "charge-small-001",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &payment)

	stored := findPaymentByAbacate(t, "charge-small-001")

	splitRules := defaultSplitRules(&stored, 5.0, 85.0)
	require.NotEmpty(t, splitRules)

	for i, rule := range splitRules {
		require.GreaterOrEqual(t, rule.Amount, 0.0,
			"split rule[%d] (%s) negativo: %.2f", i, rule.ReceiverType, rule.Amount)
	}

	totalSplit := 0.0
	for _, r := range splitRules {
		totalSplit += r.Amount
	}
	// delivery=7.00 > amount=5.00: o ajuste zera platform/establishment e
	// o total fica igual a taxa de entrega (nunca excede, nunca fica negativo).
	require.InDelta(t, 7.00, totalSplit, 0.01)
}

// Teste 4: Canais de fila alinhados
func TestCheckoutE2E_QueueChannelAlignment(t *testing.T) {
	monolithChannels := map[string]bool{
		"order_updates":    true,
		"delivery_updates": true,
		"payment_updates":  true,
	}

	require.True(t, monolithChannels[paymentRedisQueueKey])
	require.True(t, monolithChannels[orderRedisQueueKey])

	msg := map[string]interface{}{
		"order_id":     "order-123",
		"payment_id":   "pay-456",
		"status":       "PAYMENT_CONFIRMED",
		"amount":       49.90,
		"method":       "pix",
		"confirmed_at": time.Now().Format(time.RFC3339),
	}

	body, _ := json.Marshal(msg)
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotNil(t, parsed["order_id"])
	require.NotNil(t, parsed["status"])
}

// Teste 5: Endpoints admin do painel Financeiro (monolito)
// Cobre GET /payments/stats, POST /payments/:id/approve,
// POST /payments/:id/reject, GET /wallets e GET /chargebacks — as rotas que
// o WebAdmin Financeiro.jsx usa.
func TestCheckoutE2E_AdminEndpoints(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	// --- Seeds: 1 PENDING + 1 CONFIRMED + 1 wallet ---
	pending := models.Payment{
		OrderID:         "order-admin-pending",
		CustomerID:      10,
		EstablishmentID: 42,
		Amount:          10000, // centavos: R$ 100,00
		Method:          "pix",
		Status:          "PENDING",
		AbacatePayID:    "charge-admin-001",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &pending)
	pendingID := strconv.FormatInt(pending.ID, 10)

	confirmedSeed := models.Payment{
		OrderID:         "order-admin-confirmed",
		CustomerID:      11,
		EstablishmentID: 43,
		Amount:          5000, // R$ 50,00
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-admin-002",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &confirmedSeed)

	seedWallet(t, 42, "establishment", 8500) // R$ 85,00

	app := fiber.New()

	// --- GET /payments/stats ---
	app.Get("/payments/stats", GetPaymentStats)
	req := httptest.NewRequest(http.MethodGet, "/payments/stats", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var stats map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&stats))
	require.Equal(t, float64(2), stats["total"])
	require.Equal(t, float64(1), stats["pending"])
	require.Equal(t, float64(1), stats["confirmed"])
	require.Equal(t, float64(0), stats["rejected"])
	require.Equal(t, float64(15000), stats["total_amount"]) // 10000 + 5000

	// --- POST /payments/:id/approve ---
	app.Post("/payments/:id/approve", ApprovePayment)
	req = httptest.NewRequest(http.MethodPost, "/payments/"+pendingID+"/approve", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var approved map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&approved))
	require.Equal(t, "CONFIRMED", approved["status"])

	stored := findPaymentByAbacate(t, "charge-admin-001")
	require.Equal(t, "CONFIRMED", stored.Status)
	require.NotNil(t, stored.ConfirmedAt)

	// Reaprovar um pagamento ja confirmado deve dar 409
	req = httptest.NewRequest(http.MethodPost, "/payments/"+pendingID+"/approve", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 409, resp.StatusCode)

	// --- POST /payments/:id/reject ---
	app.Post("/payments/:id/reject", RejectPayment)
	rejectBody := strings.NewReader(`{"reason":"chargeback do cliente"}`)
	req = httptest.NewRequest(http.MethodPost, "/payments/"+pendingID+"/reject", rejectBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 409, resp.StatusCode, "ja esta CONFIRMED, nao pode rejeitar")

	// Cria um novo PENDING para rejeitar
	pending2 := models.Payment{
		OrderID:         "order-admin-reject",
		CustomerID:      12,
		EstablishmentID: 44,
		Amount:          7000,
		Method:          "pix",
		Status:          "PENDING",
		AbacatePayID:    "charge-admin-003",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &pending2)
	pending2ID := strconv.FormatInt(pending2.ID, 10)

	rejectBody = strings.NewReader(`{"reason":"fraude suspeita"}`)
	req = httptest.NewRequest(http.MethodPost, "/payments/"+pending2ID+"/reject", rejectBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var rejected map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rejected))
	require.Equal(t, "REJECTED", rejected["status"])

	stored = findPaymentByAbacate(t, "charge-admin-003")
	require.Equal(t, "REJECTED", stored.Status)
	require.NotNil(t, stored.RejectedAt)

	// --- GET /wallets ---
	app.Get("/wallets", ListWallets)
	req = httptest.NewRequest(http.MethodGet, "/wallets", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var wallets []map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wallets))
	require.Len(t, wallets, 1)
	require.Equal(t, "establishment", wallets[0]["owner_type"])
	require.Equal(t, float64(8500), wallets[0]["balance"])

	// --- GET /chargebacks (ledger para o painel Financeiro) ---
	// Seeds: carteiras + 1 credito (top-up do cliente) + 1 debito (chargeback)
	w5001 := seedWallet(t, 5001, "customer", 150.0)
	w42 := getWalletByUser(t, 42)
	now := time.Now()
	seedLedger(t, w5001.ID, "credit", "", 100.0, 150.0, "charge-ledger-001",
		"Wallet top-up via confirmed payment", now.Add(-2*time.Hour))
	seedLedger(t, w42.ID, "debit", "", 85.0, 115.0, "charge-e2e-refund-001",
		"Refund/chargeback: estorno do pagamento order-x", now.Add(-1*time.Hour))

	app.Get("/chargebacks", ListChargebacks)
	req = httptest.NewRequest(http.MethodGet, "/chargebacks", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var chargebacks map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&chargebacks))
	entries, ok := chargebacks["chargebacks"].([]interface{})
	require.True(t, ok, "resposta deve ter lista chargebacks")
	require.Len(t, entries, 2, "ledger deve listar credito + debito")

	// Ordenado por created_at desc: debito (mais recente) primeiro
	first := entries[0].(map[string]interface{})
	require.Equal(t, "debit", first["type"])
	require.Equal(t, "charge-e2e-refund-001", first["payment_id"])

	// owner_type enriquecido da carteira do estabelecimento 42
	require.Equal(t, "establishment", first["owner_type"])

	// Resumo agregado (sem filtro): credito 100.0 (top-up) - debito 85.0
	summary, hasSummary := chargebacks["summary"].(map[string]interface{})
	require.True(t, hasSummary, "resposta deve incluir summary agregado")
	require.InDelta(t, 100.0, summary["credit_total"], 0.01, "total de creditos do ledger")
	require.InDelta(t, 85.0, summary["debit_total"], 0.01, "total de debitos do ledger")
	require.InDelta(t, 15.0, summary["net"], 0.01, "saldo liquido = creditos - debitos")

	// Resumo reflete os filtros: so debitos -> credit_total 0 e net negativo
	req = httptest.NewRequest(http.MethodGet, "/chargebacks?type=debit", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var cbFiltered map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&cbFiltered))
	summary, hasSummary = cbFiltered["summary"].(map[string]interface{})
	require.True(t, hasSummary)
	require.InDelta(t, 0.0, summary["credit_total"], 0.01, "filtro debit nao deve ter creditos")
	require.InDelta(t, 85.0, summary["debit_total"], 0.01)
	require.InDelta(t, -85.0, summary["net"], 0.01)

	// Filtro por tipo: so debitos
	req = httptest.NewRequest(http.MethodGet, "/chargebacks?type=debit", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&chargebacks))
	entries, ok = chargebacks["chargebacks"].([]interface{})
	require.True(t, ok)
	require.Len(t, entries, 1, "filtro type=debit deve retornar so o debito")

	// Filtro por user_id do cliente
	req = httptest.NewRequest(http.MethodGet, "/chargebacks?user_id=5001", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&chargebacks))
	entries, ok = chargebacks["chargebacks"].([]interface{})
	require.True(t, ok)
	require.Len(t, entries, 1, "filtro user_id=5001 deve retornar so o credito do cliente")
	creditEntry := entries[0].(map[string]interface{})
	require.Equal(t, "credit", creditEntry["type"])
	require.Equal(t, "customer", creditEntry["owner_type"])

	// Sem resultados
	req = httptest.NewRequest(http.MethodGet, "/chargebacks?payment_id=nao-existe", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&chargebacks))
	entries, ok = chargebacks["chargebacks"].([]interface{})
	require.True(t, ok)
	require.Empty(t, entries, "sem lancamentos para payment_id inexistente")

	// --- Validacao: ID invalido ---
	req = httptest.NewRequest(http.MethodPost, "/payments/abc/approve", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	// ID numerico inexistente -> 404
	req = httptest.NewRequest(http.MethodPost, "/payments/999999999/reject", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode, "id numerico valido mas inexistente -> not found")
}

// TestCheckoutE2E_WebhookRealFlow_Cashback percorre o fluxo completo do
// webhook real do AbacatePay: POST no HandlePaymentWebhook -> verificacao
// server-side da charge na API (mockada) -> atualizacao do status -> split
// com cashback (customer credit > 0), cobrindo a regra de 4 receivers:
// platform, establishment, deliveryman e customer.
func TestCheckoutE2E_WebhookRealFlow_Cashback(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	// Sem Redis: o publish das filas cai no fallback de Go channels (nao bloqueia).
	os.Unsetenv("REDIS_URL")

	// Mock da API AbacatePay v2: o webhook NAO confia no body — verifica o
	// status da charge consultando a API (aqui mockada). Conta as chamadas
	// para provar que a verificacao server-side aconteceu de verdade.
	var mockCalls int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mockCalls, 1)
		if r.URL.Path != "/transparents/check" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		chargeID := r.URL.Query().Get("id")
		if chargeID == "" {
			t.Error("query param id ausente no check")
		}
		// Charge expirada -> status EXPIRED na API (cenario de nao-pagamento).
		chargeStatus := "PAID"
		if strings.Contains(chargeID, "expired") {
			chargeStatus = "EXPIRED"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"success":true,"data":{"id":%q,"status":%q,"amount":8990,"expiresAt":"2026-08-13T12:00:00Z"},"error":null}`, chargeID, chargeStatus)
	}))
	defer mock.Close()

	os.Setenv("ABACATE_PAY_BASE_URL", mock.URL)
	defer os.Unsetenv("ABACATE_PAY_BASE_URL")

	os.Setenv("ABACATE_PAY_WEBHOOK_SECRET", "e2e-webhook-secret")
	defer os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	// amount=89.90, delivery=7.00, 5%+85%: platform=4.495, est=76.415,
	// delivery=7.00 -> sobra 1.99 de cashback (customer credit > 0) -> 4 receivers.
	payment := models.Payment{
		OrderID:         "order-e2e-webhook-cashback",
		CustomerID:      777,
		CustomerPhone:   "+5511988887777",
		EstablishmentID: 42,
		Amount:          89.90,
		DeliveryAmount:  7.00,
		Method:          "pix",
		Status:          "PENDING",
		AbacatePayID:    "charge-e2e-cashback-001",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &payment)

	app := fiber.New()
	app.Post("/api/payment/webhook", HandlePaymentWebhook)

	// Payload real do webhook v2 (evento billing.paid)
	webhookBody := []byte(`{"event":"billing.paid","charge":{"id":"charge-e2e-cashback-001","status":"PAID","amount":89.90}}`)

	// --- 1. HMAC invalido -> 401, API nao consultada, status intacto ---
	req := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytesReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", "assinatura-errada")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 401, resp.StatusCode)
	require.Equal(t, int32(0), atomic.LoadInt32(&mockCalls), "HMAC invalido nao deve consultar a API")

	stored := findPaymentByAbacate(t, "charge-e2e-cashback-001")
	require.Equal(t, "PENDING", stored.Status, "HMAC invalido nao pode mudar o status")

	// --- 2. Webhook legitimo (HMAC valido) -> CONFIRMED + split com cashback ---
	req = httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytesReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", computeHMAC(webhookBody, "e2e-webhook-secret"))
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var processed map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&processed))
	require.Equal(t, "processed", processed["status"])

	// A verificacao server-side realmente consultou a API mockada.
	require.Equal(t, int32(1), atomic.LoadInt32(&mockCalls), "webhook legitimo deve consultar a API para confirmar a charge")

	stored = findPaymentByAbacate(t, "charge-e2e-cashback-001")
	require.Equal(t, "CONFIRMED", stored.Status)
	require.NotNil(t, stored.ConfirmedAt)

	// 4 receivers: platform + establishment + deliveryman + customer (cashback)
	require.Len(t, stored.SplitRules, 4, "cashback > 0 deve gerar a 4a regra (customer)")

	require.Equal(t, "platform", stored.SplitRules[0].ReceiverType)
	require.InDelta(t, 89.90*0.05, stored.SplitRules[0].Amount, 0.01)

	require.Equal(t, "establishment", stored.SplitRules[1].ReceiverType)
	require.Equal(t, int64(42), stored.SplitRules[1].ReceiverID)
	require.InDelta(t, 89.90*0.85, stored.SplitRules[1].Amount, 0.01)

	require.Equal(t, "deliveryman", stored.SplitRules[2].ReceiverType)
	require.InDelta(t, 7.00, stored.SplitRules[2].Amount, 0.01)

	require.Equal(t, "customer", stored.SplitRules[3].ReceiverType)
	require.Equal(t, int64(777), stored.SplitRules[3].ReceiverID, "cashback vai para o customer_id do pagamento")
	require.InDelta(t, 89.90-89.90*0.05-89.90*0.85-7.00, stored.SplitRules[3].Amount, 0.01)

	// O total dividido nunca excede o valor pago (nenhum centavo inventado).
	totalSplit := 0.0
	for _, r := range stored.SplitRules {
		totalSplit += r.Amount
	}
	require.InDelta(t, 89.90, totalSplit, 0.01)

	// --- 2b. Credito real na carteira do restaurante apos o split ---
	estWallet := getWalletByUser(t, 42)
	require.Equal(t, "establishment", estWallet.UserType)
	require.InDelta(t, 89.90*0.85, estWallet.Balance, 0.01, "carteira do restaurante recebe o share do split")

	// Marcador de credito gravado no pagamento
	stored = findPaymentByAbacate(t, "charge-e2e-cashback-001")
	require.NotNil(t, stored.EstablishmentCreditedAt, "pagamento deve registrar establishment_credited_at")

	// Lancamento de credito no ledger
	creditCount := countLedger(t, 42, "credit", "", "charge-e2e-cashback-001")
	require.Equal(t, int64(1), creditCount, "deve existir 1 lancamento de credito no ledger")

	// --- 3. Webhook idempotente: reprocessar nao muda o status nem duplica ---
	req = httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytesReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", computeHMAC(webhookBody, "e2e-webhook-secret"))
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	stored = findPaymentByAbacate(t, "charge-e2e-cashback-001")
	require.Equal(t, "CONFIRMED", stored.Status, "reprocessar webhook nao pode regredir o status")
	require.Len(t, stored.SplitRules, 4, "reprocessar nao pode duplicar split rules")

	// Reprocessar nao credita a carteira de novo (idempotencia do credito)
	estWallet = getWalletByUser(t, 42)
	require.InDelta(t, 89.90*0.85, estWallet.Balance, 0.01, "reprocessar webhook nao pode creditar duas vezes")

	creditCount = countLedger(t, 42, "credit", "", "charge-e2e-cashback-001")
	require.Equal(t, int64(1), creditCount, "ledger nao pode ter credito duplicado")

	// --- 4. Charge nao paga (EXPIRED) -> status EXPIRED, sem split ---
	expired := models.Payment{
		OrderID:         "order-e2e-webhook-expired",
		CustomerID:      778,
		EstablishmentID: 43,
		Amount:          50.00,
		DeliveryAmount:  5.00,
		Method:          "pix",
		Status:          "PENDING",
		AbacatePayID:    "charge-e2e-expired-001",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &expired)

	expiredBody := []byte(`{"event":"billing.expired","charge":{"id":"charge-e2e-expired-001","status":"EXPIRED","amount":50.00}}`)
	req = httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytesReader(expiredBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", computeHMAC(expiredBody, "e2e-webhook-secret"))
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	stored = findPaymentByAbacate(t, "charge-e2e-expired-001")
	require.Equal(t, "EXPIRED", stored.Status)
	require.Empty(t, stored.SplitRules, "charge nao paga nao pode gerar split")
}

// TestCheckoutE2E_WebhookRealFlow_Refund cobre o fluxo de chargeback/reembolso:
// webhook billing.refunded -> verificacao server-side (API mockada) -> reversao
// do credito da carteira do estabelecimento -> ledger de debito -> status
// REFUNDED. Tambem valida idempotencia (sem debito duplo), refund de
// pagamento nunca pago (sem reversao) e saldo insuficiente (nunca negativo).
func TestCheckoutE2E_WebhookRealFlow_Refund(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	os.Unsetenv("REDIS_URL")

	// Mock AbacatePay: charges com "refund" no id -> REFUNDED, resto -> PAID.
	var mockCalls int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mockCalls, 1)
		chargeID := r.URL.Query().Get("id")
		chargeStatus := "PAID"
		if strings.Contains(chargeID, "refund") {
			chargeStatus = "REFUNDED"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"success":true,"data":{"id":%q,"status":%q,"amount":10000,"expiresAt":"2026-08-13T12:00:00Z"},"error":null}`, chargeID, chargeStatus)
	}))
	defer mock.Close()

	os.Setenv("ABACATE_PAY_BASE_URL", mock.URL)
	defer os.Unsetenv("ABACATE_PAY_BASE_URL")

	os.Setenv("ABACATE_PAY_WEBHOOK_SECRET", "e2e-refund-secret")
	defer os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	app := fiber.New()
	app.Post("/api/payment/webhook", HandlePaymentWebhook)

	postWebhook := func(t *testing.T, body []byte) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytesReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-abacatepay-signature", computeHMAC(body, "e2e-refund-secret"))
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		return resp
	}

	// === Cenario 1: pagamento CONFIRMED -> refund reverte a carteira ===
	confirmed := models.Payment{
		OrderID:         "order-e2e-refund-001",
		CustomerID:      500,
		EstablishmentID: 42,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-e2e-refund-001",
		SplitRules: models.SplitRules([]models.SplitRule{
			{ReceiverType: "platform", Amount: 5.00, Percentage: 5.0},
			{ReceiverType: "establishment", Amount: 85.00, Percentage: 85.0},
			{ReceiverType: "deliveryman", Amount: 10.00, Percentage: 0},
		}),
		CreatedAt: time.Now(),
	}
	seedPayment(t, &confirmed)
	seedWallet(t, 42, "establishment", 200.0)

	refundBody := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-001","status":"REFUNDED","amount":100.00}}`)
	postWebhook(t, refundBody)

	// Verificacao server-side aconteceu
	require.Equal(t, int32(1), atomic.LoadInt32(&mockCalls))

	stored := findPaymentByAbacate(t, "charge-e2e-refund-001")
	require.Equal(t, "REFUNDED", stored.Status)
	require.NotNil(t, stored.RefundedAt)
	require.Len(t, stored.SplitRules, 3, "refund nao pode regenerar/duplicar split rules")

	// Carteira do estabelecimento debitada: 200 - 85 (share do split) = 115
	wallet := getWalletByUser(t, 42)
	require.InDelta(t, 115.0, wallet.Balance, 0.01, "chargeback deve reverter o credito do estabelecimento")

	// Ledger de debito gravado
	count := countLedger(t, 42, "debit", "", "charge-e2e-refund-001")
	require.Equal(t, int64(1), count, "deve existir 1 lancamento de debito no ledger")

	// === Cenario 2: idempotencia — reprocessar nao debita de novo ===
	postWebhook(t, refundBody)

	wallet = getWalletByUser(t, 42)
	require.InDelta(t, 115.0, wallet.Balance, 0.01, "reprocessar webhook nao pode debitar duas vezes")

	count = countLedger(t, 42, "debit", "", "charge-e2e-refund-001")
	require.Equal(t, int64(1), count, "ledger nao pode ter debito duplicado")

	// === Cenario 3: refund de pagamento PENDING (nunca pago) -> sem reversao ===
	pending := models.Payment{
		OrderID:         "order-e2e-refund-pending",
		CustomerID:      501,
		EstablishmentID: 43,
		Amount:          50.00,
		DeliveryAmount:  5.00,
		Method:          "pix",
		Status:          "PENDING",
		AbacatePayID:    "charge-e2e-refund-pending",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &pending)
	seedWallet(t, 43, "establishment", 500.0)

	pendingRefund := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-pending","status":"REFUNDED","amount":50.00}}`)
	postWebhook(t, pendingRefund)

	stored = findPaymentByAbacate(t, "charge-e2e-refund-pending")
	require.Equal(t, "REFUNDED", stored.Status)

	wallet = getWalletByUser(t, 43)
	require.InDelta(t, 500.0, wallet.Balance, 0.01, "pagamento nunca pago nao pode reverter credito")

	// === Cenario 4: saldo insuficiente — nunca fica negativo ===
	low := models.Payment{
		OrderID:         "order-e2e-refund-low",
		CustomerID:      502,
		EstablishmentID: 44,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-e2e-refund-low",
		SplitRules: models.SplitRules([]models.SplitRule{
			{ReceiverType: "platform", Amount: 5.00, Percentage: 5.0},
			{ReceiverType: "establishment", Amount: 85.00, Percentage: 85.0},
			{ReceiverType: "deliveryman", Amount: 10.00, Percentage: 0},
		}),
		CreatedAt: time.Now(),
	}
	seedPayment(t, &low)
	seedWallet(t, 44, "establishment", 10.0) // < share de 85

	lowRefund := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-low","status":"REFUNDED","amount":100.00}}`)
	postWebhook(t, lowRefund)

	stored = findPaymentByAbacate(t, "charge-e2e-refund-low")
	require.Equal(t, "REFUNDED", stored.Status)

	wallet = getWalletByUser(t, 44)
	require.InDelta(t, 10.0, wallet.Balance, 0.01, "saldo insuficiente: debito bloqueado, nunca negativo")

	// Ledger nao pode ter debito quando o saldo e insuficiente
	count = countLedger(t, 44, "debit", "", "charge-e2e-refund-low")
	require.Equal(t, int64(0), count)

	// === Cenario 5: cashback do cliente (receiver_type customer) revertido ===
	cashback := models.Payment{
		OrderID:         "order-e2e-refund-cashback",
		CustomerID:      503,
		EstablishmentID: 45,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-e2e-refund-cashback",
		SplitRules: models.SplitRules([]models.SplitRule{
			{ReceiverType: "platform", Amount: 5.00, Percentage: 5.0},
			{ReceiverType: "establishment", Amount: 82.00, Percentage: 82.0},
			{ReceiverType: "deliveryman", Amount: 10.00, Percentage: 0},
			{ReceiverType: "customer", Amount: 3.00, Percentage: 0},
		}),
		CreatedAt: time.Now(),
	}
	seedPayment(t, &cashback)

	// Carteira do estabelecimento (com credito do share 82) e carteira do
	// cliente (com credito de cashback 3).
	seedWallet(t, 45, "establishment", 182.0)
	seedWallet(t, 503, "customer", 13.0) // 10 de saldo previo + 3 de cashback

	cashbackRefund := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-cashback","status":"REFUNDED","amount":100.00}}`)
	postWebhook(t, cashbackRefund)

	wallet = getWalletByUser(t, 45)
	require.InDelta(t, 100.0, wallet.Balance, 0.01, "chargeback reverte o credito do estabelecimento (182 - 82)")

	wallet = getWalletByUser(t, 503)
	require.InDelta(t, 10.0, wallet.Balance, 0.01, "chargeback reverte o cashback do cliente (13 - 3)")

	// Ledger: debito do cashback do cliente gravado
	count = countLedger(t, 503, "debit", "", "charge-e2e-refund-cashback")
	require.Equal(t, int64(1), count, "deve existir 1 debito de cashback do cliente no ledger")

	// === Cenario 6: top-up de carteira quando o pagamento foi usado ===
	topup := models.Payment{
		OrderID:         "order-e2e-refund-topup",
		CustomerID:      504,
		EstablishmentID: 46,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-e2e-refund-topup",
		SplitRules: models.SplitRules([]models.SplitRule{
			{ReceiverType: "platform", Amount: 5.00, Percentage: 5.0},
			{ReceiverType: "establishment", Amount: 85.00, Percentage: 85.0},
			{ReceiverType: "deliveryman", Amount: 10.00, Percentage: 0},
		}),
		CreatedAt:        time.Now(),
		WalletCreditedAt: ptrTime(time.Now().Add(-time.Hour)),
	}
	seedPayment(t, &topup)
	seedWallet(t, 504, "customer", 250.0) // 150 previo + 100 do top-up

	topupRefund := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-topup","status":"REFUNDED","amount":100.00}}`)
	postWebhook(t, topupRefund)

	wallet = getWalletByUser(t, 504)
	require.InDelta(t, 150.0, wallet.Balance, 0.01, "chargeback reverte o top-up de carteira (250 - 100)")

	// Ledger: debito do top-up do cliente gravado
	count = countLedger(t, 504, "debit", "", "charge-e2e-refund-topup")
	require.Equal(t, int64(1), count, "deve existir 1 debito do top-up no ledger")
}

// TestCheckoutE2E_WebhookRealFlow_ZoneSplitConfig valida que o split usa os
// percentuais customizados da zona do estabelecimento (via
// GetSplitConfigForEstablishment, ligado pelo monolito) em vez do default 5/85.
func TestCheckoutE2E_WebhookRealFlow_ZoneSplitConfig(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	os.Unsetenv("REDIS_URL")

	// Mock AbacatePay: charge confirmada (PAID).
	var mockCalls int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mockCalls, 1)
		chargeID := r.URL.Query().Get("id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"success":true,"data":{"id":%q,"status":"PAID","amount":10000,"expiresAt":"2026-08-13T12:00:00Z"},"error":null}`, chargeID)
	}))
	defer mock.Close()

	os.Setenv("ABACATE_PAY_BASE_URL", mock.URL)
	defer os.Unsetenv("ABACATE_PAY_BASE_URL")

	os.Setenv("ABACATE_PAY_WEBHOOK_SECRET", "e2e-zone-secret")
	defer os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	// Ligacao que o monolito faz em main(): resolver de split por zona.
	// Zona com percentuais customizados 7%/80% para o estabelecimento 42.
	var resolverCalls int32
	savedResolver := GetSplitConfigForEstablishment
	GetSplitConfigForEstablishment = func(establishmentID int64) (float64, float64) {
		atomic.AddInt32(&resolverCalls, 1)
		require.Equal(t, int64(42), establishmentID, "resolver deve receber o establishment_id do pagamento")
		return 7.0, 80.0 // zona customizada: 7% plataforma / 80% estabelecimento
	}
	defer func() { GetSplitConfigForEstablishment = savedResolver }()

	app := fiber.New()
	app.Post("/api/payment/webhook", HandlePaymentWebhook)

	payment := models.Payment{
		OrderID:         "order-e2e-zone-split",
		CustomerID:      900,
		EstablishmentID: 42,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "PENDING",
		AbacatePayID:    "charge-e2e-zone-split",
		CreatedAt:       time.Now(),
	}
	seedPayment(t, &payment)

	webhookBody := []byte(`{"event":"billing.paid","charge":{"id":"charge-e2e-zone-split","status":"PAID","amount":100.00}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytesReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", computeHMAC(webhookBody, "e2e-zone-secret"))
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	// Verificacao server-side + resolver de zona chamados de verdade.
	require.Equal(t, int32(1), atomic.LoadInt32(&mockCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&resolverCalls), "GetSplitConfigForEstablishment deve ser consultado no webhook")

	stored := findPaymentByAbacate(t, "charge-e2e-zone-split")
	require.Equal(t, "CONFIRMED", stored.Status)

	// Split com percentuais da zona: 7/80/10 + cashback 3 -> 4 regras.
	require.Len(t, stored.SplitRules, 4)

	require.Equal(t, "platform", stored.SplitRules[0].ReceiverType)
	require.InDelta(t, 7.0, stored.SplitRules[0].Amount, 0.01, "7% de 100")
	require.InDelta(t, 7.0, stored.SplitRules[0].Percentage, 0.01)

	require.Equal(t, "establishment", stored.SplitRules[1].ReceiverType)
	require.InDelta(t, 80.0, stored.SplitRules[1].Amount, 0.01, "80% de 100")
	require.InDelta(t, 80.0, stored.SplitRules[1].Percentage, 0.01)

	require.Equal(t, "deliveryman", stored.SplitRules[2].ReceiverType)
	require.InDelta(t, 10.0, stored.SplitRules[2].Amount, 0.01)

	require.Equal(t, "customer", stored.SplitRules[3].ReceiverType)
	require.InDelta(t, 3.0, stored.SplitRules[3].Amount, 0.01, "cashback: 100-7-80-10")

	// O total dividido nunca excede o valor pago.
	totalSplit := 0.0
	for _, r := range stored.SplitRules {
		totalSplit += r.Amount
	}
	require.InDelta(t, 100.0, totalSplit, 0.01)

	// Controle: os percentuais vieram da zona (7/80), nao do default (5/85).
	require.Greater(t, stored.SplitRules[0].Percentage, 5.0, "percentual da plataforma deve vir da zona (7%), nao do default 5%")
	require.Less(t, stored.SplitRules[1].Percentage, 85.0, "percentual do estabelecimento deve vir da zona (80%), nao do default 85%")

	// Carteira do restaurante creditada pelo share da ZONA (80%), nao 85%.
	estWallet := getWalletByUser(t, 42)
	require.Equal(t, "establishment", estWallet.UserType)
	require.InDelta(t, 80.0, estWallet.Balance, 0.01, "carteira recebe o share da zona (80), nao o default 85")
}

// TestCheckoutE2E_WalletEstablishmentEndpoints cobre os endpoints da carteira
// do restaurante (WebRestaurant): GET /wallet/establishment/balance,
// GET /wallet/establishment/transactions e POST /wallet/establishment/withdraw.
func TestCheckoutE2E_WalletEstablishmentEndpoints(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	os.Setenv("JWT_SECRET", "e2e-wallet-secret")
	defer os.Unsetenv("JWT_SECRET")

	const estID int64 = 42

	app := fiber.New()
	app.Get("/wallet/establishment/balance", GetEstablishmentWallet)
	app.Get("/wallet/establishment/transactions", GetEstablishmentTransactions)
	app.Post("/wallet/establishment/withdraw", EstablishmentWithdraw)

	// Token de restaurante com o claim establishment_id=42.
	makeToken := func() string {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"id":               999,
			"establishment_id": float64(estID),
			"role":             "restaurant",
			"exp":              time.Now().Add(time.Hour).Unix(),
		})
		s, err := tok.SignedString([]byte("e2e-wallet-secret"))
		require.NoError(t, err)
		return s
	}

	do := func(method, path string, body []byte, token string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(method, path, bytesReader(body))
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		return resp
	}

	// --- Seeds: carteira do estabelecimento + 3 lancamentos no ledger ---
	walletSeed := seedWallet(t, estID, "establishment", 150.0)

	now := time.Now()
	// 1) credito do split (mais antigo)
	seedLedger(t, walletSeed.ID, "credit", "", 100.0, 100.0, "charge-e2e-wallet-credit",
		"Credito do split do pedido order-w-1", now.Add(-2*time.Hour))
	// 2) saque (kind=withdrawal — conta como "total sacado")
	seedLedger(t, walletSeed.ID, "debit", "withdrawal", 50.0, 150.0, "",
		"Saque via PIX para pix@example.com", now.Add(-1*time.Hour))
	// 3) debito de chargeback (sem kind — NAO conta como saque)
	seedLedger(t, walletSeed.ID, "debit", "", 20.0, 130.0, "charge-e2e-wallet-debit",
		"Refund/chargeback: estorno do pagamento order-w-3", now.Add(-30*time.Minute))

	// --- GET balance (token valido) ---
	resp := do(http.MethodGet, "/wallet/establishment/balance", nil, makeToken())
	require.Equal(t, 200, resp.StatusCode)
	var bal map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&bal))
	require.Equal(t, float64(estID), bal["user_id"])
	require.InDelta(t, 150.0, bal["available"], 0.01, "saldo disponivel = balance da carteira")
	require.InDelta(t, 0.0, bal["pending"], 0.01)
	require.InDelta(t, 0.0, bal["blocked"], 0.01)
	require.InDelta(t, 100.0, bal["total_earned"], 0.01, "total ganho = soma dos creditos do ledger")
	require.InDelta(t, 50.0, bal["total_withdrawn"], 0.01, "total sacado = so debitos kind=withdrawal")

	// --- GET balance sem token -> 403 (o estabelecimento vem do JWT) ---
	resp = do(http.MethodGet, "/wallet/establishment/balance", nil, "")
	require.Equal(t, 403, resp.StatusCode)

	// --- GET transactions: 3 lancamentos, tipos mapeados em MAIUSCULAS ---
	resp = do(http.MethodGet, "/wallet/establishment/transactions", nil, makeToken())
	require.Equal(t, 200, resp.StatusCode)
	var extract map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&extract))
	require.Equal(t, false, extract["has_more"])
	require.Equal(t, "", extract["next_cursor"])
	data, ok := extract["data"].([]interface{})
	require.True(t, ok, "resposta deve ter lista data")
	require.Len(t, data, 3, "3 lancamentos no ledger do estabelecimento")

	byType := map[string]map[string]interface{}{}
	for _, it := range data {
		entry, isMap := it.(map[string]interface{})
		require.True(t, isMap)
		typ, _ := entry["type"].(string)
		byType[typ] = entry
	}

	credit := byType["CREDIT"]
	require.NotNil(t, credit, "deve existir lancamento CREDIT")
	require.Equal(t, "charge-e2e-wallet-credit", credit["payment_ref"])
	require.InDelta(t, 100.0, credit["amount"], 0.01)
	require.InDelta(t, 100.0, credit["balance"], 0.01)
	require.NotEmpty(t, credit["id"])
	require.NotEmpty(t, credit["created_at"])

	withdrawal := byType["WITHDRAWAL"]
	require.NotNil(t, withdrawal, "saque deve mapear para WITHDRAWAL")
	require.Contains(t, withdrawal["description"], "Saque via PIX")
	require.InDelta(t, 50.0, withdrawal["amount"], 0.01)

	debit := byType["DEBIT"]
	require.NotNil(t, debit, "chargeback deve mapear para DEBIT")
	require.Equal(t, "charge-e2e-wallet-debit", debit["payment_ref"])

	// --- Paginacao: limit=2 + cursor ---
	resp = do(http.MethodGet, "/wallet/establishment/transactions?limit=2", nil, makeToken())
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&extract))
	require.Equal(t, true, extract["has_more"])
	nextCursor, _ := extract["next_cursor"].(string)
	require.NotEmpty(t, nextCursor)
	data, ok = extract["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 2)

	resp = do(http.MethodGet, "/wallet/establishment/transactions?limit=2&cursor="+nextCursor, nil, makeToken())
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&extract))
	require.Equal(t, false, extract["has_more"])
	data, ok = extract["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 1, "pagina 2 deve trazer apenas o lancamento mais antigo")

	// Cursor invalido -> 400
	resp = do(http.MethodGet, "/wallet/establishment/transactions?cursor=abc", nil, makeToken())
	require.Equal(t, 400, resp.StatusCode)

	// --- POST withdraw: saque valido de 40 (150 -> 110) ---
	withdrawBody := []byte(`{"amount":40,"destination":"pix@example.com","method":"PIX"}`)
	resp = do(http.MethodPost, "/wallet/establishment/withdraw", withdrawBody, makeToken())
	require.Equal(t, 200, resp.StatusCode)
	var wd map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wd))
	require.InDelta(t, 110.0, wd["balance"], 0.01, "150 - 40")

	wallet := getWalletByUser(t, estID)
	require.InDelta(t, 110.0, wallet.Balance, 0.01, "saque deve debitar a carteira")

	// Novo lancamento de saque no ledger (type=debit, kind=withdrawal)
	withdrawCount := countLedger(t, estID, "debit", "withdrawal", "")
	require.Equal(t, int64(2), withdrawCount, "1 saque seed + 1 novo saque")

	// total_withdrawn e available refletem o novo saque
	resp = do(http.MethodGet, "/wallet/establishment/balance", nil, makeToken())
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&bal))
	require.InDelta(t, 90.0, bal["total_withdrawn"], 0.01, "50 + 40")
	require.InDelta(t, 110.0, bal["available"], 0.01)

	// --- Validacoes do saque ---
	// Abaixo do minimo (R$ 10)
	resp = do(http.MethodPost, "/wallet/establishment/withdraw", []byte(`{"amount":5,"destination":"pix@example.com","method":"PIX"}`), makeToken())
	require.Equal(t, 400, resp.StatusCode)

	// Chave/destino invalido (curto demais)
	resp = do(http.MethodPost, "/wallet/establishment/withdraw", []byte(`{"amount":20,"destination":"abc","method":"PIX"}`), makeToken())
	require.Equal(t, 400, resp.StatusCode)

	// Saldo insuficiente (111 > 110)
	resp = do(http.MethodPost, "/wallet/establishment/withdraw", []byte(`{"amount":111,"destination":"pix@example.com","method":"PIX"}`), makeToken())
	require.Equal(t, 400, resp.StatusCode)

	// Sem token -> 403
	resp = do(http.MethodPost, "/wallet/establishment/withdraw", withdrawBody, "")
	require.Equal(t, 403, resp.StatusCode)

	// Nenhum saque invalido pode ter mexido no saldo
	wallet = getWalletByUser(t, estID)
	require.InDelta(t, 110.0, wallet.Balance, 0.01, "saques invalidos nao podem alterar o saldo")
}
