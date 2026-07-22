package services

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

type ApprovalEngine struct {
	RiskScorer *RiskScorer
}

func NewApprovalEngine() *ApprovalEngine {
	return &ApprovalEngine{
		RiskScorer: NewRiskScorer(),
	}
}

func (ae *ApprovalEngine) ProcessPayment(payment *models.Payment) error {
	assessment := ae.RiskScorer.AssessPayment(payment)

	payment.RiskScore = assessment.Score
	payment.RiskLevel = assessment.Level
	payment.RequiresApproval = assessment.RequiresApproval

	if assessment.RequiresApproval {
		payment.Status = models.PaymentPending
		return repository.CreatePayment(payment)
	}

	payment.Status = models.PaymentApproved
	now := time.Now()
	payment.ApprovedAt = &now
	payment.ApprovedBy = "system"
	return repository.CreatePayment(payment)
}

func (ae *ApprovalEngine) ApprovePayment(paymentID string, approvedBy string) error {
	objID, err := repository.HexToObjectID(paymentID)
	if err != nil {
		return err
	}

	payment, err := repository.GetPaymentByID(objID)
	if err != nil {
		return err
	}

	if payment.Status != models.PaymentPending {
		return nil
	}

	now := time.Now()
	return repository.UpdatePaymentStatus(objID, models.PaymentApproved, map[string]interface{}{
		"approved_by": approvedBy,
		"approved_at": now,
	})
}

func (ae *ApprovalEngine) RejectPayment(paymentID string, rejectedBy string, reason string) error {
	objID, err := repository.HexToObjectID(paymentID)
	if err != nil {
		return err
	}

	payment, err := repository.GetPaymentByID(objID)
	if err != nil {
		return err
	}

	if payment.Status != models.PaymentPending {
		return nil
	}

	now := time.Now()
	return repository.UpdatePaymentStatus(objID, models.PaymentRejected, map[string]interface{}{
		"rejected_by":      rejectedBy,
		"rejected_at":      now,
		"rejection_reason": reason,
	})
}
