package models

type BusinessHours struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	EstablishmentID uint   `gorm:"uniqueIndex:idx_est_day;not null" json:"establishment_id"`
	DayOfWeek       int    `gorm:"uniqueIndex:idx_est_day;not null" json:"day_of_week"`
	IsOpen          bool   `gorm:"default:true" json:"is_open"`
	OpenTime        string `gorm:"type:varchar(5)" json:"open_time"`
	CloseTime       string `gorm:"type:varchar(5)" json:"close_time"`
	BreakStartTime  string `gorm:"type:varchar(5)" json:"break_start_time"`
	BreakEndTime    string `gorm:"type:varchar(5)" json:"break_end_time"`
}

func (BusinessHours) TableName() string {
	return "business_hours"
}

func parseTimeToMinutes(timeStr string) int {
	if len(timeStr) < 5 {
		return 0
	}
	hours := int(timeStr[0]-'0')*10 + int(timeStr[1]-'0')
	minutes := int(timeStr[3]-'0')*10 + int(timeStr[4]-'0')
	return hours*60 + minutes
}
