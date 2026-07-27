package models

import "time"

// Batch agrupa multiplos pedidos sob um unico entregador na mesma rota.
// E a tabela que sustenta o batching no motor de despacho.
type Batch struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Status    string `gorm:"size:30;not null;default:'active'" json:"status"` // active, delivering, completed, cancelled
	ZoneID    uint   `gorm:"not null;index" json:"zone_id"`
	CourierID *uint  `gorm:"default:null;index" json:"courier_id,omitempty"` // entregador designado (nullable ate ser aceito)

	// Raio maximo de desvio permitido para este lote (km)
	MaxDetourKm float64 `gorm:"default:3.0" json:"max_detour_km"`

	// Coordenadas da rota calculada
	OriginLat      float64 `gorm:"default:0" json:"origin_lat,omitempty"`
	OriginLng      float64 `gorm:"default:0" json:"origin_lng,omitempty"`
	DestinationLat float64 `gorm:"default:0" json:"destination_lat,omitempty"`
	DestinationLng float64 `gorm:"default:0" json:"destination_lng,omitempty"`

	// Metricas
	TotalOrders int     `gorm:"default:0" json:"total_orders"`
	TotalKm     float64 `gorm:"default:0" json:"total_km,omitempty"`     // distancia total da rota
	TotalAmount float64 `gorm:"default:0" json:"total_amount,omitempty"` // valor total dos pedidos

	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Batch) TableName() string {
	return "batches"
}
