package repository

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
)

// AirlineRepository defines the interface for airline operations
type AirlineRepository interface {
	GetByCode(ctx context.Context, code string) (*entity.Airline, error)
}
