package models

import "time"

// Quem absorve o desconto do cupom.
const (
	CouponFundedByPlatform      = "platform"
	CouponFundedByEstablishment = "establishment"
)

type Coupon struct {
	ID              uint   `gorm:"primaryKey"`
	Code            string `gorm:"uniqueIndex;not null"`
	Description     string
	DiscountType    string  `gorm:"not null"`
	DiscountValue   float64 `gorm:"not null"`
	MinOrderValue   float64
	MaxUses         int
	MaxUsesPerUser  int
	UsedCount       int `gorm:"default:0"`
	StartDate       time.Time
	ExpiryDate      time.Time
	IsActive        bool `gorm:"default:true"`
	EstablishmentID uint

	// FundedBy diz de QUEM sai o desconto: "platform" (a taxa da plataforma
	// absorve) ou "establishment" (o restaurante absorve). Sem isto o split
	// não teria como saber de qual lado subtrair — o desconto sairia de
	// ninguém, e o pedido fecharia com a soma das partes maior que o pago.
	//
	// Default "platform": um cupom criado sem escolha explícita é promoção da
	// plataforma. Errar para "o restaurante paga" seria tirar dinheiro de
	// terceiro por omissão.
	FundedBy string `gorm:"size:20;not null;default:platform" json:"funded_by"`

	OwnerPhone string `gorm:"index"`
	CreatedBy  uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CouponUsage struct {
	ID             uint   `gorm:"primaryKey"`
	CouponID       uint   `gorm:"not null"`
	UserPhone      string `gorm:"not null"`
	OrderID        string `gorm:"not null"`
	DiscountAmount float64
	UsedAt         time.Time
}
