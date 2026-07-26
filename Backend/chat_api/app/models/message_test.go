// Package models - message_test.go
// Unit tests for the ChatMessage model.
// Tests struct creation, JSON/BSON tag compliance, default values,
// and message type / sender type validation.
package models

import (
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// === Testes de criacao do ChatMessage ===

func TestChatMessage_Creation(t *testing.T) {
	now := time.Now()
	msg := ChatMessage{
		ID:          primitive.NewObjectID(),
		OrderID:     "order_123",
		SenderID:    1001,
		SenderType:  "customer",
		SenderName:  "Joao",
		Message:     "Hello, where is my order?",
		MessageType: "text",
		CreatedAt:   now,
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

func TestChatMessage_NewObjectID(t *testing.T) {
	msg := ChatMessage{
		ID:          primitive.NewObjectID(),
		OrderID:     "order_123",
		SenderID:    1001,
		SenderType:  "customer",
		SenderName:  "Joao",
		Message:     "Hello",
		MessageType: "text",
		CreatedAt:   time.Now(),
	}

	if msg.ID.IsZero() {
		t.Error("NewObjectID should produce a non-zero ID")
	}
}

// === Testes de valores padrao ===

func TestChatMessage_DefaultID_IsZero(t *testing.T) {
	msg := ChatMessage{}

	if !msg.ID.IsZero() {
		t.Error("Zero-value ChatMessage should have a zero ID")
	}
}

func TestChatMessage_DefaultCreatedAt_IsZero(t *testing.T) {
	msg := ChatMessage{}

	if !msg.CreatedAt.IsZero() {
		t.Error("Zero-value ChatMessage should have a zero CreatedAt")
	}
}

func TestChatMessage_DefaultReadAt_IsNil(t *testing.T) {
	msg := ChatMessage{}

	if msg.ReadAt != nil {
		t.Errorf("Default ReadAt: got %v, want nil", msg.ReadAt)
	}
}

func TestChatMessage_DefaultImageURL_IsEmpty(t *testing.T) {
	msg := ChatMessage{}

	if msg.ImageURL != "" {
		t.Errorf("Default ImageURL: got %q, want empty", msg.ImageURL)
	}
}

// === Testes de tipos de mensagem ===

func TestChatMessage_TextMessageType(t *testing.T) {
	msg := ChatMessage{
		MessageType: "text",
	}

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
	if msg.ImageURL != "https://example.com/photo.jpg" {
		t.Errorf("ImageURL: got %q, want %q", msg.ImageURL, "https://example.com/photo.jpg")
	}
}

func TestChatMessage_InvalidMessageType(t *testing.T) {
	// ChatMessage struct does not enforce valid message types at the struct level.
	// The handler (saveMessage) defaults empty MessageType to "text".
	// Invalid values are stored as-is.
	msg := ChatMessage{
		MessageType: "invalid",
	}

	if msg.MessageType != "invalid" {
		t.Errorf("Struct should accept any MessageType string, got %q", msg.MessageType)
	}
}

func TestChatMessage_EmptyMessageType(t *testing.T) {
	msg := ChatMessage{}

	if msg.MessageType != "" {
		t.Errorf("Default MessageType: got %q, want empty", msg.MessageType)
	}
}

// === Testes de tipos de remetente ===

func TestChatMessage_CustomerSenderType(t *testing.T) {
	msg := ChatMessage{
		SenderType: "customer",
	}

	if msg.SenderType != "customer" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "customer")
	}
}

func TestChatMessage_RestaurantSenderType(t *testing.T) {
	msg := ChatMessage{
		SenderType: "restaurant",
	}

	if msg.SenderType != "restaurant" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "restaurant")
	}
}

func TestChatMessage_DeliverySenderType(t *testing.T) {
	msg := ChatMessage{
		SenderType: "delivery",
	}

	if msg.SenderType != "delivery" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "delivery")
	}
}

func TestChatMessage_InvalidSenderType(t *testing.T) {
	// ChatMessage does not validate sender type at the struct level.
	msg := ChatMessage{
		SenderType: "unknown",
	}

	if msg.SenderType != "unknown" {
		t.Errorf("Struct should accept any SenderType string, got %q", msg.SenderType)
	}
}

// === Testes de mensagens vazias ===

func TestChatMessage_EmptyMessage(t *testing.T) {
	msg := ChatMessage{
		Message: "",
	}

	if msg.Message != "" {
		t.Errorf("Empty message: got %q, want empty", msg.Message)
	}
}

func TestChatMessage_EmptyOrderID(t *testing.T) {
	msg := ChatMessage{
		OrderID: "",
	}

	if msg.OrderID != "" {
		t.Errorf("Empty OrderID: got %q, want empty", msg.OrderID)
	}
}

func TestChatMessage_EmptySenderName(t *testing.T) {
	msg := ChatMessage{
		SenderName: "",
	}

	if msg.SenderName != "" {
		t.Errorf("Empty SenderName: got %q, want empty", msg.SenderName)
	}
}

func TestChatMessage_ZeroSenderID(t *testing.T) {
	msg := ChatMessage{}

	if msg.SenderID != 0 {
		t.Errorf("Default SenderID: got %d, want 0", msg.SenderID)
	}
}

// === Testes de campos opcionais ===

func TestChatMessage_ReadAt(t *testing.T) {
	now := time.Now()
	msg := ChatMessage{
		ReadAt: &now,
	}

	if msg.ReadAt == nil {
		t.Fatal("ReadAt should not be nil after assignment")
	}
	if *msg.ReadAt != now {
		t.Errorf("ReadAt: got %v, want %v", *msg.ReadAt, now)
	}
}

func TestChatMessage_ReadAtNil(t *testing.T) {
	msg := ChatMessage{
		ReadAt: nil,
	}

	if msg.ReadAt != nil {
		t.Errorf("ReadAt: got %v, want nil", msg.ReadAt)
	}
}

func TestChatMessage_ImageURLEmpty(t *testing.T) {
	msg := ChatMessage{
		MessageType: "text",
		ImageURL:    "",
	}

	if msg.ImageURL != "" {
		t.Errorf("ImageURL should be empty for text message, got %q", msg.ImageURL)
	}
}

// === Testes de serializacao JSON ===

func TestChatMessage_JSONMarshal(t *testing.T) {
	now := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	oid := primitive.NewObjectID()
	msg := ChatMessage{
		ID:          oid,
		OrderID:     "order_123",
		SenderID:    1001,
		SenderType:  "customer",
		SenderName:  "Joao",
		Message:     "Hello",
		MessageType: "text",
		CreatedAt:   now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["order_id"] != "order_123" {
		t.Errorf("JSON order_id: got %v, want %q", result["order_id"], "order_123")
	}
	if result["sender_id"] != float64(1001) {
		t.Errorf("JSON sender_id: got %v, want %v", result["sender_id"], float64(1001))
	}
	if result["sender_type"] != "customer" {
		t.Errorf("JSON sender_type: got %v, want %q", result["sender_type"], "customer")
	}
	if result["sender_name"] != "Joao" {
		t.Errorf("JSON sender_name: got %v, want %q", result["sender_name"], "Joao")
	}
	if result["message"] != "Hello" {
		t.Errorf("JSON message: got %v, want %q", result["message"], "Hello")
	}
	if result["message_type"] != "text" {
		t.Errorf("JSON message_type: got %v, want %q", result["message_type"], "text")
	}
}

func TestChatMessage_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"order_id": "order_456",
		"sender_id": 2002,
		"sender_type": "restaurant",
		"sender_name": "Maria",
		"message": "Your food is ready",
		"message_type": "text"
	}`

	var msg ChatMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if msg.OrderID != "order_456" {
		t.Errorf("OrderID: got %q, want %q", msg.OrderID, "order_456")
	}
	if msg.SenderID != 2002 {
		t.Errorf("SenderID: got %d, want %d", msg.SenderID, 2002)
	}
	if msg.SenderType != "restaurant" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "restaurant")
	}
	if msg.SenderName != "Maria" {
		t.Errorf("SenderName: got %q, want %q", msg.SenderName, "Maria")
	}
	if msg.Message != "Your food is ready" {
		t.Errorf("Message: got %q, want %q", msg.Message, "Your food is ready")
	}
}

func TestChatMessage_JSONWithReadAt(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	msg := ChatMessage{
		ReadAt: &now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, exists := result["read_at"]; !exists {
		t.Error("read_at should be present when set")
	}
}

func TestChatMessage_JSONReadAtNilOmitted(t *testing.T) {
	msg := ChatMessage{}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// read_at has omitempty so it should not appear when nil
	jsonStr := string(data)
	if contains(jsonStr, "read_at") {
		t.Error("read_at should be omitted when nil (omitempty)")
	}
}

func TestChatMessage_JSONImageURLOmitted(t *testing.T) {
	msg := ChatMessage{
		MessageType: "text",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, "image_url") {
		t.Error("image_url should be omitted when empty (omitempty)")
	}
}

func TestChatMessage_JSONImageURLPresent(t *testing.T) {
	msg := ChatMessage{
		MessageType: "image",
		ImageURL:    "https://example.com/photo.jpg",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["image_url"] != "https://example.com/photo.jpg" {
		t.Errorf("JSON image_url: got %v, want %q", result["image_url"], "https://example.com/photo.jpg")
	}
}

func TestChatMessage_JSONIDAlwaysPresent(t *testing.T) {
	// id field does NOT have omitempty, so zero ObjectID should appear
	msg := ChatMessage{}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// The zero ObjectID marshals as "000000000000000000000000"
	if _, exists := result["id"]; !exists {
		t.Error("id field should always be present even when zero")
	}
}

// === Testes de serializacao BSON ===

func TestChatMessage_BSONMarshal(t *testing.T) {
	msg := ChatMessage{
		OrderID:    "order_123",
		SenderID:   1001,
		SenderType: "customer",
		Message:    "Hello",
	}

	data, err := bson.Marshal(msg)
	if err != nil {
		t.Fatalf("bson.Marshal failed: %v", err)
	}

	var result bson.M
	if err := bson.Unmarshal(data, &result); err != nil {
		t.Fatalf("bson.Unmarshal failed: %v", err)
	}

	if result["order_id"] != "order_123" {
		t.Errorf("BSON order_id: got %v, want %q", result["order_id"], "order_123")
	}
	if result["sender_id"] != int64(1001) {
		t.Errorf("BSON sender_id: got %v, want %d", result["sender_id"], 1001)
	}
}

func TestChatMessage_BSONUnmarshal(t *testing.T) {
	doc := bson.M{
		"order_id":    "order_789",
		"sender_id":   int64(3003),
		"sender_type": "delivery",
		"sender_name": "Pedro",
		"message":     "On my way",
		"message_type": "text",
	}

	data, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("bson.Marshal failed: %v", err)
	}

	var msg ChatMessage
	if err := bson.Unmarshal(data, &msg); err != nil {
		t.Fatalf("bson.Unmarshal failed: %v", err)
	}

	if msg.OrderID != "order_789" {
		t.Errorf("OrderID: got %q, want %q", msg.OrderID, "order_789")
	}
	if msg.SenderID != 3003 {
		t.Errorf("SenderID: got %d, want %d", msg.SenderID, 3003)
	}
	if msg.SenderType != "delivery" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "delivery")
	}
	if msg.SenderName != "Pedro" {
		t.Errorf("SenderName: got %q, want %q", msg.SenderName, "Pedro")
	}
}

// === Testes de mensagem completa ===

func TestChatMessage_FullMessage(t *testing.T) {
	now := time.Now()
	readAt := now.Add(time.Minute)
	oid := primitive.NewObjectID()
	msg := ChatMessage{
		ID:          oid,
		OrderID:     "order_999",
		SenderID:    5005,
		SenderType:  "restaurant",
		SenderName:  "Chef Antonio",
		Message:     "Your pasta is being prepared",
		MessageType: "text",
		ReadAt:      &readAt,
		CreatedAt:   now,
	}

	if msg.ID != oid {
		t.Error("ID mismatch")
	}
	if msg.ReadAt == nil || *msg.ReadAt != readAt {
		t.Error("ReadAt mismatch")
	}
}

func TestChatMessage_ImageMessageWithAllFields(t *testing.T) {
	msg := ChatMessage{
		OrderID:     "order_img",
		SenderID:    100,
		SenderType:  "customer",
		SenderName:  "Client",
		Message:     "Check this photo",
		MessageType: "image",
		ImageURL:    "https://storage.example.com/img123.png",
		CreatedAt:   time.Now(),
	}

	if msg.MessageType != "image" {
		t.Errorf("MessageType: got %q, want %q", msg.MessageType, "image")
	}
	if msg.ImageURL == "" {
		t.Error("ImageURL should not be empty for image message")
	}
}

// === Testes de zero value ===

func TestChatMessage_ZeroValue(t *testing.T) {
	msg := ChatMessage{}

	if msg.ID.IsZero() != true {
		t.Error("Zero ChatMessage ID should be zero")
	}
	if msg.OrderID != "" {
		t.Error("Zero ChatMessage OrderID should be empty")
	}
	if msg.SenderID != 0 {
		t.Error("Zero ChatMessage SenderID should be 0")
	}
	if msg.SenderType != "" {
		t.Error("Zero ChatMessage SenderType should be empty")
	}
	if msg.SenderName != "" {
		t.Error("Zero ChatMessage SenderName should be empty")
	}
	if msg.Message != "" {
		t.Error("Zero ChatMessage Message should be empty")
	}
	if msg.MessageType != "" {
		t.Error("Zero ChatMessage MessageType should be empty")
	}
	if msg.ImageURL != "" {
		t.Error("Zero ChatMessage ImageURL should be empty")
	}
	if msg.ReadAt != nil {
		t.Error("Zero ChatMessage ReadAt should be nil")
	}
	if !msg.CreatedAt.IsZero() {
		t.Error("Zero ChatMessage CreatedAt should be zero")
	}
}

// === helper ===

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
