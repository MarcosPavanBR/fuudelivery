// Package dto - chat_test.go
// Unit tests for chat DTOs.
// Tests ChatMessageRequest, ChatMessageResponse, and ChatWebSocketMessage
// serialization, field mapping, and edge cases.
package dto

import (
	"encoding/json"
	"testing"
	"time"
)

// === Testes de ChatMessageRequest ===

func TestChatMessageRequest_Creation(t *testing.T) {
	req := ChatMessageRequest{
		OrderID:     "order_123",
		SenderID:    1001,
		SenderType:  "customer",
		SenderName:  "Joao",
		Message:     "Hello, where is my order?",
		MessageType: "text",
		ImageURL:    "",
	}

	if req.OrderID != "order_123" {
		t.Errorf("OrderID: got %q, want %q", req.OrderID, "order_123")
	}
	if req.SenderID != 1001 {
		t.Errorf("SenderID: got %d, want %d", req.SenderID, 1001)
	}
	if req.SenderType != "customer" {
		t.Errorf("SenderType: got %q, want %q", req.SenderType, "customer")
	}
	if req.SenderName != "Joao" {
		t.Errorf("SenderName: got %q, want %q", req.SenderName, "Joao")
	}
	if req.Message != "Hello, where is my order?" {
		t.Errorf("Message: got %q, want %q", req.Message, "Hello, where is my order?")
	}
	if req.MessageType != "text" {
		t.Errorf("MessageType: got %q, want %q", req.MessageType, "text")
	}
}

func TestChatMessageRequest_WithImageURL(t *testing.T) {
	req := ChatMessageRequest{
		OrderID:     "order_456",
		SenderID:    2002,
		SenderType:  "customer",
		SenderName:  "Maria",
		Message:     "Look at this",
		MessageType: "image",
		ImageURL:    "https://example.com/photo.jpg",
	}

	if req.ImageURL != "https://example.com/photo.jpg" {
		t.Errorf("ImageURL: got %q, want %q", req.ImageURL, "https://example.com/photo.jpg")
	}
}

func TestChatMessageRequest_ZeroValue(t *testing.T) {
	req := ChatMessageRequest{}

	if req.OrderID != "" {
		t.Errorf("Default OrderID: got %q, want empty", req.OrderID)
	}
	if req.SenderID != 0 {
		t.Errorf("Default SenderID: got %d, want 0", req.SenderID)
	}
	if req.SenderType != "" {
		t.Errorf("Default SenderType: got %q, want empty", req.SenderType)
	}
	if req.SenderName != "" {
		t.Errorf("Default SenderName: got %q, want empty", req.SenderName)
	}
	if req.Message != "" {
		t.Errorf("Default Message: got %q, want empty", req.Message)
	}
	if req.MessageType != "" {
		t.Errorf("Default MessageType: got %q, want empty", req.MessageType)
	}
	if req.ImageURL != "" {
		t.Errorf("Default ImageURL: got %q, want empty", req.ImageURL)
	}
}

func TestChatMessageRequest_EmptyFields(t *testing.T) {
	tests := []struct {
		name string
		req  ChatMessageRequest
	}{
		{"empty OrderID", ChatMessageRequest{OrderID: "", SenderID: 1, Message: "hi"}},
		{"empty Message", ChatMessageRequest{OrderID: "o1", SenderID: 1, Message: ""}},
		{"empty SenderType", ChatMessageRequest{OrderID: "o1", SenderID: 1, SenderType: "", Message: "hi"}},
		{"empty SenderName", ChatMessageRequest{OrderID: "o1", SenderID: 1, SenderName: "", Message: "hi"}},
		{"empty MessageType", ChatMessageRequest{OrderID: "o1", SenderID: 1, Message: "hi", MessageType: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Struct should accept empty fields (handler validates them)
			if tt.req.OrderID == "" && tt.req.Message == "" {
				// Both empty is fine at struct level
			}
		})
	}
}

func TestChatMessageRequest_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"order_id": "order_789",
		"sender_id": 3003,
		"sender_type": "restaurant",
		"sender_name": "Chef",
		"message": "Order is ready",
		"message_type": "text"
	}`

	var req ChatMessageRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if req.OrderID != "order_789" {
		t.Errorf("OrderID: got %q, want %q", req.OrderID, "order_789")
	}
	if req.SenderID != 3003 {
		t.Errorf("SenderID: got %d, want %d", req.SenderID, 3003)
	}
	if req.SenderType != "restaurant" {
		t.Errorf("SenderType: got %q, want %q", req.SenderType, "restaurant")
	}
	if req.SenderName != "Chef" {
		t.Errorf("SenderName: got %q, want %q", req.SenderName, "Chef")
	}
	if req.Message != "Order is ready" {
		t.Errorf("Message: got %q, want %q", req.Message, "Order is ready")
	}
	if req.MessageType != "text" {
		t.Errorf("MessageType: got %q, want %q", req.MessageType, "text")
	}
}

func TestChatMessageRequest_JSONUnmarshalWithImage(t *testing.T) {
	jsonStr := `{
		"order_id": "order_img",
		"sender_id": 500,
		"sender_type": "customer",
		"sender_name": "Client",
		"message": "Check this photo",
		"message_type": "image",
		"image_url": "https://example.com/pic.jpg"
	}`

	var req ChatMessageRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if req.MessageType != "image" {
		t.Errorf("MessageType: got %q, want %q", req.MessageType, "image")
	}
	if req.ImageURL != "https://example.com/pic.jpg" {
		t.Errorf("ImageURL: got %q, want %q", req.ImageURL, "https://example.com/pic.jpg")
	}
}

func TestChatMessageRequest_JSONMarshal(t *testing.T) {
	req := ChatMessageRequest{
		OrderID:    "order_m",
		SenderID:   100,
		SenderType: "delivery",
		SenderName: "Driver",
		Message:    "Almost there",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["order_id"] != "order_m" {
		t.Errorf("JSON order_id: got %v, want %q", result["order_id"], "order_m")
	}
	if result["sender_id"] != float64(100) {
		t.Errorf("JSON sender_id: got %v, want %v", result["sender_id"], float64(100))
	}
	if result["sender_type"] != "delivery" {
		t.Errorf("JSON sender_type: got %v, want %q", result["sender_type"], "delivery")
	}
}

func TestChatMessageRequest_JSONMissingFields(t *testing.T) {
	// Missing all fields - should unmarshal to zero values
	jsonStr := `{}`
	var req ChatMessageRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if req.OrderID != "" {
		t.Errorf("OrderID should be empty, got %q", req.OrderID)
	}
	if req.SenderID != 0 {
		t.Errorf("SenderID should be 0, got %d", req.SenderID)
	}
	if req.Message != "" {
		t.Errorf("Message should be empty, got %q", req.Message)
	}
}

func TestChatMessageRequest_InvalidJSON(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
	}{
		{"invalid json", `{invalid`},
		{"trailing comma", `{"order_id": "o1",}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ChatMessageRequest
			if err := json.Unmarshal([]byte(tt.jsonStr), &req); err == nil {
				t.Error("Expected error for invalid JSON")
			}
		})
	}
}

func TestChatMessageRequest_SenderIDOverride(t *testing.T) {
	// Handler overrides SenderID and SenderType from WebSocket params
	req := ChatMessageRequest{
		OrderID:    "order_ws",
		SenderID:   999, // will be overridden by handler
		SenderType: "unknown", // will be overridden by handler
		Message:    "Hello",
	}

	// Simulate handler override
	req.SenderID = 5001
	req.SenderType = "customer"

	if req.SenderID != 5001 {
		t.Errorf("SenderID after override: got %d, want 5001", req.SenderID)
	}
	if req.SenderType != "customer" {
		t.Errorf("SenderType after override: got %q, want %q", req.SenderType, "customer")
	}
}

// === Testes de ChatMessageResponse ===

func TestChatMessageResponse_Creation(t *testing.T) {
	now := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	resp := ChatMessageResponse{
		ID:          "666e1b2c3d4e5f6a7b8c9d0e",
		OrderID:     "order_123",
		SenderID:    1001,
		SenderType:  "customer",
		SenderName:  "Joao",
		Message:     "Hello",
		MessageType: "text",
		ReadAt:      nil,
		CreatedAt:   now.Format(time.RFC3339),
	}

	if resp.ID != "666e1b2c3d4e5f6a7b8c9d0e" {
		t.Errorf("ID: got %q, want %q", resp.ID, "666e1b2c3d4e5f6a7b8c9d0e")
	}
	if resp.ReadAt != nil {
		t.Errorf("ReadAt should be nil, got %v", resp.ReadAt)
	}
}

func TestChatMessageResponse_WithReadAt(t *testing.T) {
	now := time.Now()
	readAtStr := now.Format(time.RFC3339)
	resp := ChatMessageResponse{
		ReadAt: &readAtStr,
	}

	if resp.ReadAt == nil {
		t.Fatal("ReadAt should not be nil")
	}
	if *resp.ReadAt != readAtStr {
		t.Errorf("ReadAt: got %q, want %q", *resp.ReadAt, readAtStr)
	}
}

func TestChatMessageResponse_JSONMarshal(t *testing.T) {
	now := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	resp := ChatMessageResponse{
		ID:          "666e1b2c3d4e5f6a7b8c9d0e",
		OrderID:     "order_json",
		SenderID:    2002,
		SenderType:  "restaurant",
		SenderName:  "Chef",
		Message:     "Food is ready",
		MessageType: "text",
		ReadAt:      nil,
		CreatedAt:   now.Format(time.RFC3339),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["id"] != "666e1b2c3d4e5f6a7b8c9d0e" {
		t.Errorf("JSON id: got %v, want %q", result["id"], "666e1b2c3d4e5f6a7b8c9d0e")
	}
	if result["order_id"] != "order_json" {
		t.Errorf("JSON order_id: got %v, want %q", result["order_id"], "order_json")
	}
	if result["sender_id"] != float64(2002) {
		t.Errorf("JSON sender_id: got %v, want %v", result["sender_id"], float64(2002))
	}
}

func TestChatMessageResponse_ReadAtOmittedWhenNil(t *testing.T) {
	resp := ChatMessageResponse{
		ID:      "666e1b2c3d4e5f6a7b8c9d0e",
		OrderID: "o1",
		ReadAt:  nil,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// read_at has omitempty so it should not appear when nil
	jsonStr := string(data)
	for _, key := range []string{"read_at", "image_url"} {
		if containsJSONKey(jsonStr, key) {
			t.Errorf("%q should be omitted when zero (omitempty)", key)
		}
	}
}

func TestChatMessageResponse_ReadAtPresentWhenSet(t *testing.T) {
	readAtStr := "2024-06-15T12:00:00Z"
	resp := ChatMessageResponse{
		ReadAt: &readAtStr,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(data)
	if !containsJSONKey(jsonStr, "read_at") {
		t.Error("read_at should be present when set")
	}
}

func TestChatMessageResponse_ImageURLOmitted(t *testing.T) {
	resp := ChatMessageResponse{
		ImageURL: "",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(data)
	if containsJSONKey(jsonStr, "image_url") {
		t.Error("image_url should be omitted when empty")
	}
}

func TestChatMessageResponse_ImageURLPresent(t *testing.T) {
	resp := ChatMessageResponse{
		ImageURL: "https://example.com/img.png",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["image_url"] != "https://example.com/img.png" {
		t.Errorf("image_url: got %v, want %q", result["image_url"], "https://example.com/img.png")
	}
}

func TestChatMessageResponse_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"id": "666e1b2c3d4e5f6a7b8c9d0e",
		"order_id": "order_unmarshal",
		"sender_id": 3003,
		"sender_type": "delivery",
		"sender_name": "Driver",
		"message": "Arriving soon",
		"message_type": "text",
		"created_at": "2024-06-15T10:30:00Z"
	}`

	var resp ChatMessageResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if resp.ID != "666e1b2c3d4e5f6a7b8c9d0e" {
		t.Errorf("ID: got %q, want %q", resp.ID, "666e1b2c3d4e5f6a7b8c9d0e")
	}
	if resp.OrderID != "order_unmarshal" {
		t.Errorf("OrderID: got %q, want %q", resp.OrderID, "order_unmarshal")
	}
	if resp.SenderID != 3003 {
		t.Errorf("SenderID: got %d, want %d", resp.SenderID, 3003)
	}
	if resp.ReadAt != nil {
		t.Errorf("ReadAt should be nil, got %v", resp.ReadAt)
	}
}

func TestChatMessageResponse_ZeroValue(t *testing.T) {
	resp := ChatMessageResponse{}

	if resp.ID != "" {
		t.Errorf("Default ID: got %q, want empty", resp.ID)
	}
	if resp.ReadAt != nil {
		t.Error("Default ReadAt should be nil")
	}
	if resp.CreatedAt != "" {
		t.Errorf("Default CreatedAt: got %q, want empty", resp.CreatedAt)
	}
}

func TestChatMessageResponse_FullRoundTrip(t *testing.T) {
	now := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	readAtStr := now.Add(time.Minute).Format(time.RFC3339)
	original := ChatMessageResponse{
		ID:          "666e1b2c3d4e5f6a7b8c9d0e",
		OrderID:     "order_rt",
		SenderID:    4004,
		SenderType:  "customer",
		SenderName:  "Ana",
		Message:     "Thanks!",
		MessageType: "text",
		ImageURL:    "",
		ReadAt:      &readAtStr,
		CreatedAt:   now.Format(time.RFC3339),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ChatMessageResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.OrderID != original.OrderID {
		t.Errorf("OrderID: got %q, want %q", decoded.OrderID, original.OrderID)
	}
	if decoded.SenderID != original.SenderID {
		t.Errorf("SenderID: got %d, want %d", decoded.SenderID, original.SenderID)
	}
	if decoded.ReadAt == nil || *decoded.ReadAt != *original.ReadAt {
		t.Errorf("ReadAt: got %v, want %v", decoded.ReadAt, original.ReadAt)
	}
}

// === Testes de ChatWebSocketMessage ===

func TestChatWebSocketMessage_Creation(t *testing.T) {
	msg := ChatWebSocketMessage{
		Type:    "message",
		Payload: map[string]interface{}{"text": "hello"},
	}

	if msg.Type != "message" {
		t.Errorf("Type: got %q, want %q", msg.Type, "message")
	}
	if msg.Payload == nil {
		t.Error("Payload should not be nil")
	}
}

func TestChatWebSocketMessage_MessageType(t *testing.T) {
	msg := ChatWebSocketMessage{
		Type:    "message",
		Payload: ChatMessageRequest{OrderID: "o1", Message: "hi"},
	}

	if msg.Type != "message" {
		t.Errorf("Type: got %q, want %q", msg.Type, "message")
	}
}

func TestChatWebSocketMessage_TypingType(t *testing.T) {
	msg := ChatWebSocketMessage{
		Type:    "typing",
		Payload: map[string]interface{}{
			"sender_id":   float64(100),
			"sender_type": "customer",
		},
	}

	if msg.Type != "typing" {
		t.Errorf("Type: got %q, want %q", msg.Type, "typing")
	}
}

func TestChatWebSocketMessage_JSONMarshal(t *testing.T) {
	msg := ChatWebSocketMessage{
		Type: "message",
		Payload: map[string]interface{}{
			"order_id": "order_ws",
			"message":  "Hello via WebSocket",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["type"] != "message" {
		t.Errorf("JSON type: got %v, want %q", result["type"], "message")
	}

	payload, ok := result["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload should be a map, got %T", result["payload"])
	}
	if payload["order_id"] != "order_ws" {
		t.Errorf("payload.order_id: got %v, want %q", payload["order_id"], "order_ws")
	}
}

func TestChatWebSocketMessage_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"type": "message",
		"payload": {
			"order_id": "order_ws2",
			"sender_id": 500,
			"sender_type": "restaurant",
			"sender_name": "Chef",
			"message": "Order confirmed",
			"message_type": "text"
		}
	}`

	var msg ChatWebSocketMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if msg.Type != "message" {
		t.Errorf("Type: got %q, want %q", msg.Type, "message")
	}

	// Payload is interface{}, so it unmarshals as map[string]interface{}
	payloadMap, ok := msg.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("Payload should be map[string]interface{}, got %T", msg.Payload)
	}
	if payloadMap["order_id"] != "order_ws2" {
		t.Errorf("payload.order_id: got %v, want %q", payloadMap["order_id"], "order_ws2")
	}
	if payloadMap["message"] != "Order confirmed" {
		t.Errorf("payload.message: got %v, want %q", payloadMap["message"], "Order confirmed")
	}
}

func TestChatWebSocketMessage_PayloadAsChatMessageRequest(t *testing.T) {
	// Simulate the handler's two-step unmarshal:
	// 1. Unmarshal WebSocket frame into ChatWebSocketMessage
	// 2. Marshal payload, then unmarshal into ChatMessageRequest
	wsJSON := `{
		"type": "message",
		"payload": {
			"order_id": "order_step",
			"sender_id": 1001,
			"sender_type": "customer",
			"sender_name": "Client",
			"message": "Test",
			"message_type": "text"
		}
	}`

	var wsMsg ChatWebSocketMessage
	if err := json.Unmarshal([]byte(wsJSON), &wsMsg); err != nil {
		t.Fatalf("Step 1 unmarshal failed: %v", err)
	}

	// Handler does: payloadBytes, _ := json.Marshal(wsMsg.Payload)
	// then: json.Unmarshal(payloadBytes, &msgReq)
	payloadBytes, err := json.Marshal(wsMsg.Payload)
	if err != nil {
		t.Fatalf("Marshal payload failed: %v", err)
	}

	var msgReq ChatMessageRequest
	if err := json.Unmarshal(payloadBytes, &msgReq); err != nil {
		t.Fatalf("Step 2 unmarshal failed: %v", err)
	}

	if msgReq.OrderID != "order_step" {
		t.Errorf("OrderID: got %q, want %q", msgReq.OrderID, "order_step")
	}
	if msgReq.SenderID != 1001 {
		t.Errorf("SenderID: got %d, want %d", msgReq.SenderID, 1001)
	}
	if msgReq.SenderType != "customer" {
		t.Errorf("SenderType: got %q, want %q", msgReq.SenderType, "customer")
	}
	if msgReq.Message != "Test" {
		t.Errorf("Message: got %q, want %q", msgReq.Message, "Test")
	}
}

func TestChatWebSocketMessage_EmptyType(t *testing.T) {
	msg := ChatWebSocketMessage{}

	if msg.Type != "" {
		t.Errorf("Default Type: got %q, want empty", msg.Type)
	}
	if msg.Payload != nil {
		t.Errorf("Default Payload: got %v, want nil", msg.Payload)
	}
}

func TestChatWebSocketMessage_UnknownType(t *testing.T) {
	// Handler only handles "message" and "typing"; other types are silently ignored
	msg := ChatWebSocketMessage{
		Type:    "unknown_type",
		Payload: nil,
	}

	if msg.Type != "unknown_type" {
		t.Errorf("Type: got %q, want %q", msg.Type, "unknown_type")
	}
}

func TestChatWebSocketMessage_TypingPayloadStructure(t *testing.T) {
	wsJSON := `{
		"type": "typing",
		"payload": {
			"sender_id": 100,
			"sender_type": "customer"
		}
	}`

	var msg ChatWebSocketMessage
	if err := json.Unmarshal([]byte(wsJSON), &msg); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	payloadMap, ok := msg.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("Payload should be map[string]interface{}, got %T", msg.Payload)
	}

	if payloadMap["sender_id"] != float64(100) {
		t.Errorf("payload.sender_id: got %v, want %v", payloadMap["sender_id"], float64(100))
	}
	if payloadMap["sender_type"] != "customer" {
		t.Errorf("payload.sender_type: got %v, want %q", payloadMap["sender_type"], "customer")
	}
}

// === Testes de payload nil ===

func TestChatWebSocketMessage_NilPayloadJSON(t *testing.T) {
	msg := ChatWebSocketMessage{
		Type:    "typing",
		Payload: nil,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// payload is null in JSON
	if result["payload"] != nil {
		t.Errorf("payload should be null, got %v", result["payload"])
	}
}

func TestChatWebSocketMessage_InvalidJSON(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
	}{
		{"invalid json", `{broken`},
		{"empty string", ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg ChatWebSocketMessage
			if err := json.Unmarshal([]byte(tt.jsonStr), &msg); err == nil {
				t.Error("Expected error for invalid JSON")
			}
		})
	}
}

// === helper ===

func containsJSONKey(jsonStr, key string) bool {
	// Simple check: does the JSON string contain the key as a JSON field?
	search := `"` + key + `":`
	for i := 0; i <= len(jsonStr)-len(search); i++ {
		if jsonStr[i:i+len(search)] == search {
			return true
		}
	}
	return false
}
