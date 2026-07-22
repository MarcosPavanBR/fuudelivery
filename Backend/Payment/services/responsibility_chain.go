// Package services - responsibility_chain.go
// Implementa o padrao Chain of Responsibility para processamento de pagamentos.
// Cada handler na cadeia tem uma responsabilidade especifica e pode
// encaminhar o pagamento para o proximo handler ou interromper o fluxo.
//
// Cadeia: Validation -> RiskCheck -> Approval -> Notification
package services

import (
	"github.com/carloshomar/vercardapio/payment/models"
)

// Handler define a interface para os handlers da cadeia.
// Cada handler pode processar o pagamento e chamar o proximo.
type Handler interface {
	Handle(payment *models.Payment) error // Processa o pagamento
	SetNext(handler Handler) Handler     // Define o proximo handler
}

// BaseHandler e o handler base que implementa a logica comum.
// Contem uma referencia para o proximo handler na cadeia.
type BaseHandler struct {
	next Handler
}

// SetNext define o proximo handler na cadeia e retorna ele
// para permitir encadeamento fluido: a.SetNext(b).SetNext(c)
func (h *BaseHandler) SetNext(handler Handler) Handler {
	h.next = handler
	return handler
}

// passToNext encaminha o pagamento para o proximo handler.
// Se nao houver proximo, retorna nil (fim da cadeia).
func (h *BaseHandler) passToNext(payment *models.Payment) error {
	if h.next != nil {
		return h.next.Handle(payment)
	}
	return nil
}

// ValidationHandler e o primeiro handler da cadeia.
// Valida se os campos obrigatorios do pagamento estao presentes.
// Se a validacao falhar, o pagamento e rejeitado silenciosamente.
type ValidationHandler struct {
	BaseHandler
}

// Handle valida os campos obrigatorios: OrderID, Amount e CustomerID.
func (vh *ValidationHandler) Handle(payment *models.Payment) error {
	// Se nao tem OrderID, ignora (pagamento invalido)
	if payment.OrderID == "" {
		return nil
	}
	// Se o valor e zero ou negativo, ignora
	if payment.Amount <= 0 {
		return nil
	}
	// Se nao tem CustomerID, ignora
	if payment.CustomerID == "" {
		return nil
	}
	// Validacao OK: passa para o proximo handler
	return vh.passToNext(payment)
}

// RiskCheckHandler calcula o score de risco do pagamento.
// Atualiza os campos de risco e encaminha para o proximo handler.
type RiskCheckHandler struct {
	BaseHandler
	Scorer *RiskScorer // Calculador de risco
}

// Handle calcula o score de risco e atualiza o pagamento.
func (rh *RiskCheckHandler) Handle(payment *models.Payment) error {
	// Calcula o score de risco
	assessment := rh.Scorer.AssessPayment(payment)

	// Atualiza os campos de risco no pagamento
	payment.RiskScore = assessment.Score
	payment.RiskLevel = assessment.Level
	payment.RequiresApproval = assessment.RequiresApproval

	// Passa para o proximo handler
	return rh.passToNext(payment)
}

// ApprovalHandler decide se o pagamento sera aprovado ou ficara pendente.
// Se RequiresApproval for true, fica pendente para aprovacao manual.
// Se for false, e aprovado automaticamente.
type ApprovalHandler struct {
	BaseHandler
}

// Handle decide o status do pagamento baseado no nivel de risco.
func (ah *ApprovalHandler) Handle(payment *models.Payment) error {
	// Se precisa de aprovacao, fica pendente
	if payment.RequiresApproval {
		payment.Status = models.PaymentPending
		return nil
	}
	// Se nao precisa, aprovado automaticamente
	payment.Status = models.PaymentApproved
	return ah.passToNext(payment)
}

// NotificationHandler e o ultimo handler da cadeia.
// Atualmente e um placeholder para futuras notificacoes.
// Pode ser extendido para enviar emails, push notifications, etc.
type NotificationHandler struct {
	BaseHandler
}

// Handle passa para o proximo handler (placeholder para notificacoes).
func (nh *NotificationHandler) Handle(payment *models.Payment) error {
	// TODO: Implementar notificacoes (email, push, etc)
	return nh.passToNext(payment)
}

// BuildChain constroi e retorna a cadeia completa de handlers.
// Ordem: Validation -> RiskCheck -> Approval -> Notification
func BuildChain() Handler {
	validation := &ValidationHandler{}
	riskCheck := &RiskCheckHandler{Scorer: NewRiskScorer()}
	approval := &ApprovalHandler{}
	notification := &NotificationHandler{}

	// Conecta os handlers em sequencia
	validation.SetNext(riskCheck).SetNext(approval).SetNext(notification)

	return validation
}
