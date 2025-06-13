package entity

import (
	"time"

	"gorm.io/gorm"
)

// Timezone represents timezone information for airports
type Timezone struct {
	ID          uint
	AirportCode string
	AirportName string
	CityCode    string
	CityName    string
	GmtTz       string
	TzName      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}
