package models

type User struct {
	ID              uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"-"`
	Phone           string `gorm:"column:phone" json:"phone"`
	AvatarURL       string `gorm:"column:avatar_url" json:"avatar_url"`
	EstablishmentID uint   `json:"establishment_id"`
	Role            string `gorm:"column:role;default:user" json:"role"`
}
