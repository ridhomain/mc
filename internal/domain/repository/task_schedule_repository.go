package repository

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
)

// TaskScheduleRepository defines the interface for task schedule operations
type TaskScheduleRepository interface {
	Create(ctx context.Context, taskSchedule *entity.TaskSchedule) error
	GetTaskByProviderPnr(ctx context.Context, providerPnr string) ([]*entity.TaskSchedule, error)
	GetTaskByProviderPnrAndPsgNameV1(ctx context.Context, providerPnr string, psgName string) ([]*entity.TaskSchedule, error)
	GetTaskByProviderPnrAndPsgName(ctx context.Context, providerPnr string, psgName string, segNo int) ([]*entity.TaskSchedule, error)
}
