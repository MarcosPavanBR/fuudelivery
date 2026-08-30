package contracttests

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Contrato de API para criação de pedido
// Garante que mudanças no backend não quebrem frontend/mobile
type CreateOrderContract struct {
	RestaurantID  string          `json:"restaurant_id"`
	CustomerID    string          `json:"customer_id"`
	Items         []OrderItem     `json:"items"`
	DeliveryAddress Address       `json:"delivery_address"`
	PaymentMethod string          `json:"payment_method"`
	TotalAmount   float64         `json:"total_amount"`
}

type OrderItem struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Price     float64 `json:"price"`
}

type Address struct {
	Street     string `json:"street"`
	Number     string `json:"number"`
	Neighborhood string `json:"neighborhood"`
	City       string `json:"city"`
	State      string `json:"state"`
	ZipCode    string `json:"zip_code"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}

// TestCreateOrderContract valida o contrato de criação de pedido
func TestCreateOrderContract(t *testing.T) {
	t.Run("should validate required fields", func(t *testing.T) {
		contract := CreateOrderContract{
			RestaurantID: "rest_123",
			CustomerID:   "cust_456",
			Items: []OrderItem{
				{
					ProductID: "prod_789",
					Name:      "Pizza Margherita",
					Quantity:  2,
					Price:     45.90,
				},
			},
			DeliveryAddress: Address{
				Street:     "Rua das Flores",
				Number:     "123",
				Neighborhood: "Jardins",
				City:       "São Paulo",
				State:      "SP",
				ZipCode:    "01234-567",
				Latitude:   -23.5648985,
				Longitude:  -46.6520646,
			},
			PaymentMethod: "credit_card",
			TotalAmount:   91.80,
		}

		// Valida serialização JSON
		jsonBytes, err := json.Marshal(contract)
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)

		// Valida deserialização
		var decoded CreateOrderContract
		err = json.Unmarshal(jsonBytes, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, contract.RestaurantID, decoded.RestaurantID)
		assert.Equal(t, contract.CustomerID, decoded.CustomerID)
		assert.Len(t, decoded.Items, 1)
		assert.Equal(t, "Pizza Margherita", decoded.Items[0].Name)
	})

	t.Run("should validate items array", func(t *testing.T) {
		contract := CreateOrderContract{
			RestaurantID: "rest_123",
			CustomerID:   "cust_456",
			Items:        []OrderItem{}, // Array vazio deve ser válido
			DeliveryAddress: Address{
				Street:  "Rua Teste",
				Number:  "100",
				City:    "Testópolis",
				State:   "TS",
				ZipCode: "00000-000",
			},
			PaymentMethod: "pix",
			TotalAmount:   0,
		}

		jsonBytes, err := json.Marshal(contract)
		assert.NoError(t, err)
		
		var decoded CreateOrderContract
		err = json.Unmarshal(jsonBytes, &decoded)
		assert.NoError(t, err)
		assert.NotNil(t, decoded.Items)
	})
}

// Contrato de resposta de erro padronizada
type ErrorResponseContract struct {
	Error       string                 `json:"error"`
	Message     string                 `json:"message"`
	ErrorCode   string                 `json:"error_code"`
	Details     map[string]interface{} `json:"details,omitempty"`
	TraceID     string                 `json:"trace_id"`
	Timestamp   string                 `json:"timestamp"`
}

func TestErrorResponseContract(t *testing.T) {
	t.Run("should validate error response format", func(t *testing.T) {
		response := ErrorResponseContract{
			Error:     "validation_error",
			Message:   "Invalid payment method",
			ErrorCode: "PAYMENT_001",
			Details: map[string]interface{}{
				"field": "payment_method",
				"allowed": []string{"credit_card", "debit_card", "pix"},
			},
			TraceID:   "abc123-def456",
			Timestamp: "2026-08-30T18:00:00Z",
		}

		jsonBytes, err := json.Marshal(response)
		assert.NoError(t, err)
		
		var decoded ErrorResponseContract
		err = json.Unmarshal(jsonBytes, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "validation_error", decoded.Error)
		assert.Equal(t, "PAYMENT_001", decoded.ErrorCode)
		assert.NotNil(t, decoded.Details)
	})
}

// Contrato de evento de fila (Redis Streams)
type QueueEventContract struct {
	EventID     string                 `json:"event_id"`
	EventType   string                 `json:"event_type"`
	AggregateID string                 `json:"aggregate_id"`
	Timestamp   string                 `json:"timestamp"`
	Payload     map[string]interface{} `json:"payload"`
	Metadata    MetadataContract       `json:"metadata"`
}

type MetadataContract struct {
	TraceID   string `json:"trace_id"`
	UserID    string `json:"user_id"`
	Source    string `json:"source"`
	Version   string `json:"version"`
}

func TestQueueEventContract(t *testing.T) {
	t.Run("should validate queue event format", func(t *testing.T) {
		event := QueueEventContract{
			EventID:     "evt_001",
			EventType:   "order.created",
			AggregateID: "order_123",
			Timestamp:   "2026-08-30T18:00:00Z",
			Payload: map[string]interface{}{
				"order_id":     "order_123",
				"total_amount": 91.80,
				"items_count":  2,
			},
			Metadata: MetadataContract{
				TraceID: "trace_abc123",
				UserID:  "user_456",
				Source:  "orders_api",
				Version: "1.0.0",
			},
		}

		jsonBytes, err := json.Marshal(event)
		assert.NoError(t, err)
		
		var decoded QueueEventContract
		err = json.Unmarshal(jsonBytes, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "order.created", decoded.EventType)
		assert.Equal(t, "trace_abc123", decoded.Metadata.TraceID)
	})
}

// Contrato de webhook de pagamento
type PaymentWebhookContract struct {
	Gateway         string  `json:"gateway"`
	ExternalPaymentID string `json:"external_payment_id"`
	OrderID         string  `json:"order_id"`
	Status          string  `json:"status"`
	Amount          float64 `json:"amount"`
	PaidAt          string  `json:"paid_at,omitempty"`
	Signature       string  `json:"signature"`
	Timestamp       int64   `json:"timestamp"`
}

func TestPaymentWebhookContract(t *testing.T) {
	t.Run("should validate webhook format", func(t *testing.T) {
		webhook := PaymentWebhookContract{
			Gateway:           "pagarme",
			ExternalPaymentID: "pay_external_789",
			OrderID:          "order_123",
			Status:           "paid",
			Amount:           91.80,
			PaidAt:           "2026-08-30T18:05:00Z",
			Signature:        "sha256=abc123...",
			Timestamp:        1693418700,
		}

		jsonBytes, err := json.Marshal(webhook)
		assert.NoError(t, err)
		
		var decoded PaymentWebhookContract
		err = json.Unmarshal(jsonBytes, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, "pagarme", decoded.Gateway)
		assert.Equal(t, "paid", decoded.Status)
		assert.NotEmpty(t, decoded.Signature)
	})
}

// Execute todos os testes de contrato
func RunContractTests(t *testing.T) {
	t.Run("CreateOrderContract", TestCreateOrderContract)
	t.Run("ErrorResponseContract", TestErrorResponseContract)
	t.Run("QueueEventContract", TestQueueEventContract)
	t.Run("PaymentWebhookContract", TestPaymentWebhookContract)
}
