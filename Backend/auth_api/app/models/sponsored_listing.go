package models

import "time"

// SponsoredListing é o patrocínio MENSAL antigo (tabela sponsored_listings).
// Aposentado: só o admin criava, não havia cobrança e nenhuma tela usava.
// O destaque agora é por dia — ver SponsorBooking (sponsor_booking.go).
// O struct fica só para o AutoMigrate não estranhar a tabela existente.
type SponsoredListing struct {
	ID                  uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	EstablishmentID     uint       `gorm:"not null;uniqueIndex:idx_sponsored_est_zone" json:"establishment_id"`
	ZoneID              uint       `gorm:"not null;uniqueIndex:idx_sponsored_est_zone" json:"zone_id"`
	Plan                string     `gorm:"size:20;not null;default:'basic'" json:"plan"`
	Status              string     `gorm:"size:20;not null;default:'active'" json:"status"`
	Amount              float64    `gorm:"not null" json:"amount"`
	StartDate           time.Time  `json:"start_date"`
	EndDate             time.Time  `json:"end_date"`
	CancelledAt         *time.Time `json:"cancelled_at,omitempty"`
	Priority            int        `gorm:"not null;default:0" json:"priority"`
	HasBanner           bool       `gorm:"not null;default:false" json:"has_banner"`
	HasPushNotification bool       `gorm:"not null;default:false" json:"has_push_notification"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (SponsoredListing) TableName() string {
	return "sponsored_listings"
}
