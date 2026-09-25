package models

type DeliveryMan struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Phone    string `json:"phone"`

	PaymentWalletID string `json:"-" gorm:"size:100"` // fora do JSON do modelo; PUT /delivery-man/:id/wallet devolve ao dono

	// === Campos do motor de despacho ===
	// Vinculo com a zona de atuacao principal
	ZoneID *uint `gorm:"default:null" json:"zone_id,omitempty"`

	// Localizacao em tempo real (atualizada pelo courier app)
	CurrentLat float64 `gorm:"default:0" json:"current_lat,omitempty"`
	CurrentLng float64 `gorm:"default:0" json:"current_lng,omitempty"`

	// Status do entregador: available, busy, offline
	Status string `gorm:"size:20;default:'offline'" json:"status"`

	// Maximo de pedidos simultaneos (batching)
	MaxOrders int `gorm:"default:3" json:"max_orders"`

	// Ultima atualizacao de localizacao (unix millis)
	LastUpdate int64 `gorm:"default:0" json:"last_update,omitempty"`
}

func (DeliveryMan) TableName() string {
	return "delivery_men"
}
