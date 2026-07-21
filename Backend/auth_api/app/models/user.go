package models

type User struct {
	ID              uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"-"`
	EstablishmentID uint   `json:"establishment_id"`
	Role            string `gorm:"type:varchar(20);default:'user'" json:"role"`
}
