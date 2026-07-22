package services

import (
	"math"
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/repository"
)

type RiskScorer struct{}

func NewRiskScorer() *RiskScorer {
	return &RiskScorer{}
}

type RiskAssessment struct {
	Score      float64           `json:"score"`
	Level      models.RiskLevel  `json:"level"`
	RequiresApproval bool        `json:"requires_approval"`
	Reasons    []string          `json:"reasons"`
}

func (r *RiskScorer) AssessPayment(payment *models.Payment) *RiskAssessment {
	score := 0.0
	reasons := []string{}

	score += r.checkAmount(payment.Amount)
	score += r.checkFrequency(payment.CustomerID)
	score += r.checkTimeOfDay()
	score += r.checkEstablishmentHistory(payment.EstablishmentID)

	level := r.calculateLevel(score)
	requiresApproval := level == models.RiskHigh || level == models.RiskCritical

	return &RiskAssessment{
		Score:           score,
		Level:           level,
		RequiresApproval: requiresApproval,
		Reasons:         reasons,
	}
}

func (r *RiskScorer) checkAmount(amount float64) float64 {
	if amount > 500 {
		return 30
	}
	if amount > 200 {
		return 20
	}
	if amount > 100 {
		return 10
	}
	return 0
}

func (r *RiskScorer) checkFrequency(customerID string) float64 {
	ctx := repository.MongoCtx()
	count, _ := repository.Payments.CountDocuments(ctx, map[string]interface{}{
		"customer_id": customerID,
		"created_at": map[string]interface{}{
			"$gte": time.Now().Add(-24 * time.Hour),
		},
	})

	if count > 10 {
		return 25
	}
	if count > 5 {
		return 15
	}
	return 0
}

func (r *RiskScorer) checkTimeOfDay() float64 {
	hour := time.Now().Hour()
	if hour >= 1 && hour <= 5 {
		return 15
	}
	return 0
}

func (r *RiskScorer) checkEstablishmentHistory(establishmentID string) float64 {
	ctx := repository.MongoCtx()
	chargebacks, _ := repository.Chargebacks.CountDocuments(ctx, map[string]interface{}{
		"establishment_id": establishmentID,
		"status":           "approved",
	})

	if chargebacks > 5 {
		return 20
	}
	if chargebacks > 2 {
		return 10
	}
	return 0
}

func (r *RiskScorer) calculateLevel(score float64) models.RiskLevel {
	if score >= 60 {
		return models.RiskCritical
	}
	if score >= 40 {
		return models.RiskHigh
	}
	if score >= 20 {
		return models.RiskMedium
	}
	return models.RiskLow
}

func (r *RiskScorer) NormalizeScore(score float64) float64 {
	return math.Max(0, math.Min(100, score))
}
