package services

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// BatchExpiryConfig contem a configuracao do job de expiracao de batches.
type BatchExpiryConfig struct {
	// Intervalo entre verificacoes
	Interval time.Duration
	// Idade maxima de um batch em active/delivering antes de expirar
	MaxBatchAge time.Duration
	// Se deve cancelar ou marcar como completed
	ExpireAction string // "cancel" ou "complete"
}

// DefaultBatchExpiryConfig retorna a configuracao padrao.
func DefaultBatchExpiryConfig() BatchExpiryConfig {
	return BatchExpiryConfig{
		Interval:     30 * time.Minute, // verifica a cada 30min
		MaxBatchAge:  24 * time.Hour,   // expira apos 24h
		ExpireAction: "cancelled",      // marca como cancelado
	}
}

// BatchExpiryManager gerencia a expiracao automatica de batches.
type BatchExpiryManager struct {
	config BatchExpiryConfig
	db     *gorm.DB
	stopCh chan struct{}
}

// NewBatchExpiryManager cria um novo gerenciador de expiracao.
func NewBatchExpiryManager(db *gorm.DB, config BatchExpiryConfig) *BatchExpiryManager {
	return &BatchExpiryManager{
		config: config,
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start inicia o job de expiracao em background.
func (m *BatchExpiryManager) Start() {
	go func() {
		ticker := time.NewTicker(m.config.Interval)
		defer ticker.Stop()

		log.Printf("[BATCH_EXPIRY] Started with interval=%s, max_age=%s, action=%s",
			m.config.Interval, m.config.MaxBatchAge, m.config.ExpireAction)

		// Executa uma vez na inicializacao
		m.expireBatches()

		for {
			select {
			case <-ticker.C:
				m.expireBatches()
			case <-m.stopCh:
				log.Println("[BATCH_EXPIRY] Stopped")
				return
			}
		}
	}()
}

// Stop para o job de expiracao.
func (m *BatchExpiryManager) Stop() {
	close(m.stopCh)
}

// expireBatches encontra batches expirados e os atualiza.
func (m *BatchExpiryManager) expireBatches() {
	if m.db == nil {
		return
	}

	cutoff := time.Now().Add(-m.config.MaxBatchAge)

	// Busca batches active ou delivering criados antes do cutoff
	var expiredBatches []struct {
		ID     uint
		Status string
	}

	result := m.db.Table("batches").
		Select("id, status").
		Where("status IN ('active', 'delivering') AND created_at < ?", cutoff).
		Scan(&expiredBatches)

	if result.Error != nil {
		log.Printf("[BATCH_EXPIRY] Query error: %v", result.Error)
		return
	}

	if len(expiredBatches) == 0 {
		return // nada a expirar
	}

	// Atualiza batches expirados
	now := time.Now()
	for _, b := range expiredBatches {
		updates := map[string]interface{}{
			"status":       m.config.ExpireAction,
			"completed_at": now,
			"updated_at":   now,
		}
		if m.config.ExpireAction == "cancelled" {
			delete(updates, "completed_at") // batch cancelado nao tem completed_at
		}

		if err := m.db.Table("batches").Where("id = ?", b.ID).Updates(updates).Error; err != nil {
			log.Printf("[BATCH_EXPIRY] Failed to expire batch %d: %v", b.ID, err)
			continue
		}

		log.Printf("[BATCH_EXPIRY] Batch %d expired (%s -> %s, age > %v)",
			b.ID, b.Status, m.config.ExpireAction, m.config.MaxBatchAge)
	}
}

// ForceExpire expira manualmente um batch especifico.
// Usado pelo endpoint admin POST /batches/:id/force-expire.
func (m *BatchExpiryManager) ForceExpire(batchID uint) error {
	if m.db == nil {
		return nil
	}

	updates := map[string]interface{}{
		"status":     m.config.ExpireAction,
		"updated_at": time.Now(),
	}

	return m.db.Table("batches").Where("id = ?", batchID).Updates(updates).Error
}
