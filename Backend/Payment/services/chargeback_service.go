package services

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

type ChargebackService struct{}

func NewChargebackService() *ChargebackService {
	return &ChargebackService{}
}

func (cs *ChargebackService) CreateChargeback(chargeback *models.Chargeback) error {
	return repository.CreateChargeback(chargeback)
}

func (cs *ChargebackService) GetChargeback(id string) (*models.Chargeback, error) {
	objID, err := repository.HexToObjectID(id)
	if err != nil {
		return nil, err
	}
	return repository.GetChargebackByID(objID)
}

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

func (cs *ChargebackService) ListChargebacks(status string, page, limit int) ([]models.Chargeback, int64, error) {
	return repository.ListChargebacks(status, page, limit)
}
