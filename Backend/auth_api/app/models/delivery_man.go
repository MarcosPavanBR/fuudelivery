package models

type DeliveryMan struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"`
	Phone    string `json:"phone"`

	PaymentWalletID string `json:"payment_wallet_id,omitempty" gorm:"size:100"`
}
