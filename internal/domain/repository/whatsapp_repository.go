package repository

import (
	"context"
	"mailcast-service-v2/internal/domain/entity"
	"time"
)

// WhatsappRepository defines the interface for WhatsApp operations
type WhatsappRepository interface {
	SendPayload(ctx context.Context, payload *entity.Payload) (string, error)
	RescheduleTask(ctx context.Context, taskID string, newScheduleTime time.Time, reason string) error
}
