package models

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatMessage struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderID     string             `bson:"order_id" json:"order_id"`
	SenderID    int64              `bson:"sender_id" json:"sender_id"`
	SenderType  string             `bson:"sender_type" json:"sender_type"`
	SenderName  string             `bson:"sender_name" json:"sender_name"`
	Message     string             `bson:"message" json:"message"`
	MessageType string             `bson:"message_type" json:"message_type"`
	ImageURL    string             `bson:"image_url,omitempty" json:"image_url,omitempty"`
	ReadAt      *time.Time         `bson:"read_at,omitempty" json:"read_at,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}
