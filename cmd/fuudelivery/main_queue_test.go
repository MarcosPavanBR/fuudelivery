package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProcessStatusUpdateMalformed: mensagem que não é JSON válido deve
// retornar erro — é o caminho que ativa retry + DLQ no pkg/queue.
func TestProcessStatusUpdateMalformed(t *testing.T) {
	err := processStatusUpdate("payment_updates", []byte("{not-json"))
	if err == nil {
		t.Fatal("esperava erro para mensagem malformada (ativa retry/DLQ)")
	}
	if !strings.Contains(err.Error(), "mensagem inválida") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

// TestProcessStatusUpdateSemDestinatario: community_fallback não carrega
// courier_id/client_id — é informativa e não pode ir para a DLQ.
func TestProcessStatusUpdateSemDestinatario(t *testing.T) {
	err := processStatusUpdate("delivery_updates", []byte(
		`{"type":"community_fallback","order_id":"42","zone_name":"Centro"}`))
	if err != nil {
		t.Fatalf("esperava nil para mensagem sem destinatário, got: %v", err)
	}
}

// TestProcessStatusUpdateClienteOffline: destinatário resolvido mas sem
// conexão WebSocket ativa — sendMessageToClient loga e retorna nil.
func TestProcessStatusUpdateClienteOffline(t *testing.T) {
	err := processStatusUpdate("delivery_updates", []byte(
		`{"type":"order_matched","order_id":"42","courier_id":99}`))
	if err != nil {
		t.Fatalf("esperava nil para cliente offline, got: %v", err)
	}
}

// TestResolveStatusRecipient: prioridade client_id > user_id > courier_id,
// com courier_id restrito à fila de delivery.
func TestResolveStatusRecipient(t *testing.T) {
	cases := []struct {
		name      string
		queueName string
		evt       statusEvent
		want      int64
	}{
		{"client_id preferido sobre user_id", "payment_updates", statusEvent{ClientID: 7, UserID: 3}, 7},
		{"user_id usado sem client_id", "order_updates", statusEvent{UserID: 3, CourierID: 5}, 3},
		{"courier_id usado na fila de delivery", "delivery_updates", statusEvent{CourierID: 5}, 5},
		{"courier_id ignorado fora da fila de delivery", "order_updates", statusEvent{CourierID: 5}, 0},
		{"sem destinatario -> 0", "delivery_updates", statusEvent{OrderID: "1"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveStatusRecipient(tc.queueName, &tc.evt); got != tc.want {
				t.Fatalf("resolveStatusRecipient(%q, %+v) = %d, esperava %d",
					tc.queueName, tc.evt, got, tc.want)
			}
		})
	}
}

// TestProcessStatusUpdatePaymentRefund: o webhook de estorno publica
// PAYMENT_REFUNDED na fila payment_updates com user_id (cliente) — o
// monolito deve resolver o destinatário e notificar (offline → nil, sem DLQ).
func TestProcessStatusUpdatePaymentRefund(t *testing.T) {
	err := processStatusUpdate("payment_updates", []byte(
		`{"type":"PAYMENT_REFUNDED","order_id":"42","payment_id":"pay-9","user_id":7,"status":"REFUNDED","amount":100,"method":"pix","refunded_at":"2026-08-12T10:00:00Z"}`))
	if err != nil {
		t.Fatalf("esperava nil para evento de refund com cliente offline, got: %v", err)
	}
}

// TestResolveStatusRecipientPaymentRefund: o evento de estorno publica
// user_id (CustomerID do pagamento) — o destinatário WebSocket é esse usuário.
func TestResolveStatusRecipientPaymentRefund(t *testing.T) {
	evt := statusEvent{Type: "PAYMENT_REFUNDED", OrderID: "42", UserID: 7}
	if got := resolveStatusRecipient("payment_updates", &evt); got != 7 {
		t.Fatalf("resolveStatusRecipient(PAYMENT_REFUNDED) = %d, esperava 7", got)
	}
}

// TestStatusEventJSONRefund: o payload PAYMENT_REFUNDED publicado pelo
// payment_api decodifica no struct do monolito sem erro (contrato das filas).
func TestStatusEventJSONRefund(t *testing.T) {
	raw := `{"type":"PAYMENT_REFUNDED","order_id":"42","payment_id":"pay-9","user_id":7,"status":"REFUNDED","amount":100,"method":"pix","refunded_at":"2026-08-12T10:00:00Z"}`
	var evt statusEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("falha ao decodificar mensagem de refund: %v", err)
	}
	if evt.Type != "PAYMENT_REFUNDED" || evt.OrderID != "42" || evt.UserID != 7 {
		t.Fatalf("decode incorreto do refund: %+v", evt)
	}
}

// TestStatusEventJSON: garante que o struct decodifica o formato publicado
// pelo dispatch engine do monolito (order_matched).
func TestStatusEventJSON(t *testing.T) {
	raw := `{"type":"order_matched","order_id":"123","courier_id":42,"matched_at":"2026-08-10T12:00:00Z"}`
	var evt statusEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("falha ao decodificar mensagem do dispatch: %v", err)
	}
	if evt.Type != "order_matched" || evt.OrderID != "123" || evt.CourierID != 42 {
		t.Fatalf("decode incorreto: %+v", evt)
	}
}
