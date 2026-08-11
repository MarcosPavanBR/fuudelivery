package models

type CategoryProducts struct {
	ProductID  uint     `gorm:"type:integer;not null"`
	CategoryID uint     `gorm:"type:integer;not null"`
	Category   Category `gorm:"foreignKey:CategoryID"`
	Product    Product  `gorm:"foreignKey:ProductID"`
}
