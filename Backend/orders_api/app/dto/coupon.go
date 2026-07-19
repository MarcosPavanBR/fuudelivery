package dto

type CreateCouponRequest struct {
	Code            string  `json:"code"`
	Description     string  `json:"description"`
	DiscountType    string  `json:"discount_type"`
	DiscountValue   float64 `json:"discount_value"`
	MinOrderValue   float64 `json:"min_order_value"`
	MaxUses         int     `json:"max_uses"`
	MaxUsesPerUser  int     `json:"max_uses_per_user"`
	StartDate       string  `json:"start_date"`
	ExpiryDate      string  `json:"expiry_date"`
	EstablishmentID uint    `json:"establishment_id"`
}

type ValidateCouponRequest struct {
	Code            string  `json:"code"`
	UserPhone       string  `json:"user_phone"`
	OrderValue      float64 `json:"order_value"`
	EstablishmentID uint    `json:"establishment_id"`
}

type ValidateCouponResponse struct {
	Valid          bool    `json:"valid"`
	DiscountType   string  `json:"discount_type,omitempty"`
	DiscountValue  float64 `json:"discount_value,omitempty"`
	DiscountAmount float64 `json:"discount_amount,omitempty"`
	FinalValue     float64 `json:"final_value,omitempty"`
	Message        string  `json:"message,omitempty"`
}

type ApplyCouponRequest struct {
	Code      string `json:"code"`
	UserPhone string `json:"user_phone"`
	OrderID   string `json:"order_id"`
}

type ReferralCouponRequest struct {
	ReferrerPhone string `json:"referrer_phone"`
	NewUserPhone  string `json:"new_user_phone"`
}
