package models

import "time"

// ReferralCode é o código pessoal de indicação de um cliente ("AMIGO7K3QX").
// Um por telefone; o código não revela o telefone.
type ReferralCode struct {
	ID        uint   `gorm:"primaryKey"`
	Phone     string `gorm:"size:32;not null;uniqueIndex"`
	Code      string `gorm:"size:16;not null;uniqueIndex"`
	CreatedAt time.Time
}

func (ReferralCode) TableName() string { return "referral_codes" }

const (
	ReferralPending  = "pending"  // amigo ganhou o cupom; ainda não recebeu o 1º pedido
	ReferralRewarded = "rewarded" // 1º pedido entregue: quem indicou ganhou os pontos
)

// Referral liga quem indicou ao amigo novo. RefereePhone é único: cada
// pessoa só pode ser indicada uma vez (e só se nunca pediu antes).
type Referral struct {
	ID            uint   `gorm:"primaryKey"`
	ReferrerPhone string `gorm:"size:32;not null;index"`
	RefereePhone  string `gorm:"size:32;not null;uniqueIndex"`
	// CouponCode é o cupom de boas-vindas criado para o amigo; é por ele que
	// o pedido entregue acha a indicação.
	CouponCode string `gorm:"size:40;not null;uniqueIndex"`
	Status     string `gorm:"size:20;not null;index"`
	OrderID    string `gorm:"size:64;index"`
	RewardedAt *time.Time
	CreatedAt  time.Time
}

func (Referral) TableName() string { return "referrals" }
