package dto

type ChatMessageRequest struct {
	OrderID    string `json:"order_id"`
	SenderID   int64  `json:"sender_id"`
	SenderType string `json:"sender_type"`
	SenderName string `json:"sender_name"`
	Message    string `json:"message"`
	MessageType string `json:"message_type"`
	ImageURL   string `json:"image_url,omitempty"`
}

type ChatMessageResponse struct {
	ID          string  `json:"id"`
	OrderID     string  `json:"order_id"`
	SenderID    int64   `json:"sender_id"`
	SenderType  string  `json:"sender_type"`
	SenderName  string  `json:"sender_name"`
	Message     string  `json:"message"`
	MessageType string  `json:"message_type"`
	ImageURL    string  `json:"image_url,omitempty"`
	ReadAt      *string `json:"read_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

type ChatWebSocketMessage struct {
	Type    string `json:"type"`
	Payload interface{} `json:"payload"`
}
