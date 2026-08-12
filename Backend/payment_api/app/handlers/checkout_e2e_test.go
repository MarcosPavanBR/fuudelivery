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
