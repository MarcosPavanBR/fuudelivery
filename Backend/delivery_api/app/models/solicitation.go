package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
)

// ============================================================================
// DeliverySolicitation — read-model do pedido usado pelo motor de despacho.
//
// CORTE 3 da migração banco-único (docs/ARQUITETURA-BANCO-UNICO.md):
// substitui a collection MongoDB "solicitations" pela tabela Postgres
// "delivery_solicitations" criada em sql/02_dominio_entrega.sql.
//
// Estratégia adotada (igual aos cortes 1 e 2):
//   - Escrita: Postgres é a fonte da verdade; Mongo permanece como
//     dual-write best-effort durante a transição (ver handlers).
//   - Leitura: 100% Postgres.
//
// Os campos denormalizados (establishment_*, user_*, products snapshot)
// são intencionais: o motor de matching precisa ler rápido sem JOIN
// contra orders_api. Ver comentário do sql/02.
// ============================================================================

// ProductList serializa o snapshot de produtos como JSONB na coluna `products`.
type ProductList []dto.ProductDTO

// Value implementa driver.Valuer — grava o slice como JSON no Postgres.
func (p ProductList) Value() (driver.Value, error) {
	if p == nil {
		return "[]", nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("serializar products: %w", err)
	}
	return string(b), nil
}

// Scan implementa sql.Scanner — lê o JSONB de volta para o slice.
func (p *ProductList) Scan(src interface{}) error {
	if src == nil {
		*p = ProductList{}
		return nil
	}
	var bytes []byte
	switch v := src.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("tipo inesperado para products: %T", src)
	}
	if len(bytes) == 0 {
		*p = ProductList{}
		return nil
	}
	return json.Unmarshal(bytes, p)
}

type DeliverySolicitation struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	OrderID   string    `gorm:"column:order_id;uniqueIndex" json:"order_id"`
	Status    string    `gorm:"column:status" json:"status"`
	Operation time.Time `gorm:"column:updated_at" json:"operation_date"` // espelha o antigo campo operationDate do Mongo

	EstablishmentID      int64   `gorm:"column:establishment_id" json:"establishment_id"`
	EstablishmentName    string  `gorm:"column:establishment_name" json:"establishment_name"`
	EstablishmentLat     float64 `gorm:"column:establishment_lat" json:"establishment_lat"`
	EstablishmentLong    float64 `gorm:"column:establishment_long" json:"establishment_long"`
	EstablishmentAddress string  `gorm:"column:establishment_address" json:"establishment_address"`
	EstablishmentPhone   string  `gorm:"column:establishment_phone" json:"establishment_phone"`
	EstablishmentImage   string  `gorm:"column:establishment_image" json:"establishment_image"`

	UserID    int64  `gorm:"column:user_id" json:"user_id"`
	UserName  string `gorm:"column:user_name" json:"user_name"`
	UserPhone string `gorm:"column:user_phone" json:"user_phone"`

	DeliveryManID     int64  `gorm:"column:delivery_man_id" json:"delivery_man_id"` // 0 = não atribuído (equivalente ao nil/0 do Mongo)
	DeliveryManName   string `gorm:"column:delivery_man_name" json:"delivery_man_name"`
	DeliveryManStatus string `gorm:"column:delivery_man_status" json:"delivery_man_status"`

	Products ProductList `gorm:"column:products;type:jsonb" json:"products"`

	Total         float64 `gorm:"column:total" json:"total"`
	PaymentMethod string  `gorm:"column:payment_method" json:"payment_method"`
	PaymentChange float64 `gorm:"column:payment_change" json:"payment_change"`

	ZoneID *int64 `gorm:"column:zone_id" json:"zone_id"`

	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"-"`
}

// TableName fixa o nome da tabela (evita surpresa de pluralização do GORM).
func (DeliverySolicitation) TableName() string { return "delivery_solicitations" }

// FromDTO converte o OrderDTO (formato dos apps/filas) para o modelo relacional.
// Mantido como método para centralizar o mapeamento num só lugar — se um campo
// novo aparecer no OrderDTO, é AQUI que ele entra no read-model.
func (d *DeliverySolicitation) FromDTO(o dto.OrderDTO) {
	d.OrderID = o.OrderId
	d.Status = o.Status
	d.EstablishmentID = o.Establishment.Id
	d.EstablishmentName = o.Establishment.Name
	d.EstablishmentLat = o.Establishment.Lat
	d.EstablishmentLong = o.Establishment.Long
	d.EstablishmentAddress = o.Establishment.Address
	d.EstablishmentPhone = o.Establishment.Phone
	d.EstablishmentImage = o.Establishment.Image
	d.UserID = o.User.ID
	d.UserName = o.User.Name
	d.UserPhone = o.User.Phone
	d.DeliveryManID = o.DeliveryMan.Id
	d.DeliveryManName = o.DeliveryMan.Name
	d.DeliveryManStatus = o.DeliveryMan.Status
	d.Products = ProductList(o.Products)
	d.Total = o.Total
	d.PaymentMethod = o.Payment.Method
	d.PaymentChange = o.Payment.Change
	if o.ZoneID != nil {
		z := int64(*o.ZoneID)
		d.ZoneID = &z
	}
}

// ToDTO reconstrói o OrderDTO a partir do modelo relacional (respostas de API
// continuam com exatamente o mesmo JSON de antes — contrato com os apps intacto).
func (d DeliverySolicitation) ToDTO() dto.OrderDTO {
	o := dto.OrderDTO{
		OrderId:  d.OrderID,
		Status:   d.Status,
		Total:    d.Total,
		Products: []dto.ProductDTO(d.Products),
		Establishment: dto.EstablishmentDTO{
			Id:      d.EstablishmentID,
			Name:    d.EstablishmentName,
			Lat:     d.EstablishmentLat,
			Long:    d.EstablishmentLong,
			Address: d.EstablishmentAddress,
			Phone:   d.EstablishmentPhone,
			Image:   d.EstablishmentImage,
		},
		User: dto.UserDTO{
			ID:    d.UserID,
			Name:  d.UserName,
			Phone: d.UserPhone,
		},
		Payment: dto.PaymentDTO{
			Method: d.PaymentMethod,
			Change: d.PaymentChange,
		},
		LastModified: d.UpdatedAt.Format(time.RFC3339),
	}
	if d.DeliveryManID != 0 || d.DeliveryManName != "" || d.DeliveryManStatus != "" {
		o.DeliveryMan = dto.DeliveryManDTO{
			Id:     d.DeliveryManID,
			Name:   d.DeliveryManName,
			Status: d.DeliveryManStatus,
		}
	}
	if d.ZoneID != nil {
		z := uint(*d.ZoneID)
		o.ZoneID = &z
	}
	return o
}
