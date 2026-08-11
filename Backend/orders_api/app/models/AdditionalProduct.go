package models

type AdditionalProducts struct {
	ProductID    uint       `gorm:"type:integer;not null"`
	AdditionalID uint       `gorm:"type:integer;not null"`
	Product      Product    `gorm:"foreignKey:ProductID"`
	Additional   Additional `gorm:"foreignKey:AdditionalID"`
}
