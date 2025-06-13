package repository

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
)

// TimezoneRepository defines the interface for timezone operations
type TimezoneRepository interface {
	GetByAirportCode(ctx context.Context, code string) (*entity.Timezone, error)
	GetTimezoneByCode(ctx context.Context, code string) (*entity.Timezone, error)
}
