package repository

import (
	"context"
	"time"

	"mailcast-service-v2/internal/domain/entity"
)

// EmailRepository defines the interface for email storage operations
type EmailRepository interface {
	Save(ctx context.Context, email *entity.Email) error
	FindByID(ctx context.Context, id string) (*entity.Email, error)
	FindUnprocessed(ctx context.Context, limit int) ([]*entity.Email, error)
	MarkAsProcessed(ctx context.Context, id, status, processorType, errorDetail string, extractedData map[string]interface{}) error
	GetLastEmail(ctx context.Context) (*entity.Email, error)
	UpdateStatus(ctx context.Context, id string, status string, startedAt time.Time) error
	UpdateProcessSteps(ctx context.Context, id string, steps entity.ProcessSteps) error
	FindByStatus(ctx context.Context, status string, limit int) ([]*entity.Email, error)
	ResetProcessingEmails(ctx context.Context) error
	FindByEmailID(ctx context.Context, emailID string) (*entity.Email, error)
	FindByEmailIDs(ctx context.Context, emailIDs []string) (map[string]*entity.Email, error)
	UpdateStatusByEmailID(ctx context.Context, emailID string, status string, startedAt time.Time) error
	MarkAsProcessedByEmailID(ctx context.Context, emailID, status, processorType, errorDetail string, extractedData map[string]interface{}) error
	UpdateProcessStepsByEmailID(ctx context.Context, emailID string, steps entity.ProcessSteps) error
}
