package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Wallet struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      int64              `bson:"user_id" json:"user_id"`
	UserType    string             `bson:"user_type" json:"user_type"`
	Balance     float64            `bson:"balance" json:"balance"`
	LastUpdated time.Time          `bson:"last_updated" json:"last_updated"`
}
