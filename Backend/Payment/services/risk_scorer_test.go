// Package services - risk_scorer_test.go
// Testes unitarios do motor de risco.
// Testam a logica pura de calculo de nivel e normalizacao de score.
// checkFrequency e checkEstablishmentHistory dependem de MongoDB
// e sao testadas apenas indiretamente via integration tests.
package services

import (
	"testing"

	"github.com/carloshomar/vercardapio/payment/models"
)

// === Testes de calculateLevel ===

func TestCalculateLevel(t *testing.T) {
	scorer := NewRiskScorer()

	tests := []struct {
		name     string
		score    float64
		expected models.RiskLevel
	}{
		{"zero score is low", 0, models.RiskLow},
		{"score 19 is low", 19, models.RiskLow},
		{"score 20 is medium", 20, models.RiskMedium},
		{"score 39 is medium", 39, models.RiskMedium},
		{"score 40 is high", 40, models.RiskHigh},
		{"score 59 is high", 59, models.RiskHigh},
		{"score 60 is critical", 60, models.RiskCritical},
		{"score 100 is critical", 100, models.RiskCritical},
		{"score 25 is medium", 25, models.RiskMedium},
		{"score 45 is high", 45, models.RiskHigh},
		{"score 65 is critical", 65, models.RiskCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scorer.calculateLevel(tt.score)
			if result != tt.expected {
				t.Errorf("calculateLevel(%f) = %q, want %q", tt.score, result, tt.expected)
			}
		})
	}
}

// === Testes de NormalizeScore ===

func TestNormalizeScore(t *testing.T) {
	scorer := NewRiskScorer()

	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"normal score unchanged", 50, 50},
		{"negative becomes zero", -10, 0},
		{"over 100 becomes 100", 150, 100},
		{"zero stays zero", 0, 0},
		{"100 stays 100", 100, 100},
		{"negative large becomes zero", -999, 0},
		{"positive large becomes 100", 999, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scorer.NormalizeScore(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeScore(%f) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

// === Testes de checkAmount (logica pura, sem MongoDB) ===

func TestCheckAmount(t *testing.T) {
	scorer := NewRiskScorer()

	tests := []struct {
		name     string
		amount   float64
		expected float64
	}{
		{"small amount no risk", 50.0, 0},
		{"exactly 100 no risk", 100.0, 0},
		{"101 medium risk", 101.0, 10},
		{"200 medium risk", 200.0, 10},
		{"201 high risk", 201.0, 20},
		{"500 high risk", 500.0, 20},
		{"501 very high risk", 501.0, 30},
		{"1000 very high risk", 1000.0, 30},
		{"zero amount", 0.0, 0},
		{"99.99 no risk", 99.99, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scorer.checkAmount(tt.amount)
			if result != tt.expected {
				t.Errorf("checkAmount(%f) = %f, want %f", tt.amount, result, tt.expected)
			}
		})
	}
}

// === Testes de checkTimeOfDay ===

func TestCheckTimeOfDay(t *testing.T) {
	scorer := NewRiskScorer()

	// O teste so verifica que o metodo retorna 0 ou 15
	// (nao podemos controlar time.Now() sem injecao de dependencia)
	result := scorer.checkTimeOfDay()
	if result != 0 && result != 15 {
		t.Errorf("checkTimeOfDay() = %f, want 0 or 15", result)
	}
}

// === Testes de RiskAssessment ===

func TestRiskAssessment_RequiresApproval(t *testing.T) {
	tests := []struct {
		name              string
		level             models.RiskLevel
		requiresApproval  bool
	}{
		{"low does not require approval", models.RiskLow, false},
		{"medium does not require approval", models.RiskMedium, false},
		{"high requires approval", models.RiskHigh, true},
		{"critical requires approval", models.RiskCritical, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simula a logica do ApprovalEngine
			requiresApproval := tt.level == models.RiskHigh || tt.level == models.RiskCritical
			if requiresApproval != tt.requiresApproval {
				t.Errorf("level=%q: requiresApproval=%v, want %v",
					tt.level, requiresApproval, tt.requiresApproval)
			}
		})
	}
}

// === Testes de PaymentMethod ===

func TestPaymentMethods(t *testing.T) {
	methods := map[models.PaymentMethod]string{
		models.PaymentMethodPix:  "pix",
		models.PaymentMethodCard: "card",
	}

	for method, expected := range methods {
		if string(method) != expected {
			t.Errorf("PaymentMethod %v: got %q, want %q", method, string(method), expected)
		}
	}
}

// === Testes de PaymentFilter ===

func TestPaymentFilter_Defaults(t *testing.T) {
	filter := models.PaymentFilter{}

	// Page e Limit default sao zero (serao tratados pelo repository)
	if filter.Page != 0 {
		t.Errorf("Default page: got %d, want 0", filter.Page)
	}
	if filter.Limit != 0 {
		t.Errorf("Default limit: got %d, want 0", filter.Limit)
	}
}

func TestPaymentFilter_WithValues(t *testing.T) {
	filter := models.PaymentFilter{
		Status:          "pending",
		RiskLevel:       "high",
		EstablishmentID: "rest_001",
		Page:            2,
		Limit:           10,
	}

	if filter.Status != "pending" {
		t.Errorf("Status: got %q, want %q", filter.Status, "pending")
	}
	if filter.Page != 2 {
		t.Errorf("Page: got %d, want 2", filter.Page)
	}
}
