// Package services - approval_engine.go
// Motor de decisao de aprovacao de pagamentos.
// Coordena o fluxo: avaliacao de risco -> decisao -> atualizacao no banco.
package services

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

// ApprovalEngine coordena o processamento e decisao de aprovacao.
type ApprovalEngine struct {
	RiskScorer *RiskScorer // Calculador de risco
}

// NewApprovalEngine cria uma nova instancia do motor de aprovacao.
func NewApprovalEngine() *ApprovalEngine {
	return &ApprovalEngine{
		RiskScorer: NewRiskScorer(),
	}
}

// ProcessPayment processa um pagamento: calcula risco, decide aprovacao e salva.
// Se o pagamento for de alto risco (score >= 40), fica pendente para aprovacao manual.
// Se for de baixo risco, e aprovado automaticamente.
func (ae *ApprovalEngine) ProcessPayment(payment *models.Payment) error {
	// 1. Calcula o score de risco do pagamento
	assessment := ae.RiskScorer.AssessPayment(payment)

	// 2. Atualiza os campos de risco no pagamento
	payment.RiskScore = assessment.Score
	payment.RiskLevel = assessment.Level
	payment.RequiresApproval = assessment.RequiresApproval

	// 3. Decide o status baseado no nivel de risco
	if assessment.RequiresApproval {
		// Pagamento de alto risco: fica pendente
		payment.Status = models.PaymentPending
		return repository.CreatePayment(payment)
	}

	// Pagamento de baixo risco: aprovado automaticamente
	payment.Status = models.PaymentApproved
	now := time.Now()
	payment.ApprovedAt = &now
	payment.ApprovedBy = "system" // Aprovado pelo sistema, nao por humano
	return repository.CreatePayment(payment)
}

// ApprovePayment aprova manualmente um pagamento pendente.
// Atualiza o status e registra quem aprovou e quando.
func (ae *ApprovalEngine) ApprovePayment(paymentID string, approvedBy string) error {
	// Converte o ID de hex para ObjectID do MongoDB
	objID, err := repository.HexToObjectID(paymentID)
	if err != nil {
		return err
	}

	// Busca o pagamento no banco
	payment, err := repository.GetPaymentByID(objID)
	if err != nil {
		return err
	}

	// So aprova pagamentos que estao pendentes
	if payment.Status != models.PaymentPending {
		return nil
	}

	// Atualiza o status para aprovado
	now := time.Now()
	return repository.UpdatePaymentStatus(objID, models.PaymentApproved, map[string]interface{}{
		"approved_by": approvedBy,
		"approved_at": now,
	})
}

// RejectPayment rejeita manualmente um pagamento pendente.
// Registra quem rejeitou, quando e o motivo.
func (ae *ApprovalEngine) RejectPayment(paymentID string, rejectedBy string, reason string) error {
	// Converte o ID de hex para ObjectID do MongoDB
	objID, err := repository.HexToObjectID(paymentID)
	if err != nil {
		return err
	}

	// Busca o pagamento no banco
	payment, err := repository.GetPaymentByID(objID)
	if err != nil {
		return err
	}

	// So rejeita pagamentos que estao pendentes
	if payment.Status != models.PaymentPending {
		return nil
	}

	// Atualiza o status para rejeitado
	now := time.Now()
	return repository.UpdatePaymentStatus(objID, models.PaymentRejected, map[string]interface{}{
		"rejected_by":      rejectedBy,
		"rejected_at":      now,
		"rejection_reason": reason,
	})
}
