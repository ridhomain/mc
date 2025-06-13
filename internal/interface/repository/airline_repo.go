package repository

import (
	"context"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"

	"gorm.io/gorm"
)

// GormAirlineRepository implements the AirlineRepository interface
type GormAirlineRepository struct {
	db *gorm.DB
}

// NewGormAirlineRepository creates a new GORM airline repository
func NewGormAirlineRepository(db *gorm.DB) repository.AirlineRepository {
	return &GormAirlineRepository{
		db: db,
	}
}

// Airlines GORM model for database mapping
type Airlines struct {
	gorm.Model                // ID        uint           `gorm:"primaryKey"`
	ID         uint           `gorm:"primaryKey"`
	Code       string         `gorm:"column:code;unique"`
	Name       string         `gorm:"column:name;unique"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName overrides the default table name
func (Airlines) TableName() string {
	return "m_airlines"
}

// GetByCode finds an airline by code
func (r *GormAirlineRepository) GetByCode(ctx context.Context, code string) (*entity.Airline, error) {
	var airline Airlines
	result := r.db.WithContext(ctx).Unscoped().Where("code = ?", code).First(&airline)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert GORM model to domain entity
	return &entity.Airline{
		ID:        airline.ID,
		Code:      airline.Code,
		Name:      airline.Name,
		CreatedAt: airline.CreatedAt,
		UpdatedAt: airline.UpdatedAt,
		DeletedAt: airline.DeletedAt,
	}, nil
}
