package models

import "time"

type Coupon struct {
	ID              uint      `gorm:"primaryKey"`
	Code            string    `gorm:"uniqueIndex;not null"`
	Description     string
	DiscountType    string    `gorm:"not null"`
	DiscountValue   float64   `gorm:"not null"`
	MinOrderValue   float64
	MaxUses         int
	MaxUsesPerUser  int
	UsedCount       int       `gorm:"default:0"`
	StartDate       time.Time
	ExpiryDate      time.Time
	IsActive        bool      `gorm:"default:true"`
	EstablishmentID uint
	CreatedBy       uint
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CouponUsage struct {
	ID              uint      `gorm:"primaryKey"`
	CouponID        uint      `gorm:"not null"`
	UserPhone       string    `gorm:"not null"`
	OrderID         string    `gorm:"not null"`
	DiscountAmount  float64
	UsedAt          time.Time
}
