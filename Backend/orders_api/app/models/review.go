package models

import "time"

type Review struct {
    ID              uint      `gorm:"primaryKey"`
    OrderID         string    `gorm:"type:varchar(100);not null;uniqueIndex"`
    EstablishmentID uint      `gorm:"not null;index"`
    UserPhone       string    `gorm:"type:varchar(20);not null;index"`
    UserName        string    `gorm:"type:varchar(100)"`
    ProductID       uint      `gorm:"index"`
    Rating          int       `gorm:"not null;check:rating >= 1 AND rating <= 5"`
    Comment         string    `gorm:"type:text"`
    ImageURL        string    `gorm:"type:varchar(500)"`
    ResponseText    string    `gorm:"type:text"`
    ResponseAt      *time.Time `gorm:"default:null"`
    IsAnonymous     bool      `gorm:"default:false"`
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
