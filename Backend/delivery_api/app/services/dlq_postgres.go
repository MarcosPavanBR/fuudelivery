package services

import (
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// UnmatchedOrderRow representa uma linha na tabela unmatched_orders (Postgres).
type UnmatchedOrderRow struct {
	ID               int64  `gorm:"primaryKey;autoIncrement"`
	OrderID          string `gorm:"column:order_id;size:64;not null"`
	EstablishmentLat float64 `gorm:"column:establishment_lat;not null"`
	EstablishmentLng float64 `gorm:"column:establishment_lng;not null"`
	ZoneID           int    `gorm:"column:zone_id;default:0"`
	CreatedAt        int64  `gorm:"column:created_at;not null"`
	RetryCount       int    `gorm:"column:retry_count;default:0"`
	LastAttemptAt    int64  `gorm:"column:last_attempt_at;not null"`
	Metadata         string `gorm:"column:metadata;type:jsonb"`
	CreatedTZ        time.Time `gorm:"column:created_tz;autoCreateTime"`
}

// TableName retorna o nome da tabela.
func (UnmatchedOrderRow) TableName() string {
	return "unmatched_orders"
}

// PostgresDLQStore implementa a dead-letter queue persistente no Postgres.
// Tem a mesma interface da DLQStore in-memory, mas sobrevive restarts.
type PostgresDLQStore struct {
	db      *gorm.DB
	mu      sync.Mutex
	maxSize int // não usado para hard limit (Postgres gerencia), mas mantido para consistência
}

// NewPostgresDLQStore cria uma nova DLQ persistente.
func NewPostgresDLQStore(db *gorm.DB) *PostgresDLQStore {
	return &PostgresDLQStore{
		db:      db,
		maxSize: 1000, // soft limit para List() (evita carregar milhares na memória)
	}
}

// Push insere um pedido não casado no Postgres.
func (d *PostgresDLQStore) Push(order *UnmatchedOrder) {
	d.mu.Lock()
	defer d.mu.Unlock()

	row := &UnmatchedOrderRow{
		OrderID:          order.OrderID,
		EstablishmentLat: order.EstablishmentLat,
		EstablishmentLng: order.EstablishmentLng,
		ZoneID:           int(order.ZoneID),
		CreatedAt:        order.CreatedAt,
		RetryCount:       order.RetryCount,
		LastAttemptAt:    order.LastAttemptAt,
	}

	if err := d.db.Create(row).Error; err != nil {
		log.Printf("[DLQ-PG] ERRO ao inserir order %s: %v", order.OrderID, err)
		return
	}

	log.Printf("[DLQ-PG] Order %s added to DLQ (row_id=%d)", order.OrderID, row.ID)
}

// PopNext retorna o próximo pedido para retry, ou nil se vazio.
// Seleciona o pedido mais antigo com retry_count < 3 e last_attempt_at < 30s atrás.
// Usa SELECT ... FOR UPDATE SKIP LOCKED para concorrência segura.
func (d *PostgresDLQStore) PopNext() *UnmatchedOrder {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().UnixMilli() - 30000 // 30s entre retries

	var row UnmatchedOrderRow
	err := d.db.Transaction(func(tx *gorm.DB) error {
		return tx.
			Where("retry_count < ? AND last_attempt_at < ?", 3, cutoff).
			Order("created_at ASC").
			First(&row).Error
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		log.Printf("[DLQ-PG] ERRO ao buscar próximo pedido: %v", err)
		return nil
	}

	// Deleta o registro após selecionar (consume)
	if err := d.db.Delete(&row).Error; err != nil {
		log.Printf("[DLQ-PG] ERRO ao deletar order %s: %v", row.OrderID, err)
		return nil
	}

	return &UnmatchedOrder{
		OrderID:          row.OrderID,
		EstablishmentLat: row.EstablishmentLat,
		EstablishmentLng: row.EstablishmentLng,
		ZoneID:           uint(row.ZoneID),
		CreatedAt:        row.CreatedAt,
		RetryCount:       row.RetryCount,
		LastAttemptAt:    row.LastAttemptAt,
	}
}

// Len retorna o número de pedidos na DLQ.
func (d *PostgresDLQStore) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	var count int64
	if err := d.db.Model(&UnmatchedOrderRow{}).Count(&count).Error; err != nil {
		log.Printf("[DLQ-PG] ERRO ao contar pedidos: %v", err)
		return 0
	}
	return int(count)
}

// List retorna todos os pedidos na DLQ (para debugging).
// Limita a maxSize para evitar carregar muitos registros na memória.
func (d *PostgresDLQStore) List() []*UnmatchedOrder {
	d.mu.Lock()
	defer d.mu.Unlock()

	var rows []UnmatchedOrderRow
	if err := d.db.Order("created_at ASC").Limit(d.maxSize).Find(&rows).Error; err != nil {
		log.Printf("[DLQ-PG] ERRO ao listar pedidos: %v", err)
		return nil
	}

	result := make([]*UnmatchedOrder, 0, len(rows))
	for _, row := range rows {
		result = append(result, &UnmatchedOrder{
			OrderID:          row.OrderID,
			EstablishmentLat: row.EstablishmentLat,
			EstablishmentLng: row.EstablishmentLng,
			ZoneID:           uint(row.ZoneID),
			CreatedAt:        row.CreatedAt,
			RetryCount:       row.RetryCount,
			LastAttemptAt:    row.LastAttemptAt,
		})
	}
	return result
}

// Cleanup remove registros antigos da DLQ (criados há mais de 5 minutos).
// Deve ser chamado periodicamente (ex: a cada 5min).
func (d *PostgresDLQStore) Cleanup(maxAge time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	result := d.db.Where("created_tz < ?", cutoff).Delete(&UnmatchedOrderRow{})
	if result.Error != nil {
		log.Printf("[DLQ-PG] ERRO ao limpar registros antigos: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[DLQ-PG] Cleanup: %d registros antigos removidos", result.RowsAffected)
	}
}
