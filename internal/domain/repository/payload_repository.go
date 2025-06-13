package repository

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
)

// PayloadRepository defines the interface for payload storage operations
type PayloadRepository interface {
	Save(ctx context.Context, payload *entity.Payload) error
	FindByID(ctx context.Context, id string) (*entity.Payload, error)
	FindByStatus(ctx context.Context, status string, limit int) ([]*entity.Payload, error)
	UpdateStatus(ctx context.Context, id, status string) error
}