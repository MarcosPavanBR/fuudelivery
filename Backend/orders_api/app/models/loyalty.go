package models

import "time"

type LoyaltyPoints struct {
	ID          uint      `gorm:"primaryKey"`
	UserPhone   string    `gorm:"index;not null"`
	Points      int       `gorm:"default:0"`
	Tier        string    `gorm:"default:'bronze'"`
	TotalOrders int       `gorm:"default:0"`
	TotalSpent  float64   `gorm:"default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type LoyaltyTransaction struct {
	ID          uint      `gorm:"primaryKey"`
	UserPhone   string    `gorm:"index;not null"`
	Points      int       `gorm:"not null"`
	Type        string    `gorm:"not null"`
	Description string
	OrderID     string
	CreatedAt   time.Time
}
