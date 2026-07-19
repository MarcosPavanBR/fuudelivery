package models

import "time"

type BusinessHours struct {
	ID              uint   `gorm:"primaryKey"`
	EstablishmentID uint   `gorm:"uniqueIndex:idx_est_day;not null"`
	DayOfWeek       int    `gorm:"uniqueIndex:idx_est_day;not null"`
	IsOpen          bool   `gorm:"default:true"`
	OpenTime        string `gorm:"type:varchar(5)"`
	CloseTime       string `gorm:"type:varchar(5)"`
	BreakStartTime  string `gorm:"type:varchar(5)"`
	BreakEndTime    string `gorm:"type:varchar(5)"`
}

func (BusinessHours) TableName() string {
	return "business_hours"
}

func IsEstablishmentOpen(establishmentID uint) (bool, error) {
	now := time.Now()
	currentWeekday := int(now.Weekday())
	currentMinutes := now.Hour()*60 + now.Minute()

	var hours BusinessHours
	if err := DB.Where("establishment_id = ? AND day_of_week = ?", establishmentID, currentWeekday).First(&hours).Error; err != nil {
		return false, err
	}

	if !hours.IsOpen {
		return false, nil
	}

	openMinutes := parseTimeToMinutes(hours.OpenTime)
	closeMinutes := parseTimeToMinutes(hours.CloseTime)

	if currentMinutes >= openMinutes && currentMinutes <= closeMinutes {
		if hours.BreakStartTime != "" && hours.BreakEndTime != "" {
			breakStart := parseTimeToMinutes(hours.BreakStartTime)
			breakEnd := parseTimeToMinutes(hours.BreakEndTime)
			if currentMinutes >= breakStart && currentMinutes <= breakEnd {
				return false, nil
			}
		}
		return true, nil
	}

	return false, nil
}

func parseTimeToMinutes(timeStr string) int {
	if len(timeStr) < 5 {
		return 0
	}
	hours := int(timeStr[0]-'0')*10 + int(timeStr[1]-'0')
	minutes := int(timeStr[3]-'0')*10 + int(timeStr[4]-'0')
	return hours*60 + minutes
}
