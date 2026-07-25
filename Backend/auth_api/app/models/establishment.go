package models

type Establishment struct {
	ID                   uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                 string `json:"name"`
	HorarioFuncionamento string `json:"horarioFuncionamento"`
	Description          string `json:"description"`
	OwnerID              uint   `json:"owner_id"`
	Image                string `json:"image"`
	PrimaryColor         string `json:"primary_color"`
	SecondaryColor       string `json:"secondary_color"`

	Lat                 float64 `json:"lat"`
	Long                float64 `json:"long"`
	LocationString      string  `json:"location_string"`
	MaxDistanceDelivery float64 `json:"max_distance_delivery"`

	OpenData *string `json:"open_data,omitempty"`

	PaymentWalletID string `json:"payment_wallet_id,omitempty" gorm:"size:100"`

	// ZoneID vincula o estabelecimento a uma praça/regiao
	// que define as regras de split de pagamento (percentuais
	// plataforma x estabelecimento). Se nil, usa 5/85 padrao.
	ZoneID *uint `gorm:"default:null" json:"zone_id,omitempty"`
}
