package repository

import (
	"context"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"

	"gorm.io/gorm"
)

// GormTimezoneRepository implements the TimezoneRepository interface
type GormTimezoneRepository struct {
	db *gorm.DB
}

// NewGormTimezoneRepository creates a new GORM timezone repository
func NewGormTimezoneRepository(db *gorm.DB) repository.TimezoneRepository {
	return &GormTimezoneRepository{
		db: db,
	}
}

// Timezonelist GORM model for database mapping
type Timezonelist struct {
	gorm.Model
	ID          uint           `gorm:"primaryKey"`
	AirportCode string         `gorm:"column:airportcode;unique"`
	AirportName string         `gorm:"column:airport_name"`
	CityCode    string         `gorm:"column:citycode"`
	CityName    string         `gorm:"column:cityname"`
	GmtTz       string         `gorm:"column:gmttz"`
	TzName      string         `gorm:"column:tzname"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName overrides the default table name
func (Timezonelist) TableName() string {
	return "m_timezone_list"
}

// GetByAirportCode finds a timezone by airport code
func (r *GormTimezoneRepository) GetByAirportCode(ctx context.Context, code string) (*entity.Timezone, error) {
	var timezone Timezonelist
	result := r.db.WithContext(ctx).Unscoped().Where("airportcode = ?", code).First(&timezone)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert GORM model to domain entity
	return &entity.Timezone{
		ID:          timezone.ID,
		AirportCode: timezone.AirportCode,
		AirportName: timezone.AirportName,
		CityCode:    timezone.CityCode,
		CityName:    timezone.CityName,
		GmtTz:       timezone.GmtTz,
		TzName:      timezone.TzName,
		CreatedAt:   timezone.CreatedAt,
		UpdatedAt:   timezone.UpdatedAt,
		DeletedAt:   timezone.DeletedAt,
	}, nil
}

// GetTimezoneByCode retrieves a timezone by its code
func (r *GormTimezoneRepository) GetTimezoneByCode(ctx context.Context, code string) (*entity.Timezone, error) {
	var timezone Timezonelist
	result := r.db.WithContext(ctx).Unscoped().Where("airportcode = ?", code).First(&timezone)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert GORM model to domain entity
	return &entity.Timezone{
		ID:          timezone.ID,
		AirportCode: timezone.AirportCode,
		AirportName: timezone.AirportName,
		CityCode:    timezone.CityCode,
		CityName:    timezone.CityName,
		GmtTz:       timezone.GmtTz,
		TzName:      timezone.TzName,
		CreatedAt:   timezone.CreatedAt,
		UpdatedAt:   timezone.UpdatedAt,
		DeletedAt:   timezone.DeletedAt,
	}, nil
}
