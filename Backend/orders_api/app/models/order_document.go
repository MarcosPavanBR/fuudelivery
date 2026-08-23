package models

import (
	"time"
)

// OrderDocument espelha, em Postgres, os documentos da collection Mongo
// "orders" (corte 5 da migração banco-único — ver docs/ARQUITETURA-BANCO-UNICO.md).
//
// Estratégia de modelagem:
//   - O documento original (dto.RequestPayload) é profundamente aninhado e
//     evolui junto com o frontend. Em vez de tipar cada campo (alto risco de
//     drift de schema), guardamos o payload completo como JSONB na coluna
//     `payload` e extraímos para COLUNAS TIPADAS apenas os campos usados em
//     filtros/índices: estabelecimento, telefone do cliente, status,
//     agendamento e código de retirada.
//   - `legacy_id` preserva o formato de ID que TODOS os clientes já conhecem
//     (ObjectID hex de 24 chars): apps mobile, webs, delivery_api e reviews
//     referenciam pedidos por essa string. Novos pedidos continuam gerando
//     IDs no mesmo formato — nada muda para o consumidor.
//
// Durante a transição o handler faz dual-write (Postgres primário + Mongo
// best-effort); a leitura é Postgres-first com fallback/lazy-import do Mongo.
// Quando o Atlas for desligado, remover a escrita legada em orders_pg.go.
type OrderDocument struct {
	// ID é a PK sequencial interna do Postgres (não exposta aos clientes).
	ID int64 `gorm:"primaryKey"`

	// LegacyID é o identificador público do pedido (ObjectID hex legado).
	LegacyID string `gorm:"column:legacy_id;uniqueIndex;size:32"`

	// Colunas tipadas — extraídas do payload para permitir índices e filtros
	// eficientes sem desserializar o JSONB.
	EstablishmentID int64  `gorm:"index;column:establishment_id"`
	UserPhone       string `gorm:"index;size:32;column:user_phone"`
	Status          string `gorm:"index;size:40"`
	PickupCode      string `gorm:"size:6;column:pickup_code"`
	ScheduledAt     *time.Time
	IsScheduled     bool `gorm:"column:is_scheduled"`

	// Payload é o documento completo (dto.RequestPayload serializado).
	Payload []byte `gorm:"type:jsonb;column:payload"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName fixa o nome da tabela (GORM pluralizaria para "order_documents"
// por padrão, mas deixamos explícito para auditoria e clareza).
func (OrderDocument) TableName() string { return "order_documents" }
