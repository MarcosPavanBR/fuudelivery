package services

import (
	"github.com/carloshomar/vercardapio/payment/models"
)

type Handler interface {
	Handle(payment *models.Payment) error
	SetNext(handler Handler) Handler
}

type BaseHandler struct {
	next Handler
}

func (h *BaseHandler) SetNext(handler Handler) Handler {
	h.next = handler
	return handler
}

func (h *BaseHandler) passToNext(payment *models.Payment) error {
	if h.next != nil {
		return h.next.Handle(payment)
	}
	return nil
}

type ValidationHandler struct {
	BaseHandler
}

func (vh *ValidationHandler) Handle(payment *models.Payment) error {
	if payment.OrderID == "" {
		return nil
	}
	if payment.Amount <= 0 {
		return nil
	}
	if payment.CustomerID == "" {
		return nil
	}
	return vh.passToNext(payment)
}

type RiskCheckHandler struct {
	BaseHandler
	Scorer *RiskScorer
}

func (rh *RiskCheckHandler) Handle(payment *models.Payment) error {
	assessment := rh.Scorer.AssessPayment(payment)
	payment.RiskScore = assessment.Score
	payment.RiskLevel = assessment.Level
	payment.RequiresApproval = assessment.RequiresApproval
	return rh.passToNext(payment)
}

type ApprovalHandler struct {
	BaseHandler
}

func (ah *ApprovalHandler) Handle(payment *models.Payment) error {
	if payment.RequiresApproval {
		payment.Status = models.PaymentPending
		return nil
	}
	payment.Status = models.PaymentApproved
	return ah.passToNext(payment)
}

type NotificationHandler struct {
	BaseHandler
}

func (nh *NotificationHandler) Handle(payment *models.Payment) error {
	return nh.passToNext(payment)
}

func BuildChain() Handler {
	validation := &ValidationHandler{}
	riskCheck := &RiskCheckHandler{Scorer: NewRiskScorer()}
	approval := &ApprovalHandler{}
	notification := &NotificationHandler{}

	validation.SetNext(riskCheck).SetNext(approval).SetNext(notification)

	return validation
}
