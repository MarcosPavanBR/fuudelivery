package models

type Order struct {
	ID              uint `gorm:"primaryKey"`
	UserID          uint `gorm:"foreignKey:IDUsuario"`
	EstablishmentID uint
	OrderDate       string
	Status          string // Pending, In Progress, Delivered, etc.

	// === Motor de despacho ===
	// Vinculo com lote de entrega (batching)
	BatchID *uint `gorm:"default:null;index" json:"batch_id,omitempty"`
	// Raio de busca usado no matching (km)
	MatchRadiusKm float64 `gorm:"default:5.0" json:"match_radius_km,omitempty"`
	// Zona onde o pedido foi processado
	ZoneID *uint `gorm:"default:null;index" json:"zone_id,omitempty"`
}
