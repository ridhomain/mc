package repository

import (
	"context"
	"mailcast-service-v2/internal/domain/entity"
	"time"
)

// FlightRecordRepository defines the interface for flight record operations
type FlightRecordRepository interface {
	FindByBookingKey(ctx context.Context, bookingKey string) (*entity.FlightRecord, error)
	Upsert(ctx context.Context, record *entity.FlightRecord) error
	UpdateTaskInfo(ctx context.Context, bookingKey string, taskID string, scheduledAt time.Time) error
}
