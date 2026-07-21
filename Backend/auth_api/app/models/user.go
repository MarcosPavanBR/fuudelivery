package models

type User struct {
	ID              uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"-"`
	EstablishmentID uint   `json:"establishment_id"`
	Role            string `gorm:"column:role;default:user" json:"role"`
}
