package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
)

// DB é a conexão GORM/Postgres do delivery_api. Fica nil (degradando para
// Mongo-only) enquanto o monólito não a injetar via InitPostgres — é o
// comportamento do corte: o Postgres é a fonte primária e o Mongo o espelho.
var DB *gorm.DB

// InitPostgres injeta a conexão GORM compartilhada (mesmo *gorm.DB do
// monólito). Nil-safe: se db == nil, o repository opera só no Mongo.
// O schema NÃO é migrado aqui — a tabela delivery_solicitations é criada
// pelo script sql/02_dominio_entrega.sql (fonte de verdade do schema).
func InitPostgres(db *gorm.DB) {
	DB = db
	if db != nil {
		log.Println("[DELIVERY_PG] Postgres conectado (tabela delivery_solicitations)")
	}
}

// JSONB é um tipo serializado como jsonb no Postgres (evita dependência
// externa de datatypes). Products é um snapshot denormalizado do pedido.
type JSONB []byte

// Value implementa driver.Valuer para escrita.
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "[]", nil
	}
	return string(j), nil
}

// Scan implementa sql.Scanner para leitura.
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = []byte("[]")
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = append((*j)[:0], v...)
	case string:
		*j = []byte(v)
	default:
		return fmt.Errorf("JSONB: tipo inesperado %T", value)
	}
	return nil
}

// DeliverySolicitation é o read-model do pedido usado pelo motor de
// despacho (tabela delivery_solicitations). Substitui a collection Mongo
// "solicitations" (struct dto.OrderDTO). Mantém os dados denormalizados
// de propósito: o matching lê rápido, sem JOIN contra orders_api.
type DeliverySolicitation struct {
	ID                   int64     `gorm:"column:id;primaryKey"`
	OrderID              string    `gorm:"column:order_id;size:100;not null;uniqueIndex"`
	Status               string    `gorm:"column:status;size:30;not null;default:pending"`
	EstablishmentID      int64     `gorm:"column:establishment_id;not null"`
	EstablishmentName    string    `gorm:"column:establishment_name;size:255"`
	EstablishmentLat     float64   `gorm:"column:establishment_lat"`
	EstablishmentLong    float64   `gorm:"column:establishment_long"`
	EstablishmentAddress string    `gorm:"column:establishment_address;size:500"`
	EstablishmentPhone   string    `gorm:"column:establishment_phone;size:30"`
	EstablishmentImage   string    `gorm:"column:establishment_image"`
	UserID               int64     `gorm:"column:user_id;not null"`
	UserName             string    `gorm:"column:user_name;size:255"`
	UserPhone            string    `gorm:"column:user_phone;size:30"`
	DeliveryManID        *int64    `gorm:"column:delivery_man_id"`
	DeliveryManName      string    `gorm:"column:delivery_man_name;size:255"`
	DeliveryManStatus    string    `gorm:"column:delivery_man_status;size:20"`
	Products             JSONB     `gorm:"column:products;type:jsonb"`
	Total                float64   `gorm:"column:total"`
	PaymentMethod        string    `gorm:"column:payment_method;size:30"`
	PaymentChange        float64   `gorm:"column:payment_change"`
	ZoneID               *int64    `gorm:"column:zone_id"`
	MatchRadiusKm        float64   `gorm:"column:match_radius_km;default:5.0"`
	BatchID              *int64    `gorm:"column:batch_id"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

// TableName fixa o nome da tabela (plural convencional do GORM).
func (DeliverySolicitation) TableName() string { return "delivery_solicitations" }

// FromOrderDTO converte o DTO da fila/Mongo para a linha Postgres.
func FromOrderDTO(d dto.OrderDTO) DeliverySolicitation {
	var dmID *int64
	if d.DeliveryMan.Id != 0 {
		v := d.DeliveryMan.Id
		dmID = &v
	}
	var zoneID *int64
	if d.ZoneID != nil {
		v := int64(*d.ZoneID)
		zoneID = &v
	}

	createdAt := time.Now()
	if d.CreatedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, d.CreatedAt); err == nil {
			createdAt = parsed
		} else if parsed, err := time.Parse("2006-01-02 15:04:05", d.CreatedAt); err == nil {
			createdAt = parsed
		}
	}

	products, _ := json.Marshal(d.Products)
	if len(products) == 0 {
		products = []byte("[]")
	}

	return DeliverySolicitation{
		OrderID:              d.OrderId,
		Status:               d.Status,
		EstablishmentID:      d.Establishment.Id,
		EstablishmentName:    d.Establishment.Name,
		EstablishmentLat:     d.Establishment.Lat,
		EstablishmentLong:    d.Establishment.Long,
		EstablishmentAddress: d.Establishment.Address,
		EstablishmentPhone:   d.Establishment.Phone,
		EstablishmentImage:   d.Establishment.Image,
		UserID:               d.User.ID,
		UserName:             d.User.Name,
		UserPhone:            d.User.Phone,
		DeliveryManID:        dmID,
		DeliveryManName:      d.DeliveryMan.Name,
		DeliveryManStatus:    d.DeliveryMan.Status,
		Products:             products,
		Total:                d.Total,
		PaymentMethod:        d.Payment.Method,
		PaymentChange:        d.Payment.Change,
		ZoneID:               zoneID,
		MatchRadiusKm:        5.0,
		CreatedAt:            createdAt,
		UpdatedAt:            createdAt,
	}
}

// ToOrderDTO converte a linha Postgres de volta para o DTO (formato da API).
func (r DeliverySolicitation) ToOrderDTO() dto.OrderDTO {
	var dm dto.DeliveryManDTO
	if r.DeliveryManID != nil && *r.DeliveryManID != 0 {
		dm = dto.DeliveryManDTO{
			Id:     *r.DeliveryManID,
			Name:   r.DeliveryManName,
			Status: r.DeliveryManStatus,
		}
	}

	var zoneID *uint
	if r.ZoneID != nil {
		v := uint(*r.ZoneID)
		zoneID = &v
	}

	var products []dto.ProductDTO
	if len(r.Products) > 0 {
		_ = json.Unmarshal(r.Products, &products)
	}
	if products == nil {
		products = []dto.ProductDTO{}
	}

	return dto.OrderDTO{
		OrderId: r.OrderID,
		Status:  r.Status,
		Establishment: dto.EstablishmentDTO{
			Id:      r.EstablishmentID,
			Name:    r.EstablishmentName,
			Lat:     r.EstablishmentLat,
			Long:    r.EstablishmentLong,
			Address: r.EstablishmentAddress,
			Phone:   r.EstablishmentPhone,
			Image:   r.EstablishmentImage,
		},
		User: dto.UserDTO{
			ID:    r.UserID,
			Name:  r.UserName,
			Phone: r.UserPhone,
		},
		Products:    products,
		DeliveryMan: dm,
		Total:       r.Total,
		Payment: dto.PaymentDTO{
			Method: r.PaymentMethod,
			Change: r.PaymentChange,
		},
		CreatedAt:    r.CreatedAt.Format(time.RFC3339),
		LastModified: r.UpdatedAt.Format(time.RFC3339),
		ZoneID:       zoneID,
	}
}
