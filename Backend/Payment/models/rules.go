package models

import "time"

type ApprovalRules struct {
	ID                       string    `bson:"_id,omitempty" json:"id"`
	AutoApproveMaxAmount     float64   `bson:"auto_approve_max_amount" json:"auto_approve_max_amount"`
	AutoApproveMaxRisk       float64   `bson:"auto_approve_max_risk" json:"auto_approve_max_risk"`
	ManualReviewMinAmount    float64   `bson:"manual_review_min_amount" json:"manual_review_min_amount"`
	ManualReviewMinRisk      float64   `bson:"manual_review_min_risk" json:"manual_review_min_risk"`
	ComplianceMinRisk        float64   `bson:"compliance_min_risk" json:"compliance_min_risk"`
	BlockChargebackActive    bool      `bson:"block_chargeback_active" json:"block_chargeback_active"`
	BlockMaxDailyWithdrawals int       `bson:"block_max_daily_withdrawals" json:"block_max_daily_withdrawals"`
	UpdatedAt                time.Time `bson:"updated_at" json:"updated_at"`
}

func DefaultApprovalRules() ApprovalRules {
	return ApprovalRules{
		AutoApproveMaxAmount:     1000,
		AutoApproveMaxRisk:       20,
		ManualReviewMinAmount:    5000,
		ManualReviewMinRisk:      60,
		ComplianceMinRisk:        80,
		BlockChargebackActive:    true,
		BlockMaxDailyWithdrawals: 3,
	}
}
