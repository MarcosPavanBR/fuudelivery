package models

import "time"

// Planos de assinatura disponiveis
const (
	PlanBasic   = "basic"   // R$ 19,90/mês: frete grátis acima de R$ 30
	PlanPremium = "premium" // R$ 34,90/mês: frete grátis + cashback 5%
)

// Status da assinatura
const (
	SubscriptionActive    = "active"
	SubscriptionCancelled = "cancelled"
	SubscriptionExpired   = "expired"
)

// Subscription representa a assinatura de um cliente (clube de frete grátis).
// Gera receita recorrente previsível para a plataforma.
type Subscription struct {
	ID     uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID uint   `gorm:"not null;uniqueIndex:idx_subscription_user" json:"user_id"`
	Plan   string `gorm:"size:20;not null;default:'basic'" json:"plan"`

	// Status: active, cancelled, expired
	Status string `gorm:"size:20;not null;default:'active'" json:"status"`

	// Valor mensal da assinatura (R$)
	Amount float64 `gorm:"not null;default:19.90" json:"amount"`

	// Beneficios
	// Frete grátis para pedidos acima deste valor (0 = sem frete grátis)
	FreeDeliveryAbove float64 `gorm:"default:30.0" json:"free_delivery_above"`
	// Percentual de cashback (ex: 5.0 = 5%)
	CashbackPct float64 `gorm:"default:0" json:"cashback_pct"`

	// Ciclo de faturamento atual
	CurrentPeriodStart time.Time  `json:"current_period_start"`
	CurrentPeriodEnd   time.Time  `json:"current_period_end"`
	CancelledAt        *time.Time `json:"cancelled_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Subscription) TableName() string {
	return "subscriptions"
}

// IsActive retorna true se a assinatura está ativa e dentro do período vigente.
func (s *Subscription) IsActive() bool {
	if s.Status != SubscriptionActive {
		return false
	}
	now := time.Now()
	return now.After(s.CurrentPeriodStart) && now.Before(s.CurrentPeriodEnd)
}

// GetPlanAmount retorna o valor do plano escolhido.
func GetPlanAmount(plan string) float64 {
	switch plan {
	case PlanBasic:
		return 19.90
	case PlanPremium:
		return 34.90
	default:
		return 19.90
	}
}

// GetPlanBenefits retorna os beneficios de cada plano.
func GetPlanBenefits(plan string) (freeDeliveryAbove, cashbackPct float64) {
	switch plan {
	case PlanBasic:
		return 30.0, 0
	case PlanPremium:
		return 0, 5.0 // frete grátis SEM valor mínimo + 5% cashback
	default:
		return 30.0, 0
	}
}

// CalculateDeliveryFee calcula se o cliente tem direito a frete grátis
// baseado no valor do pedido e no plano.
// Retorna a taxa de entrega efetiva (0 = grátis).
func (s *Subscription) CalculateDeliveryFee(orderTotal, deliveryFee float64) float64 {
	if !s.IsActive() {
		return deliveryFee
	}

	// Premium: frete grátis sempre
	if s.Plan == PlanPremium {
		return 0
	}

	// Basic: frete grátis acima do valor mínimo
	if s.FreeDeliveryAbove > 0 && orderTotal >= s.FreeDeliveryAbove {
		return 0
	}

	return deliveryFee
}
