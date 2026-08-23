// Package models - message_test.go
// Unit tests for the ChatMessage model (Postgres/GORM — corte 2 banco-único).
// Tests struct creation, JSON tag compliance, default values,
// and message type / sender type validation.
package models

import (
	"encoding/json"
	"testing"
	"time"
)

// === Testes de criacao do ChatMessage ===

func TestChatMessage_Creation(t *testing.T) {
	now := time.Now()
	msg := ChatMessage{
		ID:          42,
		OrderID:     "order_123",
		SenderID:    1001,
		SenderType:  "customer",
		SenderName:  "Joao",
		Message:     "Hello, where is my order?",
		MessageType: "text",
		CreatedAt:   now,
	}

	if msg.ID != 42 {
		t.Errorf("ID: got %d, want %d", msg.ID, 42)
	}
	if msg.OrderID != "order_123" {
		t.Errorf("OrderID: got %q, want %q", msg.OrderID, "order_123")
	}
	if msg.SenderID != 1001 {
		t.Errorf("SenderID: got %d, want %d", msg.SenderID, 1001)
	}
	if msg.SenderType != "customer" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "customer")
	}
	if msg.SenderName != "Joao" {
		t.Errorf("SenderName: got %q, want %q", msg.SenderName, "Joao")
	}
	if msg.Message != "Hello, where is my order?" {
		t.Errorf("Message: got %q, want %q", msg.Message, "Hello, where is my order?")
	}
	if msg.MessageType != "text" {
		t.Errorf("MessageType: got %q, want %q", msg.MessageType, "text")
	}
	if msg.CreatedAt != now {
		t.Errorf("CreatedAt: got %v, want %v", msg.CreatedAt, now)
	}
}

func TestChatMessage_DefaultID_IsZero(t *testing.T) {
	msg := ChatMessage{}
	if msg.ID != 0 {
		t.Errorf("ID default deveria ser 0 (BIGSERIAL preenche no INSERT), got %d", msg.ID)
	}
}

func TestChatMessage_DefaultCreatedAt_IsZero(t *testing.T) {
	msg := ChatMessage{}
	if !msg.CreatedAt.IsZero() {
		t.Error("CreatedAt default deveria ser zero time")
	}
}

func TestChatMessage_DefaultReadAt_IsNil(t *testing.T) {
	msg := ChatMessage{}
	if msg.ReadAt != nil {
		t.Error("ReadAt default deveria ser nil (mensagem não lida)")
	}
}

func TestChatMessage_DefaultImageURL_IsEmpty(t *testing.T) {
	msg := ChatMessage{}
	if msg.ImageURL != "" {
		t.Error("ImageURL default deveria ser vazio")
	}
}

func TestChatMessage_TextMessageType(t *testing.T) {
	msg := ChatMessage{MessageType: "text"}
	if msg.MessageType != "text" {
		t.Errorf("MessageType: got %q, want %q", msg.MessageType, "text")
	}
}

func TestChatMessage_ImageMessageType(t *testing.T) {
	msg := ChatMessage{
		MessageType: "image",
		ImageURL:    "https://example.com/photo.jpg",
	}
	if msg.MessageType != "image" {
		t.Errorf("MessageType: got %q, want %q", msg.MessageType, "image")
	}
	if msg.ImageURL == "" {
		t.Error("mensagem de imagem deveria ter ImageURL")
	}
}

func TestChatMessage_CustomerSenderType(t *testing.T) {
	msg := ChatMessage{SenderType: "client"}
	if msg.SenderType != "client" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "client")
	}
}

func TestChatMessage_RestaurantSenderType(t *testing.T) {
	msg := ChatMessage{SenderType: "establishment"}
	if msg.SenderType != "establishment" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "establishment")
	}
}

func TestChatMessage_DeliverySenderType(t *testing.T) {
	msg := ChatMessage{SenderType: "delivery_man"}
	if msg.SenderType != "delivery_man" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "delivery_man")
	}
}

func TestChatMessage_ReadAt(t *testing.T) {
	now := time.Now()
	msg := ChatMessage{ReadAt: &now}
	if msg.ReadAt == nil {
		t.Fatal("ReadAt deveria estar preenchido")
	}
	if !msg.ReadAt.Equal(now) {
		t.Errorf("ReadAt: got %v, want %v", msg.ReadAt, now)
	}
}

func TestChatMessage_ReadAtNil(t *testing.T) {
	msg := ChatMessage{}
	if msg.ReadAt != nil {
		t.Error("ReadAt deveria ser nil quando mensagem não foi lida")
	}
}

func TestChatMessage_TableName(t *testing.T) {
	// A tabela DEVE se chamar chat_messages — é o nome criado por
	// sql/04_dominio_chat.sql. Se mudar, o AutoMigrate cria tabela errada.
	if got := (ChatMessage{}).TableName(); got != "chat_messages" {
		t.Errorf("TableName(): got %q, want %q", got, "chat_messages")
	}
}

// === Testes de JSON (contrato com os apps) ===

func TestChatMessage_JSONMarshal(t *testing.T) {
	now := time.Now()
	msg := ChatMessage{
		ID:          7,
		OrderID:     "order_json",
		SenderID:    2002,
		SenderType:  "establishment",
		SenderName:  "Pizzaria",
		Message:     "Seu pedido saiu!",
		MessageType: "text",
		CreatedAt:   now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("falha ao serializar: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("falha ao desserializar: %v", err)
	}

	for _, key := range []string{"id", "order_id", "sender_id", "sender_type", "sender_name", "message", "message_type", "created_at"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON deveria conter a chave %q", key)
		}
	}
}

func TestChatMessage_JSONUnmarshal(t *testing.T) {
	payload := `{"id":9,"order_id":"order_u","sender_id":1,"sender_type":"client","sender_name":"Maria","message":"Oi","message_type":"text","created_at":"2026-08-23T12:00:00Z"}`

	var msg ChatMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		t.Fatalf("falha ao desserializar: %v", err)
	}

	if msg.ID != 9 {
		t.Errorf("ID: got %d, want 9", msg.ID)
	}
	if msg.OrderID != "order_u" {
		t.Errorf("OrderID: got %q, want %q", msg.OrderID, "order_u")
	}
	if msg.SenderID != 1 {
		t.Errorf("SenderID: got %d, want 1", msg.SenderID)
	}
	if msg.Message != "Oi" {
		t.Errorf("Message: got %q, want %q", msg.Message, "Oi")
	}
}

func TestChatMessage_JSONWithReadAt(t *testing.T) {
	now := time.Now()
	msg := ChatMessage{ID: 1, ReadAt: &now}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("falha ao serializar: %v", err)
	}

	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	if _, ok := m["read_at"]; !ok {
		t.Error("JSON deveria conter read_at quando preenchido")
	}
}

func TestChatMessage_JSONImageURLOmitted(t *testing.T) {
	msg := ChatMessage{ID: 1, MessageType: "text"}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("falha ao serializar: %v", err)
	}

	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	if _, ok := m["image_url"]; ok {
		t.Error("JSON NÃO deveria conter image_url quando vazio (omitempty)")
	}
}

func TestChatMessage_JSONImageURLPresent(t *testing.T) {
	msg := ChatMessage{ID: 1, ImageURL: "https://example.com/img.png"}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("falha ao serializar: %v", err)
	}

	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	if m["image_url"] != "https://example.com/img.png" {
		t.Errorf("image_url: got %v, want https://example.com/img.png", m["image_url"])
	}
}

// === Testes de valor zero ===

func TestChatMessage_ZeroValue(t *testing.T) {
	var msg ChatMessage

	if msg.ID != 0 || msg.OrderID != "" || msg.SenderID != 0 ||
		msg.SenderType != "" || msg.SenderName != "" || msg.Message != "" ||
		msg.MessageType != "" || msg.ImageURL != "" {
		t.Error("valor zero do ChatMessage deveria ter todos os campos vazios")
	}
	if msg.ReadAt != nil {
		t.Error("ReadAt deveria ser nil no valor zero")
	}
}
