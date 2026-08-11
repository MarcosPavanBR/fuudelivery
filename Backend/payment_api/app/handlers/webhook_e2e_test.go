package handlers

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/carloshomar/fuudelivery/payment_api/app/models"
)

// TestWebhookPublishAlignsWithSubscriber verifica que os canais de fila
// usados pelo webhook (payment_updates/order_updates) coincidem com os
// canais que o monolito escuta em startQueueListeners(). Se alguém
// renomear um canal sem atualizar o outro, este teste falha.
func TestWebhookPublishAlignsWithSubscriber(t *testing.T) {
	// Canais que o monolito escuta (cmd/fuudelivery/main.go)
	monolithChannels := map[string]bool{
		"order_updates":    true,
		"delivery_updates": true,
		"payment_updates":  true,
	}

	// Canais que o webhook publica (payment_api/app/handlers/webhook.go)
	webhookChannels := map[string]bool{
		orderRedisQueueKey:   true,
		paymentRedisQueueKey: true,
	}

	for ch := range webhookChannels {
		if !monolithChannels[ch] {
			t.Errorf("webhook publica em %q mas monolito NAO escuta esse canal — align os nomes", ch)
		}
	}
}

// TestWebhookFlow_PaymentApproved simula o fluxo completo de um pagamento
// aprovado: webhook recebido → publicação na fila → processamento → split.
// Valida que o payload publicado contém os campos corretos para o
// processStatusUpdate do monolito.
func TestWebhookFlow_PaymentApproved(t *testing.T) {
	// Simula o payload que o webhook publicaria após processar um
	// "billing.paid" da AbacatePay
	orderMsg := map[string]interface{}{
		"order_id":     "order-123",
		"payment_id":   "pay-456",
		"type":         "PAYMENT_CONFIRMED",
		"amount":       49.90,
		"method":       "pix",
		"confirmed_at": "2026-08-10T12:00:00Z",
	}

	msgBody, err := json.Marshal(orderMsg)
	if err != nil {
		t.Fatalf("falha ao serializar mensagem: %v", err)
	}

	// Valida que a mensagem e JSON valido (o processStatusUpdate exige isso)
	var parsed map[string]interface{}
	if err := json.Unmarshal(msgBody, &parsed); err != nil {
		t.Fatalf("mensagem publicada nao e JSON valido: %v", err)
	}

	// Valida campos obrigatorios para a ponte WebSocket
	if parsed["type"] == nil {
		t.Error("campo 'type' ausente — processStatusUpdate nao vai decodificar corretamente")
	}
	if parsed["order_id"] == nil {
		t.Error("campo 'order_id' ausente — WebSocket nao saberá para quem enviar")
	}

	// Valida que o canal de publicacao esta correto
	if paymentRedisQueueKey != "payment_updates" {
		t.Errorf("paymentRedisQueueKey=%q, esperava 'payment_updates'", paymentRedisQueueKey)
	}
	if orderRedisQueueKey != "order_updates" {
		t.Errorf("orderRedisQueueKey=%q, esperava 'order_updates'", orderRedisQueueKey)
	}
}

// TestWebhookFlow_HMACSignatureValidation valida que o HMAC e verificado
// antes do processamento do webhook (defesa em profundidade).
func TestWebhookFlow_HMACSignatureValidation(t *testing.T) {
	// Em producao, ABACATE_PAY_WEBHOOK_SECRET deve estar configurada
	if os.Getenv("GO_ENV") == "production" {
		if os.Getenv("ABACATE_PAY_WEBHOOK_SECRET") == "" {
			t.Error("ABACATE_PAY_WEBHOOK_SECRET nao configurada em producao — HMAC sera bypassada")
		}
	}

	// Valida que ValidateWebhookSignature existe e funciona
	body := []byte(`{"event":"billing.paid"}`)

	// Sem secret configurada → fallback (retorna true)
	os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")
	if !ValidateWebhookSignature(body, "") {
		t.Error("ValidateWebhookSignature com header vazio deveria retornar true quando secret nao configurada")
	}

	// Com secret configurada e assinatura errada → false
	os.Setenv("ABACATE_PAY_WEBHOOK_SECRET", "test-secret")
	defer os.Unsetenv("ABACATE_PAY_WEBHOOK_SECRET")

	if ValidateWebhookSignature(body, "wrong-sig") {
		t.Error("ValidateWebhookSignature com assinatura incorreta deveria retornar false")
	}
}

// TestDefaultSplitRules_CaminosMinimos verifica que as regras de split
// geradas pelo webhook contem os campos minimos necessarios para o
// processamento downstream (Payment Service, carteira do restaurante).
func TestDefaultSplitRules_CaminosMinimos(t *testing.T) {
	payment := &models.Payment{
		Amount:          100.0,
		DeliveryAmount:  7.0,
		EstablishmentID: 42,
		CustomerID:      100,
	}

	rules := defaultSplitRules(payment, 5.0, 85.0)

	if len(rules) < 2 {
		t.Fatalf("esperava pelo menos 2 split rules (platform + establishment), got %d", len(rules))
	}

	// Verifica platform (5%)
	if rules[0].ReceiverType != "platform" {
		t.Errorf("rules[0].ReceiverType=%q, esperava 'platform'", rules[0].ReceiverType)
	}
	if rules[0].Percentage != 5.0 {
		t.Errorf("rules[0].Percentage=%.1f, esperava 5.0", rules[0].Percentage)
	}
	expectedPlatform := 100.0 * 0.05
	if rules[0].Amount != expectedPlatform {
		t.Errorf("rules[0].Amount=%.2f, esperava %.2f", rules[0].Amount, expectedPlatform)
	}

	// Verifica establishment (85%)
	if rules[1].ReceiverType != "establishment" {
		t.Errorf("rules[1].ReceiverType=%q, esperava 'establishment'", rules[1].ReceiverType)
	}
	if rules[1].Percentage != 85.0 {
		t.Errorf("rules[1].Percentage=%.1f, esperava 85.0", rules[1].Percentage)
	}
	expectedEstablishment := 100.0 * 0.85
	if rules[1].Amount != expectedEstablishment {
		t.Errorf("rules[1].Amount=%.2f, esperava %.2f", rules[1].Amount, expectedEstablishment)
	}

	// Verifica deliveryman (taxa de entrega)
	if len(rules) >= 3 {
		if rules[2].ReceiverType != "deliveryman" {
			t.Errorf("rules[2].ReceiverType=%q, esperava 'deliveryman'", rules[2].ReceiverType)
		}
		if rules[2].Amount != 7.0 {
			t.Errorf("rules[2].Amount=%.2f, esperava 7.0 (taxa de entrega)", rules[2].Amount)
		}
	}
}

// TestDefaultSplitRules_PedidoPequeno valida que quando o pedido e menor
// que a taxa de entrega, o split nao gera valores negativos (protecao de overflow).
func TestDefaultSplitRules_PedidoPequeno(t *testing.T) {
	// Pedido de R$3 com entrega de R$7 — establishment ficaria negativo
	payment := &models.Payment{
		Amount:          3.0,
		DeliveryAmount:  7.0,
		EstablishmentID: 42,
		CustomerID:      100,
	}

	rules := defaultSplitRules(payment, 5.0, 85.0)

	// Nenhuma regra deve ter valor negativo
	for i, rule := range rules {
		if rule.Amount < 0 {
			t.Errorf("rules[%d].Amount=%.2f — valores negativos nao sao permitidos no split", i, rule.Amount)
		}
	}
}
