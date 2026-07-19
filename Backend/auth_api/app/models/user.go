package models

type User struct {
	ID              uint   `gorm:"primaryKey;default:(-)" json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"-"`
	EstablishmentID uint   `json:"establishment_id"`
}
