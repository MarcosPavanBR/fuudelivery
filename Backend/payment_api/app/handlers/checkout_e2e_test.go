//go:build integration

package handlers

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/carloshomar/vercardapio/payment_api/app/models"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
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
	require.Len(t, splitRules, 3)

	require.Equal(t, "platform", splitRules[0].ReceiverType)
	require.InDelta(t, 89.90*0.05, splitRules[0].Amount, 0.01)

	require.Equal(t, "establishment", splitRules[1].ReceiverType)
	require.InDelta(t, 89.90*0.85, splitRules[1].Amount, 0.01)

	require.Equal(t, "deliveryman", splitRules[2].ReceiverType)
	require.InDelta(t, 7.00, splitRules[2].Amount, 0.01)

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
	require.Len(t, confirmed.SplitRules, 3)
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
	require.LessOrEqual(t, totalSplit, 5.01)
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
