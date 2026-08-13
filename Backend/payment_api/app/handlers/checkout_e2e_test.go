//go:build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ptrTime retorna um ponteiro para o time.Time informado — usado para
// preencher campos opcionais como WalletCreditedAt/RefundedAt.
func ptrTime(t time.Time) *time.Time {
	return &t
}

func setupCheckoutE2EEnv(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()

	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err, "subir container MongoDB")

	uri, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.NoError(t, err)
	require.NoError(t, client.Ping(ctx, nil))

	db := client.Database("payment_api_e2e_test")
	models.MongoClient = client
	models.MongoDabase = db

	db.Collection("payments").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: []string{"order_id"}, Options: options.Index().SetUnique(true)},
		{Keys: []string{"abacatepay_id"}},
	})

	return func() {
		_ = client.Disconnect(ctx)
		_ = mongoContainer.Terminate(ctx)
	}
}

// Teste 1: Fluxo completo pagamento -> webhook -> split -> CONFIRMED
func TestCheckoutE2E_PaymentWebhookToSplit(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")
	ctx := context.Background()
	paymentCollection := models.MongoDabase.Collection("payments")

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
	_, err := paymentCollection.InsertOne(ctx, payment)
	require.NoError(t, err)

	var stored models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-test-001"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "PENDING", stored.Status)

	splitRules := defaultSplitRules(&stored, 5.0, 85.0)
	// amount=89.90, delivery=7.00: 5%+85%+delivery = 87.91 → sobra 1.99
	// de cashback → regra "customer" adicionada → 4 regras.
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
	_, err = paymentCollection.UpdateOne(ctx,
		bson.M{"abacatepay_id": "charge-e2e-test-001"},
		bson.M{"$set": bson.M{"status": "CONFIRMED", "split_rules": splitRules, "confirmed_at": now}},
	)
	require.NoError(t, err)

	var confirmed models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-test-001"}).Decode(&confirmed)
	require.NoError(t, err)
	require.Equal(t, "CONFIRMED", confirmed.Status)
	require.Len(t, confirmed.SplitRules, 4)
	require.NotNil(t, confirmed.ConfirmedAt)

	orderMsg := map[string]interface{}{
		"order_id":     confirmed.OrderID,
		"payment_id":   confirmed.ID.Hex(),
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

// Teste 2: Idempotencia - webhook reprocessado nao duplica
func TestCheckoutE2E_WebhookIdempotent(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	ctx := context.Background()
	paymentCollection := models.MongoDabase.Collection("payments")

	payment := models.Payment{
		OrderID:         "order-e2e-idempotent",
		CustomerID:      200,
		EstablishmentID: 55,
		Amount:          50.00,
		DeliveryAmount:  5.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-idempotent-001",
		SplitRules: []models.SplitRule{
			{ReceiverType: "platform", Amount: 2.25, Percentage: 4.5},
			{ReceiverType: "establishment", Amount: 40.25, Percentage: 80.5},
			{ReceiverType: "deliveryman", Amount: 5.00, Percentage: 0},
		},
		CreatedAt: time.Now(),
	}
	_, err := paymentCollection.InsertOne(ctx, payment)
	require.NoError(t, err)

	var stored models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-idempotent-001"}).Decode(&stored)
	require.NoError(t, err)

	newSplit := defaultSplitRules(&stored, 5.0, 85.0)
	_, err = paymentCollection.UpdateOne(ctx,
		bson.M{"abacatepay_id": "charge-idempotent-001"},
		bson.M{"$set": bson.M{"split_rules": newSplit}},
	)
	require.NoError(t, err)

	var result models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-idempotent-001"}).Decode(&result)
	require.NoError(t, err)
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

	ctx := context.Background()
	paymentCollection := models.MongoDabase.Collection("payments")

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
	_, err := paymentCollection.InsertOne(ctx, payment)
	require.NoError(t, err)

	var stored models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-small-001"}).Decode(&stored)
	require.NoError(t, err)

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
	// o total fica igual à taxa de entrega (nunca excede, nunca fica negativo).
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
// POST /payments/:id/reject e GET /wallets — as rotas que o WebAdmin
// Financeiro.jsx usa e que antes viviam no serviço isolado removido.
func TestCheckoutE2E_AdminEndpoints(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	ctx := context.Background()
	paymentCollection := models.MongoDabase.Collection("payments")
	walletCollection := models.MongoDabase.Collection("wallets")

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
	resPending, err := paymentCollection.InsertOne(ctx, pending)
	require.NoError(t, err)
	pendingID := resPending.InsertedID.(primitive.ObjectID).Hex()

	confirmed := models.Payment{
		OrderID:         "order-admin-confirmed",
		CustomerID:      11,
		EstablishmentID: 43,
		Amount:          5000, // R$ 50,00
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-admin-002",
		CreatedAt:       time.Now(),
	}
	_, err = paymentCollection.InsertOne(ctx, confirmed)
	require.NoError(t, err)

	_, err = walletCollection.InsertOne(ctx, bson.M{
		"user_id":      42,
		"user_type":    "establishment",
		"balance":      8500, // R$ 85,00
		"last_updated": time.Now(),
	})
	require.NoError(t, err)

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

	var stored models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"_id": resPending.InsertedID}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "CONFIRMED", stored.Status)
	require.NotNil(t, stored.ConfirmedAt)

	// Reaprovar um pagamento já confirmado deve dar 409
	req = httptest.NewRequest(http.MethodPost, "/payments/"+pendingID+"/approve", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 409, resp.StatusCode)

	// --- POST /payments/:id/reject ---
	app.Post("/payments/:id/reject", RejectPayment)
	rejectBody := bytes.NewBufferString(`{"reason":"chargeback do cliente"}`)
	req = httptest.NewRequest(http.MethodPost, "/payments/"+pendingID+"/reject", rejectBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 409, resp.StatusCode, "já está CONFIRMED, não pode rejeitar")

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
	resPending2, err := paymentCollection.InsertOne(ctx, pending2)
	require.NoError(t, err)
	pending2ID := resPending2.InsertedID.(primitive.ObjectID).Hex()

	rejectBody = bytes.NewBufferString(`{"reason":"fraude suspeita"}`)
	req = httptest.NewRequest(http.MethodPost, "/payments/"+pending2ID+"/reject", rejectBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var rejected map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rejected))
	require.Equal(t, "REJECTED", rejected["status"])

	err = paymentCollection.FindOne(ctx, bson.M{"_id": resPending2.InsertedID}).Decode(&stored)
	require.NoError(t, err)
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

	// --- GET /chargebacks (wallet_ledger para o painel Financeiro) ---
	// Seeds: 1 crédito (top-up do cliente) + 1 débito (chargeback do estab)
	ledgerCollection := models.MongoDabase.Collection("wallet_ledger")
	now := time.Now()
	_, err = ledgerCollection.InsertOne(ctx, bson.M{
		"user_id":       5001,
		"type":          "credit",
		"amount":        100.0,
		"payment_id":    "charge-ledger-001",
		"balance_after": 150.0,
		"description":   "Wallet top-up via confirmed payment",
		"created_at":    now.Add(-2 * time.Hour),
	})
	require.NoError(t, err)
	_, err = ledgerCollection.InsertOne(ctx, bson.M{
		"user_id":       42,
		"type":          "debit",
		"amount":        85.0,
		"payment_id":    "charge-e2e-refund-001",
		"balance_after": 115.0,
		"description":   "Refund/chargeback: estorno do pagamento order-x",
		"created_at":    now.Add(-1 * time.Hour),
	})
	require.NoError(t, err)

	app.Get("/chargebacks", ListChargebacks)
	req = httptest.NewRequest(http.MethodGet, "/chargebacks", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var chargebacks map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&chargebacks))
	entries, ok := chargebacks["chargebacks"].([]interface{})
	require.True(t, ok, "resposta deve ter lista chargebacks")
	require.Len(t, entries, 2, "ledger deve listar crédito + débito")

	// Ordenado por created_at desc: débito (mais recente) primeiro
	first := entries[0].(map[string]interface{})
	require.Equal(t, "debit", first["type"])
	require.Equal(t, "charge-e2e-refund-001", first["payment_id"])

	// owner_type enriquecido da carteira do estabelecimento 42
	require.Equal(t, "establishment", first["owner_type"])

	// Resumo agregado (sem filtro): crédito 100.0 (top-up) − débito 85.0
	// (chargeback) = saldo líquido 15.0
	summary, hasSummary := chargebacks["summary"].(map[string]interface{})
	require.True(t, hasSummary, "resposta deve incluir summary agregado")
	require.InDelta(t, 100.0, summary["credit_total"], 0.01, "total de créditos do ledger")
	require.InDelta(t, 85.0, summary["debit_total"], 0.01, "total de débitos do ledger")
	require.InDelta(t, 15.0, summary["net"], 0.01, "saldo líquido = créditos − débitos")

	// Resumo reflete os filtros: só débitos → credit_total 0 e net negativo
	req = httptest.NewRequest(http.MethodGet, "/chargebacks?type=debit", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var cbFiltered map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&cbFiltered))
	summary, hasSummary = cbFiltered["summary"].(map[string]interface{})
	require.True(t, hasSummary)
	require.InDelta(t, 0.0, summary["credit_total"], 0.01, "filtro debit não deve ter créditos")
	require.InDelta(t, 85.0, summary["debit_total"], 0.01)
	require.InDelta(t, -85.0, summary["net"], 0.01)

	// Filtro por tipo: só débitos
	req = httptest.NewRequest(http.MethodGet, "/chargebacks?type=debit", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&chargebacks))
	entries, ok = chargebacks["chargebacks"].([]interface{})
	require.True(t, ok)
	require.Len(t, entries, 1, "filtro type=debit deve retornar só o débito")

	// Filtro por user_id do cliente (sem carteira → owner_type vazio)
	req = httptest.NewRequest(http.MethodGet, "/chargebacks?user_id=5001", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&chargebacks))
	entries, ok = chargebacks["chargebacks"].([]interface{})
	require.True(t, ok)
	require.Len(t, entries, 1, "filtro user_id=5001 deve retornar só o crédito do cliente")
	creditEntry := entries[0].(map[string]interface{})
	require.Equal(t, "credit", creditEntry["type"])
	require.Equal(t, "", creditEntry["owner_type"], "cliente sem carteira → owner_type vazio")

	// Sem resultados
	req = httptest.NewRequest(http.MethodGet, "/chargebacks?payment_id=nao-existe", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&chargebacks))
	entries, ok = chargebacks["chargebacks"].([]interface{})
	require.True(t, ok)
	require.Empty(t, entries, "sem lançamentos para payment_id inexistente")

	// --- Validação: ID inválido ---
	req = httptest.NewRequest(http.MethodPost, "/payments/abc/approve", nil)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	req = httptest.NewRequest(http.MethodPost, "/payments/ffffffffffffffffffffffff/reject", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode, "hex válido mas inexistente → not found")
}

// TestCheckoutE2E_WebhookRealFlow_Cashback percorre o fluxo completo do
// webhook real do AbacatePay: POST no HandlePaymentWebhook → verificação
// server-side da charge na API (mockada) → atualização do status → split
// com cashback (customer credit > 0), cobrindo a regra de 4 receivers:
// platform, establishment, deliveryman e customer.
func TestCheckoutE2E_WebhookRealFlow_Cashback(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	// Sem Redis: o publish das filas cai no fallback de Go channels (não bloqueia).
	os.Unsetenv("REDIS_URL")

	ctx := context.Background()
	paymentCollection := models.MongoDabase.Collection("payments")
	walletCollection := models.MongoDabase.Collection("wallets")
	ledgerCollection := models.MongoDabase.Collection("wallet_ledger")

	// Mock da API AbacatePay v2: o webhook NÃO confia no body — verifica o
	// status da charge consultando a API (aqui mockada). Conta as chamadas
	// para provar que a verificação server-side aconteceu de verdade.
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
		// Charge expirada → status EXPIRED na API (cenário de não-pagamento).
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
	// delivery=7.00 → sobra 1.99 de cashback (customer credit > 0) → 4 receivers.
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
	_, err := paymentCollection.InsertOne(ctx, payment)
	require.NoError(t, err)

	app := fiber.New()
	app.Post("/api/payment/webhook", HandlePaymentWebhook)

	// Payload real do webhook v2 (evento billing.paid)
	webhookBody := []byte(`{"event":"billing.paid","charge":{"id":"charge-e2e-cashback-001","status":"PAID","amount":89.90}}`)

	// --- 1. HMAC inválido → 401, API não consultada, status intacto ---
	req := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", "assinatura-errada")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 401, resp.StatusCode)
	require.Equal(t, int32(0), atomic.LoadInt32(&mockCalls), "HMAC inválido não deve consultar a API")

	var stored models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-cashback-001"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "PENDING", stored.Status, "HMAC inválido não pode mudar o status")

	// --- 2. Webhook legítimo (HMAC válido) → CONFIRMED + split com cashback ---
	req = httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", computeHMAC(webhookBody, "e2e-webhook-secret"))
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var processed map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&processed))
	require.Equal(t, "processed", processed["status"])

	// A verificação server-side realmente consultou a API mockada.
	require.Equal(t, int32(1), atomic.LoadInt32(&mockCalls), "webhook legítimo deve consultar a API para confirmar a charge")

	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-cashback-001"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "CONFIRMED", stored.Status)
	require.NotNil(t, stored.ConfirmedAt)

	// 4 receivers: platform + establishment + deliveryman + customer (cashback)
	require.Len(t, stored.SplitRules, 4, "cashback > 0 deve gerar a 4ª regra (customer)")

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

	// --- 2b. Crédito real na carteira do restaurante após o split ---
	// A confirmação do webhook deve criar (upsert) a carteira do
	// estabelecimento e creditar o share do split: 89.90 * 85% = 76.415.
	var estWallet models.Wallet
	err = walletCollection.FindOne(ctx, bson.M{"user_id": 42}).Decode(&estWallet)
	require.NoError(t, err, "webhook confirmado deve criar a carteira do estabelecimento")
	require.Equal(t, "establishment", estWallet.UserType)
	require.InDelta(t, 89.90*0.85, estWallet.Balance, 0.01, "carteira do restaurante recebe o share do split")

	// Marcador de crédito gravado no pagamento
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-cashback-001"}).Decode(&stored)
	require.NoError(t, err)
	require.NotNil(t, stored.EstablishmentCreditedAt, "pagamento deve registrar establishment_credited_at")

	// Lançamento de crédito no ledger
	creditCount, err := ledgerCollection.CountDocuments(ctx, bson.M{
		"user_id":    42,
		"type":       "credit",
		"payment_id": "charge-e2e-cashback-001",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), creditCount, "deve existir 1 lançamento de crédito no ledger")

	// --- 3. Webhook idempotente: reprocessar não muda o status nem duplica ---
	req = httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", computeHMAC(webhookBody, "e2e-webhook-secret"))
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-cashback-001"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "CONFIRMED", stored.Status, "reprocessar webhook não pode regredir o status")
	require.Len(t, stored.SplitRules, 4, "reprocessar não pode duplicar split rules")

	// Reprocessar não credita a carteira de novo (idempotência do crédito)
	err = walletCollection.FindOne(ctx, bson.M{"user_id": 42}).Decode(&estWallet)
	require.NoError(t, err)
	require.InDelta(t, 89.90*0.85, estWallet.Balance, 0.01, "reprocessar webhook não pode creditar duas vezes")

	creditCount, err = ledgerCollection.CountDocuments(ctx, bson.M{
		"user_id":    42,
		"type":       "credit",
		"payment_id": "charge-e2e-cashback-001",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), creditCount, "ledger não pode ter crédito duplicado")

	// --- 4. Charge não paga (EXPIRED) → status EXPIRED, sem split ---
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
	_, err = paymentCollection.InsertOne(ctx, expired)
	require.NoError(t, err)

	expiredBody := []byte(`{"event":"billing.expired","charge":{"id":"charge-e2e-expired-001","status":"EXPIRED","amount":50.00}}`)
	req = httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytes.NewReader(expiredBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", computeHMAC(expiredBody, "e2e-webhook-secret"))
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-expired-001"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "EXPIRED", stored.Status)
	require.Empty(t, stored.SplitRules, "charge não paga não pode gerar split")
}

// TestCheckoutE2E_WebhookRealFlow_Refund cobre o fluxo de chargeback/reembolso:
// webhook billing.refunded → verificação server-side (API mockada) → reversão
// do crédito da carteira do estabelecimento → ledger de débito → evento
// PAYMENT_REFUNDED nas filas → status REFUNDED. Também valida idempotência
// (sem débito duplo), refund de pagamento nunca pago (sem reversão) e
// saldo insuficiente (nunca deixa saldo negativo).
func TestCheckoutE2E_WebhookRealFlow_Refund(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	os.Unsetenv("REDIS_URL")

	ctx := context.Background()
	paymentCollection := models.MongoDabase.Collection("payments")
	walletCollection := models.MongoDabase.Collection("wallets")
	ledgerCollection := models.MongoDabase.Collection("wallet_ledger")

	// Mock AbacatePay: charges com "refund" no id → REFUNDED, resto → PAID.
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

	postWebhook := func(t *testing.T, chargeID string, body []byte) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-abacatepay-signature", computeHMAC(body, "e2e-refund-secret"))
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		return resp
	}

	// === Cenário 1: pagamento CONFIRMED → refund reverte a carteira ===
	// amount=100, delivery=10 → split 5/85/10 = 100 (3 regras, est share 85).
	confirmed := models.Payment{
		OrderID:         "order-e2e-refund-001",
		CustomerID:      500,
		EstablishmentID: 42,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-e2e-refund-001",
		SplitRules: []models.SplitRule{
			{ReceiverType: "platform", Amount: 5.00, Percentage: 5.0},
			{ReceiverType: "establishment", Amount: 85.00, Percentage: 85.0},
			{ReceiverType: "deliveryman", Amount: 10.00, Percentage: 0},
		},
		CreatedAt: time.Now(),
	}
	_, err := paymentCollection.InsertOne(ctx, confirmed)
	require.NoError(t, err)

	_, err = walletCollection.InsertOne(ctx, bson.M{
		"user_id":      42,
		"user_type":    "establishment",
		"balance":      200.0,
		"last_updated": time.Now(),
	})
	require.NoError(t, err)

	refundBody := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-001","status":"REFUNDED","amount":100.00}}`)
	postWebhook(t, "charge-e2e-refund-001", refundBody)

	// Verificação server-side aconteceu
	require.Equal(t, int32(1), atomic.LoadInt32(&mockCalls))

	var stored models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-refund-001"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "REFUNDED", stored.Status)
	require.NotNil(t, stored.RefundedAt)
	require.Len(t, stored.SplitRules, 3, "refund não pode regenerar/duplicar split rules")

	// Carteira do estabelecimento debitada: 200 - 85 (share do split) = 115
	var wallet models.Wallet
	err = walletCollection.FindOne(ctx, bson.M{"user_id": 42}).Decode(&wallet)
	require.NoError(t, err)
	require.InDelta(t, 115.0, wallet.Balance, 0.01, "chargeback deve reverter o crédito do estabelecimento")

	// Ledger de débito gravado
	count, err := ledgerCollection.CountDocuments(ctx, bson.M{
		"user_id":    42,
		"type":       "debit",
		"payment_id": "charge-e2e-refund-001",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), count, "deve existir 1 lançamento de débito no ledger")

	// === Cenário 2: idempotência — reprocessar não debita de novo ===
	postWebhook(t, "charge-e2e-refund-001", refundBody)

	err = walletCollection.FindOne(ctx, bson.M{"user_id": 42}).Decode(&wallet)
	require.NoError(t, err)
	require.InDelta(t, 115.0, wallet.Balance, 0.01, "reprocessar webhook não pode debitar duas vezes")

	count, err = ledgerCollection.CountDocuments(ctx, bson.M{
		"user_id":    42,
		"type":       "debit",
		"payment_id": "charge-e2e-refund-001",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), count, "ledger não pode ter débito duplicado")

	// === Cenário 3: refund de pagamento PENDING (nunca pago) → sem reversão ===
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
	_, err = paymentCollection.InsertOne(ctx, pending)
	require.NoError(t, err)

	_, err = walletCollection.InsertOne(ctx, bson.M{
		"user_id":      43,
		"user_type":    "establishment",
		"balance":      500.0,
		"last_updated": time.Now(),
	})
	require.NoError(t, err)

	pendingRefund := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-pending","status":"REFUNDED","amount":50.00}}`)
	postWebhook(t, "charge-e2e-refund-pending", pendingRefund)

	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-refund-pending"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "REFUNDED", stored.Status)

	err = walletCollection.FindOne(ctx, bson.M{"user_id": 43}).Decode(&wallet)
	require.NoError(t, err)
	require.InDelta(t, 500.0, wallet.Balance, 0.01, "pagamento nunca pago não pode reverter crédito")

	// === Cenário 4: saldo insuficiente — nunca fica negativo ===
	low := models.Payment{
		OrderID:         "order-e2e-refund-low",
		CustomerID:      502,
		EstablishmentID: 44,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-e2e-refund-low",
		SplitRules: []models.SplitRule{
			{ReceiverType: "platform", Amount: 5.00, Percentage: 5.0},
			{ReceiverType: "establishment", Amount: 85.00, Percentage: 85.0},
			{ReceiverType: "deliveryman", Amount: 10.00, Percentage: 0},
		},
		CreatedAt: time.Now(),
	}
	_, err = paymentCollection.InsertOne(ctx, low)
	require.NoError(t, err)

	_, err = walletCollection.InsertOne(ctx, bson.M{
		"user_id":      44,
		"user_type":    "establishment",
		"balance":      10.0, // < share de 85
		"last_updated": time.Now(),
	})
	require.NoError(t, err)

	lowRefund := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-low","status":"REFUNDED","amount":100.00}}`)
	postWebhook(t, "charge-e2e-refund-low", lowRefund)

	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-refund-low"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "REFUNDED", stored.Status)

	err = walletCollection.FindOne(ctx, bson.M{"user_id": 44}).Decode(&wallet)
	require.NoError(t, err)
	require.InDelta(t, 10.0, wallet.Balance, 0.01, "saldo insuficiente: débito bloqueado, nunca negativo")

	// Ledger não pode ter débito quando o saldo é insuficiente
	count, err = ledgerCollection.CountDocuments(ctx, bson.M{
		"user_id":    44,
		"type":       "debit",
		"payment_id": "charge-e2e-refund-low",
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	// === Cenário 5: cashback do cliente (receiver_type customer) é revertido ===
	// amount=100, delivery=10, split 5/85/10 → sobra 0; adicionamos uma regra
	// customer (cashback 3) simulando zona com cashback → 4 receivers. O
	// chargeback deve debitar a carteira do cliente pelo cashback, além do
	// débito do estabelecimento.
	cashback := models.Payment{
		OrderID:         "order-e2e-refund-cashback",
		CustomerID:      503,
		EstablishmentID: 45,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-e2e-refund-cashback",
		SplitRules: []models.SplitRule{
			{ReceiverType: "platform", Amount: 5.00, Percentage: 5.0},
			{ReceiverType: "establishment", Amount: 82.00, Percentage: 82.0},
			{ReceiverType: "deliveryman", Amount: 10.00, Percentage: 0},
			{ReceiverType: "customer", Amount: 3.00, Percentage: 0},
		},
		CreatedAt: time.Now(),
	}
	_, err = paymentCollection.InsertOne(ctx, cashback)
	require.NoError(t, err)

	// Carteira do estabelecimento (com crédito do share 82) e carteira do
	// cliente (com crédito de cashback 3).
	_, err = walletCollection.InsertOne(ctx, bson.M{
		"user_id":      45,
		"user_type":    "establishment",
		"balance":      182.0,
		"last_updated": time.Now(),
	})
	require.NoError(t, err)
	_, err = walletCollection.InsertOne(ctx, bson.M{
		"user_id":      503,
		"user_type":    "customer",
		"balance":      13.0, // 10 de saldo prévio + 3 de cashback
		"last_updated": time.Now(),
	})
	require.NoError(t, err)

	cashbackRefund := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-cashback","status":"REFUNDED","amount":100.00}}`)
	postWebhook(t, "charge-e2e-refund-cashback", cashbackRefund)

	err = walletCollection.FindOne(ctx, bson.M{"user_id": 45}).Decode(&wallet)
	require.NoError(t, err)
	require.InDelta(t, 100.0, wallet.Balance, 0.01, "chargeback reverte o crédito do estabelecimento (182 - 82)")

	err = walletCollection.FindOne(ctx, bson.M{"user_id": 503}).Decode(&wallet)
	require.NoError(t, err)
	require.InDelta(t, 10.0, wallet.Balance, 0.01, "chargeback reverte o cashback do cliente (13 - 3)")

	// Ledger: débito do cashback do cliente gravado
	count, err = ledgerCollection.CountDocuments(ctx, bson.M{
		"user_id":    503,
		"type":       "debit",
		"payment_id": "charge-e2e-refund-cashback",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), count, "deve existir 1 débito de cashback do cliente no ledger")

	// === Cenário 6: top-up de carteira quando o pagamento foi usado ===
	// Pagamento CONFIRMED com wallet_credited_at preenchido (cliente usou o
	// pagamento para top-up da carteira). O chargeback deve reverter o valor
	// total do top-up (payment.Amount) da carteira do cliente.
	topup := models.Payment{
		OrderID:         "order-e2e-refund-topup",
		CustomerID:      504,
		EstablishmentID: 46,
		Amount:          100.00,
		DeliveryAmount:  10.00,
		Method:          "pix",
		Status:          "CONFIRMED",
		AbacatePayID:    "charge-e2e-refund-topup",
		SplitRules: []models.SplitRule{
			{ReceiverType: "platform", Amount: 5.00, Percentage: 5.0},
			{ReceiverType: "establishment", Amount: 85.00, Percentage: 85.0},
			{ReceiverType: "deliveryman", Amount: 10.00, Percentage: 0},
		},
		CreatedAt:        time.Now(),
		WalletCreditedAt: ptrTime(time.Now().Add(-time.Hour)),
	}
	_, err = paymentCollection.InsertOne(ctx, topup)
	require.NoError(t, err)

	_, err = walletCollection.InsertOne(ctx, bson.M{
		"user_id":      504,
		"user_type":    "customer",
		"balance":      250.0, // 150 prévio + 100 do top-up
		"last_updated": time.Now(),
	})
	require.NoError(t, err)

	topupRefund := []byte(`{"event":"billing.refunded","charge":{"id":"charge-e2e-refund-topup","status":"REFUNDED","amount":100.00}}`)
	postWebhook(t, "charge-e2e-refund-topup", topupRefund)

	err = walletCollection.FindOne(ctx, bson.M{"user_id": 504}).Decode(&wallet)
	require.NoError(t, err)
	require.InDelta(t, 150.0, wallet.Balance, 0.01, "chargeback reverte o top-up de carteira (250 - 100)")

	// Ledger: débito do top-up do cliente gravado
	count, err = ledgerCollection.CountDocuments(ctx, bson.M{
		"user_id":    504,
		"type":       "debit",
		"payment_id": "charge-e2e-refund-topup",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), count, "deve existir 1 débito do top-up no ledger")
}

// TestCheckoutE2E_WebhookRealFlow_ZoneSplitConfig valida que o split usa os
// percentuais customizados da zona do estabelecimento (via
// GetSplitConfigForEstablishment, ligado pelo monólito) em vez do default
// 5/85. Com zona 7%/80%, amount=100 e delivery=10: platform=7, estab=80,
// delivery=10, cashback=3 → 4 receivers.
func TestCheckoutE2E_WebhookRealFlow_ZoneSplitConfig(t *testing.T) {
	cleanup := setupCheckoutE2EEnv(t)
	defer cleanup()

	os.Unsetenv("REDIS_URL")

	ctx := context.Background()
	paymentCollection := models.MongoDabase.Collection("payments")
	walletCollection := models.MongoDabase.Collection("wallets")

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

	// Ligação que o monólito faz em main(): resolver de split por zona.
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
	_, err := paymentCollection.InsertOne(ctx, payment)
	require.NoError(t, err)

	webhookBody := []byte(`{"event":"billing.paid","charge":{"id":"charge-e2e-zone-split","status":"PAID","amount":100.00}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-abacatepay-signature", computeHMAC(webhookBody, "e2e-zone-secret"))
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	// Verificação server-side + resolver de zona chamados de verdade.
	require.Equal(t, int32(1), atomic.LoadInt32(&mockCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&resolverCalls), "GetSplitConfigForEstablishment deve ser consultado no webhook")

	var stored models.Payment
	err = paymentCollection.FindOne(ctx, bson.M{"abacatepay_id": "charge-e2e-zone-split"}).Decode(&stored)
	require.NoError(t, err)
	require.Equal(t, "CONFIRMED", stored.Status)

	// Split com percentuais da zona: 7/80/10 + cashback 3 → 4 regras.
	require.Len(t, stored.SplitRules, 4, "7%%+80%%+delivery 10 = 97 → sobra 3 de cashback → 4 regras")

	require.Equal(t, "platform", stored.SplitRules[0].ReceiverType)
	require.InDelta(t, 7.0, stored.SplitRules[0].Amount, 0.01, "7%% de 100")
	require.InDelta(t, 7.0, stored.SplitRules[0].Percentage, 0.01)

	require.Equal(t, "establishment", stored.SplitRules[1].ReceiverType)
	require.InDelta(t, 80.0, stored.SplitRules[1].Amount, 0.01, "80%% de 100")
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

	// Controle: se o resolver NÃO tivesse sido usado, o split seria 5/85
	// (platform 5, estab 85, delivery 10, cashback 0 → 3 regras). As regras
	// acima só existem com a config da zona — prova que o default foi trocado.
	require.Greater(t, stored.SplitRules[0].Percentage, 5.0, "percentual da plataforma deve vir da zona (7%%), nao do default 5%%")
	require.Less(t, stored.SplitRules[1].Percentage, 85.0, "percentual do estabelecimento deve vir da zona (80%%), nao do default 85%%")

	// Carteira do restaurante creditada pelo share da ZONA (80%), não 85%.
	var estWallet models.Wallet
	err = walletCollection.FindOne(ctx, bson.M{"user_id": 42}).Decode(&estWallet)
	require.NoError(t, err, "webhook confirmado deve criar a carteira do estabelecimento")
	require.Equal(t, "establishment", estWallet.UserType)
	require.InDelta(t, 80.0, estWallet.Balance, 0.01, "carteira recebe o share da zona (80), não o default 85")
}
