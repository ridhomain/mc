package entity

import (
	"time"

	"gorm.io/gorm"
)

// Airline represents an airline entity
type Airline struct {
	ID        uint
	Code      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}
