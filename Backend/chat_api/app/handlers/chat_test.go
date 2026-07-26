// Package handlers - chat_test.go
// Unit tests for the chat handlers.
// Tests HTTP handler validation (BodyParser, userId parsing),
// message construction logic, ClientInfo, and WebSocket message parsing.
// Does NOT require MongoDB or external services - only tests validation
// that fires BEFORE database access, plus pure logic simulations.
// WebSocket room management and DB-dependent tests need integration tests.
package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/carloshomar/vercardapio/chat_api/app/dto"
	"github.com/carloshomar/vercardapio/chat_api/app/models"
	"github.com/gofiber/fiber/v2"
)

// === Testes de SendMessage - validacao de BodyParser ===
// These tests verify validation that fires BEFORE any DB access.

func TestSendMessage_EmptyBody(t *testing.T) {
	app := fiber.New()
	app.Post("/chat/message", SendMessage)

	req, _ := http.NewRequest(http.MethodPost, "/chat/message", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("StatusCode: got %d, want 400", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["error"] != "Invalid request" {
		t.Errorf("error: got %v, want %q", result["error"], "Invalid request")
	}
}

func TestSendMessage_InvalidJSON(t *testing.T) {
	app := fiber.New()
	app.Post("/chat/message", SendMessage)

	payload := `{invalid json}`
	req, _ := http.NewRequest(http.MethodPost, "/chat/message", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(&testStringReader{s: payload})
	req.ContentLength = int64(len(payload))

	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("StatusCode: got %d, want 400", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["error"] != "Invalid request" {
		t.Errorf("error: got %v, want %q", result["error"], "Invalid request")
	}
}

func TestSendMessage_WrongContentType(t *testing.T) {
	app := fiber.New()
	app.Post("/chat/message", SendMessage)

	req, _ := http.NewRequest(http.MethodPost, "/chat/message", nil)
	req.Header.Set("Content-Type", "text/plain")
	req.Body = io.NopCloser(&testStringReader{s: "hello"})
	req.ContentLength = 5

	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}

	// Wrong content type causes BodyParser to fail -> 400
	if resp.StatusCode != 400 {
		t.Errorf("StatusCode: got %d, want 400", resp.StatusCode)
	}
}

// === Testes de MarkAsRead - validacao de userId ===

func TestMarkAsRead_InvalidUserId(t *testing.T) {
	app := fiber.New()
	app.Put("/chat/read/:orderId/:userId", MarkAsRead)

	req, _ := http.NewRequest(http.MethodPut, "/chat/read/order_123/not_a_number", nil)
	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("StatusCode: got %d, want 400", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["error"] != "userId inválido" {
		t.Errorf("error: got %v, want %q", result["error"], "userId inválido")
	}
}

func TestMarkAsRead_FloatUserId(t *testing.T) {
	app := fiber.New()
	app.Put("/chat/read/:orderId/:userId", MarkAsRead)

	req, _ := http.NewRequest(http.MethodPut, "/chat/read/order_123/12.5", nil)
	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("StatusCode: got %d, want 400", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["error"] != "userId inválido" {
		t.Errorf("error: got %v, want %q", result["error"], "userId inválido")
	}
}

func TestMarkAsRead_AlphanumericUserId(t *testing.T) {
	app := fiber.New()
	app.Put("/chat/read/:orderId/:userId", MarkAsRead)

	req, _ := http.NewRequest(http.MethodPut, "/chat/read/order_123/abc123", nil)
	resp, err := app.Test(req, 1000)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("StatusCode: got %d, want 400", resp.StatusCode)
	}
}

// === Testes de MarkAsRead - logica de filtro ===

func TestMarkAsRead_FilterLogic(t *testing.T) {
	// Verify the filter logic: only marks messages from OTHER senders as read.
	// MongoDB filter: { order_id: X, sender_id: { $ne: Y }, read_at: nil }
	tests := []struct {
		name            string
		currentUserID   int64
		messageSenderID int64
		messageReadAt   bool
		shouldBeMarked  bool
	}{
		{
			name:            "message from other user, unread",
			currentUserID:   1001,
			messageSenderID: 2002,
			messageReadAt:   false,
			shouldBeMarked:  true,
		},
		{
			name:            "message from self, unread - skip",
			currentUserID:   1001,
			messageSenderID: 1001,
			messageReadAt:   false,
			shouldBeMarked:  false,
		},
		{
			name:            "message from other user, already read - skip",
			currentUserID:   1001,
			messageSenderID: 2002,
			messageReadAt:   true,
			shouldBeMarked:  false,
		},
		{
			name:            "message from self, already read - skip",
			currentUserID:   1001,
			messageSenderID: 1001,
			messageReadAt:   true,
			shouldBeMarked:  false,
		},
		{
			name:            "message from delivery, unread",
			currentUserID:   1001,
			messageSenderID: 3003,
			messageReadAt:   false,
			shouldBeMarked:  true,
		},
		{
			name:            "message from restaurant, unread",
			currentUserID:   1001,
			messageSenderID: 2002,
			messageReadAt:   false,
			shouldBeMarked:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isOtherSender := tt.messageSenderID != tt.currentUserID
			isUnread := !tt.messageReadAt

			shouldMark := isOtherSender && isUnread
			if shouldMark != tt.shouldBeMarked {
				t.Errorf("shouldMark: got %v, want %v (sender=%d, current=%d, read=%v)",
					shouldMark, tt.shouldBeMarked, tt.messageSenderID, tt.currentUserID, tt.messageReadAt)
			}
		})
	}
}

// === Testes de saveMessage - logica de defaulting ===

func TestSaveMessage_DefaultMessageType(t *testing.T) {
	req := dto.ChatMessageRequest{
		OrderID:    "order_test",
		SenderID:   100,
		SenderType: "customer",
		SenderName: "Test",
		Message:    "Hello",
	}

	// Simulate the saveMessage function's MessageType defaulting logic
	msgType := req.MessageType
	if msgType == "" {
		msgType = "text"
	}

	if msgType != "text" {
		t.Errorf("MessageType: got %q, want %q (should default to text)", msgType, "text")
	}
}

func TestSaveMessage_PreservesImageMessageType(t *testing.T) {
	req := dto.ChatMessageRequest{
		OrderID:     "order_img",
		SenderID:    100,
		SenderType:  "customer",
		Message:     "Look",
		MessageType: "image",
		ImageURL:    "https://example.com/pic.jpg",
	}

	msgType := req.MessageType
	if msgType == "" {
		msgType = "text"
	}

	if msgType != "image" {
		t.Errorf("MessageType: got %q, want %q", msgType, "image")
	}
}

func TestSaveMessage_SenderFieldsCopied(t *testing.T) {
	req := dto.ChatMessageRequest{
		OrderID:    "order_fields",
		SenderID:   5005,
		SenderType: "delivery",
		SenderName: "Pedro",
		Message:    "On my way",
	}

	// Simulate saveMessage field copying
	msg := models.ChatMessage{
		OrderID:     req.OrderID,
		SenderID:    req.SenderID,
		SenderType:  req.SenderType,
		SenderName:  req.SenderName,
		Message:     req.Message,
		MessageType: req.MessageType,
		ImageURL:    req.ImageURL,
	}

	if msg.SenderID != 5005 {
		t.Errorf("SenderID: got %d, want 5005", msg.SenderID)
	}
	if msg.SenderType != "delivery" {
		t.Errorf("SenderType: got %q, want %q", msg.SenderType, "delivery")
	}
	if msg.SenderName != "Pedro" {
		t.Errorf("SenderName: got %q, want %q", msg.SenderName, "Pedro")
	}
}

func TestSaveMessage_EmptyRequestFields(t *testing.T) {
	req := dto.ChatMessageRequest{}

	msgType := req.MessageType
	if msgType == "" {
		msgType = "text"
	}

	if req.OrderID != "" {
		t.Errorf("OrderID should be empty, got %q", req.OrderID)
	}
	if req.SenderID != 0 {
		t.Errorf("SenderID should be 0, got %d", req.SenderID)
	}
	if msgType != "text" {
		t.Errorf("MessageType should default to text, got %q", msgType)
	}
}

// === Testes de construcao de resposta de broadcast ===

func TestBroadcastMessageStructure(t *testing.T) {
	msg := &models.ChatMessage{
		OrderID:     "order_bcast",
		SenderID:    1001,
		SenderType:  "customer",
		SenderName:  "Joao",
		Message:     "Hello",
		MessageType: "text",
		CreatedAt:   time.Now(),
	}

	broadcastMsg := map[string]interface{}{
		"type":    "new_message",
		"payload": msg,
	}

	data, err := json.Marshal(broadcastMsg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["type"] != "new_message" {
		t.Errorf("type: got %v, want %q", result["type"], "new_message")
	}

	payload, ok := result["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload should be a map, got %T", result["payload"])
	}

	if payload["order_id"] != "order_bcast" {
		t.Errorf("payload.order_id: got %v, want %q", payload["order_id"], "order_bcast")
	}
	if payload["message"] != "Hello" {
		t.Errorf("payload.message: got %v, want %q", payload["message"], "Hello")
	}
	if payload["sender_type"] != "customer" {
		t.Errorf("payload.sender_type: got %v, want %q", payload["sender_type"], "customer")
	}
}

func TestBroadcastMessageStructureForTyping(t *testing.T) {
	userID := int64(1001)
	userType := "customer"

	broadcastMsg := map[string]interface{}{
		"type": "typing",
		"payload": map[string]interface{}{
			"sender_id":   userID,
			"sender_type": userType,
		},
	}

	data, err := json.Marshal(broadcastMsg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["type"] != "typing" {
		t.Errorf("type: got %v, want %q", result["type"], "typing")
	}

	payload, ok := result["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload should be a map, got %T", result["payload"])
	}

	if payload["sender_id"] != float64(1001) {
		t.Errorf("payload.sender_id: got %v, want %v", payload["sender_id"], float64(1001))
	}
	if payload["sender_type"] != "customer" {
		t.Errorf("payload.sender_type: got %v, want %q", payload["sender_type"], "customer")
	}
}

func TestBroadcastMessageSentResponse(t *testing.T) {
	// Simulate the "message_sent" response sent back to sender via WebSocket
	msg := &models.ChatMessage{
		OrderID:     "order_ws_reply",
		SenderID:    2002,
		SenderType:  "restaurant",
		SenderName:  "Chef",
		Message:     "Order confirmed",
		MessageType: "text",
		CreatedAt:   time.Now(),
	}

	replyMsg := map[string]interface{}{
		"type":    "message_sent",
		"payload": msg,
	}

	data, err := json.Marshal(replyMsg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["type"] != "message_sent" {
		t.Errorf("type: got %v, want %q", result["type"], "message_sent")
	}
}

// === Testes de parsing de mensagem WebSocket ===

func TestWebSocketMessageParsing_MessageType(t *testing.T) {
	wsJSON := `{
		"type": "message",
		"payload": {
			"order_id": "order_ws",
			"sender_name": "Client",
			"message": "Hello via WS",
			"message_type": "text"
		}
	}`

	var wsMsg dto.ChatWebSocketMessage
	if err := json.Unmarshal([]byte(wsJSON), &wsMsg); err != nil {
		t.Fatalf("Step 1 unmarshal failed: %v", err)
	}

	if wsMsg.Type != "message" {
		t.Errorf("Type: got %q, want %q", wsMsg.Type, "message")
	}

	// Marshal payload and unmarshal into ChatMessageRequest (same as handler)
	payloadBytes, err := json.Marshal(wsMsg.Payload)
	if err != nil {
		t.Fatalf("Marshal payload failed: %v", err)
	}

	var msgReq dto.ChatMessageRequest
	if err := json.Unmarshal(payloadBytes, &msgReq); err != nil {
		t.Fatalf("Step 2 unmarshal failed: %v", err)
	}

	if msgReq.OrderID != "order_ws" {
		t.Errorf("OrderID: got %q, want %q", msgReq.OrderID, "order_ws")
	}
	if msgReq.SenderName != "Client" {
		t.Errorf("SenderName: got %q, want %q", msgReq.SenderName, "Client")
	}
	if msgReq.Message != "Hello via WS" {
		t.Errorf("Message: got %q, want %q", msgReq.Message, "Hello via WS")
	}
	if msgReq.MessageType != "text" {
		t.Errorf("MessageType: got %q, want %q", msgReq.MessageType, "text")
	}
}

func TestWebSocketMessageParsing_TypingType(t *testing.T) {
	wsJSON := `{
		"type": "typing",
		"payload": {
			"sender_id": 1001,
			"sender_type": "customer"
		}
	}`

	var wsMsg dto.ChatWebSocketMessage
	if err := json.Unmarshal([]byte(wsJSON), &wsMsg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if wsMsg.Type != "typing" {
		t.Errorf("Type: got %q, want %q", wsMsg.Type, "typing")
	}

	payloadMap, ok := wsMsg.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("Payload should be map[string]interface{}, got %T", wsMsg.Payload)
	}

	if payloadMap["sender_id"] != float64(1001) {
		t.Errorf("sender_id: got %v, want %v", payloadMap["sender_id"], float64(1001))
	}
}

func TestWebSocketMessageParsing_WithImageURL(t *testing.T) {
	wsJSON := `{
		"type": "message",
		"payload": {
			"order_id": "order_img_ws",
			"sender_name": "Client",
			"message": "Check this",
			"message_type": "image",
			"image_url": "https://example.com/ws_photo.jpg"
		}
	}`

	var wsMsg dto.ChatWebSocketMessage
	if err := json.Unmarshal([]byte(wsJSON), &wsMsg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	payloadBytes, _ := json.Marshal(wsMsg.Payload)
	var msgReq dto.ChatMessageRequest
	if err := json.Unmarshal(payloadBytes, &msgReq); err != nil {
		t.Fatalf("Step 2 unmarshal failed: %v", err)
	}

	if msgReq.MessageType != "image" {
		t.Errorf("MessageType: got %q, want %q", msgReq.MessageType, "image")
	}
	if msgReq.ImageURL != "https://example.com/ws_photo.jpg" {
		t.Errorf("ImageURL: got %q, want %q", msgReq.ImageURL, "https://example.com/ws_photo.jpg")
	}
}

func TestWebSocketMessageParsing_SenderIDOverride(t *testing.T) {
	// Handler overrides SenderID and SenderType from WebSocket params
	wsJSON := `{
		"type": "message",
		"payload": {
			"order_id": "order_override",
			"sender_id": 999,
			"sender_type": "unknown",
			"sender_name": "Client",
			"message": "Override test"
		}
	}`

	var wsMsg dto.ChatWebSocketMessage
	if err := json.Unmarshal([]byte(wsJSON), &wsMsg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	payloadBytes, _ := json.Marshal(wsMsg.Payload)
	var msgReq dto.ChatMessageRequest
	if err := json.Unmarshal(payloadBytes, &msgReq); err != nil {
		t.Fatalf("Step 2 unmarshal failed: %v", err)
	}

	// Handler overrides these from WebSocket params (mimicking HandleChatWebSocket)
	wsUserID := int64(5001)
	wsUserType := "customer"
	msgReq.SenderID = wsUserID
	msgReq.SenderType = wsUserType

	if msgReq.SenderID != 5001 {
		t.Errorf("SenderID after override: got %d, want 5001", msgReq.SenderID)
	}
	if msgReq.SenderType != "customer" {
		t.Errorf("SenderType after override: got %q, want %q", msgReq.SenderType, "customer")
	}
}

func TestWebSocketMessageParsing_InvalidJSON(t *testing.T) {
	invalidJSON := `{invalid`

	var wsMsg dto.ChatWebSocketMessage
	if err := json.Unmarshal([]byte(invalidJSON), &wsMsg); err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestWebSocketMessageParsing_EmptyPayload(t *testing.T) {
	wsJSON := `{
		"type": "message",
		"payload": {}
	}`

	var wsMsg dto.ChatWebSocketMessage
	if err := json.Unmarshal([]byte(wsJSON), &wsMsg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	payloadBytes, _ := json.Marshal(wsMsg.Payload)
	var msgReq dto.ChatMessageRequest
	if err := json.Unmarshal(payloadBytes, &msgReq); err != nil {
		t.Fatalf("Step 2 unmarshal failed: %v", err)
	}

	if msgReq.OrderID != "" {
		t.Errorf("OrderID should be empty, got %q", msgReq.OrderID)
	}
	if msgReq.SenderID != 0 {
		t.Errorf("SenderID should be 0, got %d", msgReq.SenderID)
	}
	if msgReq.Message != "" {
		t.Errorf("Message should be empty, got %q", msgReq.Message)
	}
}

func TestWebSocketMessageParsing_FullCycle(t *testing.T) {
	// Test the complete WebSocket message handling cycle:
	// 1. Receive raw JSON
	// 2. Parse into ChatWebSocketMessage
	// 3. Extract payload
	// 4. Override sender fields from WS params
	// 5. Construct ChatMessage for DB storage

	wsJSON := `{
		"type": "message",
		"payload": {
			"order_id": "order_full",
			"sender_id": 999,
			"sender_type": "unknown",
			"sender_name": "WS User",
			"message": "Full cycle test",
			"message_type": "text"
		}
	}`

	// Step 1-2: Parse WebSocket frame
	var wsMsg dto.ChatWebSocketMessage
	if err := json.Unmarshal([]byte(wsJSON), &wsMsg); err != nil {
		t.Fatalf("Step 1 failed: %v", err)
	}

	// Step 3: Extract payload (same as handler)
	payloadBytes, err := json.Marshal(wsMsg.Payload)
	if err != nil {
		t.Fatalf("Marshal payload failed: %v", err)
	}

	var msgReq dto.ChatMessageRequest
	if err := json.Unmarshal(payloadBytes, &msgReq); err != nil {
		t.Fatalf("Step 2 failed: %v", err)
	}

	// Step 4: Override sender from WS params (same as handler)
	msgReq.SenderID = 1001
	msgReq.SenderType = "customer"

	// Step 5: Construct ChatMessage (same as saveMessage)
	msg := models.ChatMessage{
		OrderID:     msgReq.OrderID,
		SenderID:    msgReq.SenderID,
		SenderType:  msgReq.SenderType,
		SenderName:  msgReq.SenderName,
		Message:     msgReq.Message,
		MessageType: msgReq.MessageType,
		ImageURL:    msgReq.ImageURL,
	}
	if msg.MessageType == "" {
		msg.MessageType = "text"
	}

	// Verify the constructed message
	if msg.OrderID != "order_full" {
		t.Errorf("OrderID: got %q, want %q", msg.OrderID, "order_full")
	}
	if msg.SenderID != 1001 {
		t.Errorf("SenderID should be overridden: got %d, want 1001", msg.SenderID)
	}
	if msg.SenderType != "customer" {
		t.Errorf("SenderType should be overridden: got %q, want %q", msg.SenderType, "customer")
	}
	if msg.SenderName != "WS User" {
		t.Errorf("SenderName: got %q, want %q", msg.SenderName, "WS User")
	}
	if msg.Message != "Full cycle test" {
		t.Errorf("Message: got %q, want %q", msg.Message, "Full cycle test")
	}
	if msg.MessageType != "text" {
		t.Errorf("MessageType: got %q, want %q", msg.MessageType, "text")
	}
}

// === Testes de ClientInfo ===

func TestClientInfo_Creation(t *testing.T) {
	info := ClientInfo{
		UserID:   1001,
		UserType: "customer",
	}

	if info.UserID != 1001 {
		t.Errorf("UserID: got %d, want 1001", info.UserID)
	}
	if info.UserType != "customer" {
		t.Errorf("UserType: got %q, want %q", info.UserType, "customer")
	}
}

func TestClientInfo_RestaurantType(t *testing.T) {
	info := ClientInfo{
		UserID:   2002,
		UserType: "restaurant",
	}

	if info.UserType != "restaurant" {
		t.Errorf("UserType: got %q, want %q", info.UserType, "restaurant")
	}
}

func TestClientInfo_DeliveryType(t *testing.T) {
	info := ClientInfo{
		UserID:   3003,
		UserType: "delivery",
	}

	if info.UserType != "delivery" {
		t.Errorf("UserType: got %q, want %q", info.UserType, "delivery")
	}
}

func TestClientInfo_ZeroValues(t *testing.T) {
	info := ClientInfo{}

	if info.UserID != 0 {
		t.Errorf("Default UserID: got %d, want 0", info.UserID)
	}
	if info.UserType != "" {
		t.Errorf("Default UserType: got %q, want empty", info.UserType)
	}
}

// === Testes de UserID parsing (validacao do HandleChatWebSocket) ===

func TestUserIDParsing(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  int64
		expectErr bool
	}{
		{"valid positive", "1001", 1001, false},
		{"valid zero", "0", 0, false},
		{"valid negative", "-1", -1, false},
		{"valid large", "9999999999", 9999999999, false},
		{"invalid string", "abc", 0, true},
		{"invalid float", "12.5", 0, true},
		{"empty string", "", 0, true},
		{"invalid hex", "0x1A", 0, true},
		{"with spaces", " 1001 ", 0, true},
		{"overflow", "999999999999999999999", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := strconv.ParseInt(tt.input, 10, 64)
			if (err != nil) != tt.expectErr {
				t.Errorf("ParseInt(%q): err=%v, wantErr=%v", tt.input, err, tt.expectErr)
			}
			if !tt.expectErr && result != tt.expected {
				t.Errorf("ParseInt(%q): got %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// === Testes de message response serialization ===

func TestMessageResponseSerialization(t *testing.T) {
	msg := &models.ChatMessage{
		OrderID:     "order_ser",
		SenderID:    100,
		SenderType:  "customer",
		SenderName:  "Client",
		Message:     "Hello",
		MessageType: "text",
		CreatedAt:   time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expectedFields := []string{
		"id", "order_id", "sender_id", "sender_type",
		"sender_name", "message", "message_type", "created_at",
	}
	for _, field := range expectedFields {
		if _, exists := result[field]; !exists {
			t.Errorf("Response missing field: %q", field)
		}
	}

	if result["order_id"] != "order_ser" {
		t.Errorf("order_id: got %v, want %q", result["order_id"], "order_ser")
	}
	if result["message"] != "Hello" {
		t.Errorf("message: got %v, want %q", result["message"], "Hello")
	}
}

func TestMessageResponseWithReadAt(t *testing.T) {
	now := time.Now()

	msg := &models.ChatMessage{
		OrderID:     "order_read",
		SenderID:    100,
		SenderType:  "customer",
		Message:     "Read test",
		MessageType: "text",
		ReadAt:      &now,
		CreatedAt:   time.Now(),
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

func TestMessageResponseWithoutReadAt(t *testing.T) {
	msg := &models.ChatMessage{
		OrderID:     "order_no_read",
		SenderID:    100,
		SenderType:  "customer",
		Message:     "No read",
		MessageType: "text",
		ReadAt:      nil,
		CreatedAt:   time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// read_at and image_url have omitempty so should not appear
	jsonStr := string(data)
	for _, key := range []string{"\"read_at\"", "\"image_url\""} {
		for i := 0; i <= len(jsonStr)-len(key); i++ {
			if i+len(key) <= len(jsonStr) && jsonStr[i:i+len(key)] == key {
				t.Errorf("Field %s should be omitted when nil/empty", key)
			}
		}
	}
}

// === Testes de JSON response format ===

func TestErrorResponseFormat(t *testing.T) {
	errResponse := fiber.Map{"error": "orderId é obrigatório"}

	data, err := json.Marshal(errResponse)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["error"] != "orderId é obrigatório" {
		t.Errorf("error: got %v, want %q", result["error"], "orderId é obrigatório")
	}
}

func TestSuccessResponseFormat(t *testing.T) {
	successResponse := fiber.Map{"message": "Mensagens marcadas como lidas"}

	data, err := json.Marshal(successResponse)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if result["message"] != "Mensagens marcadas como lidas" {
		t.Errorf("message: got %v, want %q", result["message"], "Mensagens marcadas como lidas")
	}
}

// === helper ===

// testStringReader is a simple io.Reader implementation for test request bodies.
type testStringReader struct {
	s   string
	pos int
}

func (r *testStringReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.pos:])
	r.pos += n
	return n, nil
}
