package repository

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
)

// EmailRepository defines the interface for email storage operations
type EmailRepository interface {
	Save(ctx context.Context, email *entity.Email) error
	FindByID(ctx context.Context, id string) (*entity.Email, error)
	FindByMessageID(ctx context.Context, messageID string) (*entity.Email, error) // Add this method
	FindUnprocessed(ctx context.Context, limit int) ([]*entity.Email, error)
	MarkAsProcessed(ctx context.Context, id, status, processorType, errorDetail string, extractedData map[string]interface{}) error // Updated based on new model
	GetLastEmail(ctx context.Context) (*entity.Email, error)
}
