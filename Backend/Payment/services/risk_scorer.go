// Package services - risk_scorer.go
// Motor de calculo de risco para pagamentos.
// Analisa 4 fatores para atribuir um score de 0 a 100:
// 1. Valor do pagamento (quanto maior, mais arriscado)
// 2. Frequencia do cliente (muitas transacoes em 24h = suspeito)
// 3. Horario (transacoes entre 1h-5h sao mais arriscadas)
// 4. Historico do estabelecimento (muitos chargebacks = arriscado)
//
// O score determina o nivel de risco e se o pagamento precisa de aprovacao manual.
package services

import (
	"math"
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

// RiskScorer e responsavel por calcular o score de risco de um pagamento.
type RiskScorer struct{}

// NewRiskScorer cria uma nova instancia do RiskScorer.
func NewRiskScorer() *RiskScorer {
	return &RiskScorer{}
}

// RiskAssessment representa o resultado da avaliacao de risco.
// Contem o score numerico, nivel qualitativo e se requer aprovacao.
type RiskAssessment struct {
	Score           float64          `json:"score"`            // Score numerico 0-100
	Level           models.RiskLevel `json:"level"`            // Nivel: low, medium, high, critical
	RequiresApproval bool            `json:"requires_approval"` // Se precisa de aprovacao manual
	Reasons         []string         `json:"reasons"`          // Motivos do score (para auditoria)
}

// AssessPayment analisa um pagamento e retorna sua avaliacao de risco.
// Soma os scores de 4 fatores: valor, frequencia, horario e historico.
func (r *RiskScorer) AssessPayment(payment *models.Payment) *RiskAssessment {
	score := 0.0
	reasons := []string{}

	// Fator 1: Valor do pagamento
	// > R$500 = +30, > R$200 = +20, > R$100 = +10
	score += r.checkAmount(payment.Amount)

	// Fator 2: Frequencia do cliente nas ultimas 24h
	// > 10 transacoes = +25, > 5 = +15
	score += r.checkFrequency(payment.CustomerID)

	// Fator 3: Horario da transacao
	// Entre 1h-5h = +15 (horario incomum)
	score += r.checkTimeOfDay()

	// Fator 4: Historico de chargebacks do estabelecimento
	// > 5 aprovados = +20, > 2 = +10
	score += r.checkEstablishmentHistory(payment.EstablishmentID)

	// Calcula nivel baseado no score total
	level := r.calculateLevel(score)

	// Pagamentos com score >= 40 (high ou critical) precisam de aprovacao
	requiresApproval := level == models.RiskHigh || level == models.RiskCritical

	return &RiskAssessment{
		Score:           score,
		Level:           level,
		RequiresApproval: requiresApproval,
		Reasons:         reasons,
	}
}

// checkAmount avalia o risco baseado no valor do pagamento.
// Pagamentos maiores tem maior risco de fraude.
func (r *RiskScorer) checkAmount(amount float64) float64 {
	if amount > 500 {
		return 30 // Risco alto: valor muito alto
	}
	if amount > 200 {
		return 20 // Risco medio: valor alto
	}
	if amount > 100 {
		return 10 // Risco baixo: valor moderado
	}
	return 0 // Risco minimo: valor baixo
}

// checkFrequency avalia o risco baseado na frequencia de transacoes do cliente.
// Muitas transacoes em 24h podem indicar fraude ou uso indevido.
func (r *RiskScorer) checkFrequency(customerID string) float64 {
	ctx := repository.MongoCtx()
	count, _ := repository.Payments.CountDocuments(ctx, map[string]interface{}{
		"customer_id": customerID,
		"created_at": map[string]interface{}{
			"$gte": time.Now().Add(-24 * time.Hour), // Ultimas 24 horas
		},
	})

	if count > 10 {
		return 25 // Muitas transacoes: muito arriscado
	}
	if count > 5 {
		return 15 // Algumas transacoes: moderado
	}
	return 0 // Poucas transacoes: normal
}

// checkTimeOfDay avalia o risco baseado no horario da transacao.
// Transacoes entre 1h e 5h da manha sao mais suspeitas (horario de dormir).
func (r *RiskScorer) checkTimeOfDay() float64 {
	hour := time.Now().Hour()
	if hour >= 1 && hour <= 5 {
		return 15 // Horario suspeito
	}
	return 0 // Horario normal
}

// checkEstablishmentHistory avalia o risco baseado no historico de chargebacks.
// Estabelecimentos com muitos estornos aprovados sao mais arriscados.
func (r *RiskScorer) checkEstablishmentHistory(establishmentID string) float64 {
	ctx := repository.MongoCtx()
	chargebacks, _ := repository.Chargebacks.CountDocuments(ctx, map[string]interface{}{
		"establishment_id": establishmentID,
		"status":           "approved", // Apenas chargebacks aprovados
	})

	if chargebacks > 5 {
		return 20 // Historico ruim: muito arriscado
	}
	if chargebacks > 2 {
		return 10 // Historico moderado
	}
	return 0 // Historico limpo
}

// calculateLevel converte o score numerico em um nivel qualitativo.
// Score >= 60: critical, >= 40: high, >= 20: medium, < 20: low
func (r *RiskScorer) calculateLevel(score float64) models.RiskLevel {
	if score >= 60 {
		return models.RiskCritical // Bloqueado: nao processa
	}
	if score >= 40 {
		return models.RiskHigh // Requer aprovacao manual
	}
	if score >= 20 {
		return models.RiskMedium // Pode precisar de revisao
	}
	return models.RiskLow // Aprovacao automatica
}

// NormalizeScore garante que o score esteja entre 0 e 100.
func (r *RiskScorer) NormalizeScore(score float64) float64 {
	return math.Max(0, math.Min(100, score))
}
