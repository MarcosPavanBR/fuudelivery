// Package services - chargeback_service.go
// Servico de gerenciamento de estornos (chargebacks).
// Fornece operacoes CRUD e decisoes de aprovar/rejeitar estornos.
package services

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

// ChargebackService e responsavel pelas operacoes de estorno.
type ChargebackService struct{}

// NewChargebackService cria uma nova instancia do servico.
func NewChargebackService() *ChargebackService {
	return &ChargebackService{}
}

// CreateChargeback cria um novo estorno no banco de dados.
func (cs *ChargebackService) CreateChargeback(chargeback *models.Chargeback) error {
	return repository.CreateChargeback(chargeback)
}

// GetChargeback busca um estorno pelo ID (hex string).
func (cs *ChargebackService) GetChargeback(id string) (*models.Chargeback, error) {
	objID, err := repository.HexToObjectID(id)
	if err != nil {
		return nil, err
	}
	return repository.GetChargebackByID(objID)
}

// ApproveChargeback aprova um estorno, registrando quem aprovou e quando.
func (cs *ChargebackService) ApproveChargeback(id string, resolvedBy string) error {
	objID, err := repository.HexToObjectID(id)
	if err != nil {
		return err
	}

	now := time.Now()
	return repository.UpdateChargebackStatus(objID, models.ChargebackApproved, map[string]interface{}{
		"resolved_by": resolvedBy,
		"resolved_at": now,
		"resolution":  "Chargeback approved - refund processed",
	})
}

// RejectChargeback rejeita um estorno com motivo.
func (cs *ChargebackService) RejectChargeback(id string, resolvedBy string, reason string) error {
	objID, err := repository.HexToObjectID(id)
	if err != nil {
		return err
	}

	now := time.Now()
	return repository.UpdateChargebackStatus(objID, models.ChargebackRejected, map[string]interface{}{
		"resolved_by": resolvedBy,
		"resolved_at": now,
		"resolution":  reason,
	})
}

// ListChargebacks lista estornos com filtro e paginacao.
func (cs *ChargebackService) ListChargebacks(status string, page, limit int) ([]models.Chargeback, int64, error) {
	return repository.ListChargebacks(status, page, limit)
}
