package repository

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"

	"gorm.io/gorm"
)

// GormTaskScheduleRepository implements the TaskScheduleRepository interface
type GormTaskScheduleRepository struct {
	db *gorm.DB
}

// NewGormTaskScheduleRepository creates a new GORM task schedule repository
func NewGormTaskScheduleRepository(db *gorm.DB) repository.TaskScheduleRepository {
	return &GormTaskScheduleRepository{
		db: db,
	}
}

// TaskSchedules GORM model for database mapping
type TaskSchedules struct {
	gorm.Model
	IdAsynq     string `gorm:"column:id_asynq"`
	Type        string `gorm:"column:type"`
	QueueName   string `gorm:"column:queue_name"`
	Payload     string `gorm:"column:payload"`
	ProviderPnr string `gorm:"column:provider_pnr"`
	PsgName     string `gorm:"column:psg_name"`
	PsgPhone    string `gorm:"column:psg_phone"`
	SegNo       int    `gorm:"column:segment_no"`
	Status      string `gorm:"column:status"`
}

// TableName overrides the default table name
func (TaskSchedules) TableName() string {
	return "task_schedules"
}

// Create inserts a new task_schedule into the database
func (r *GormTaskScheduleRepository) Create(ctx context.Context, taskSchedule *entity.TaskSchedule) error {
	model := TaskSchedules{
		IdAsynq:     taskSchedule.IdAsynq,
		Type:        taskSchedule.Type,
		QueueName:   taskSchedule.QueueName,
		Payload:     taskSchedule.Payload,
		ProviderPnr: taskSchedule.ProviderPnr,
		PsgName:     taskSchedule.PsgName,
		PsgPhone:    taskSchedule.PsgPhone,
		SegNo:       taskSchedule.SegNo,
		Status:      taskSchedule.Status,
	}

	result := r.db.WithContext(ctx).Create(&model)
	if result.Error != nil {
		return result.Error
	}

	// Update the entity with the generated ID
	taskSchedule.ID = model.ID
	taskSchedule.CreatedAt = model.CreatedAt
	taskSchedule.UpdatedAt = model.UpdatedAt

	return nil
}

// GetTaskByProviderPnr finds task schedules by provider PNR
func (r *GormTaskScheduleRepository) GetTaskByProviderPnr(ctx context.Context, providerPnr string) ([]*entity.TaskSchedule, error) {
	var tasks []TaskSchedules
	result := r.db.WithContext(ctx).Unscoped().Where("provider_pnr = ?", providerPnr).Find(&tasks)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert to domain entities
	var entities []*entity.TaskSchedule
	for _, task := range tasks {
		entities = append(entities, &entity.TaskSchedule{
			ID:          task.ID,
			IdAsynq:     task.IdAsynq,
			Type:        task.Type,
			QueueName:   task.QueueName,
			Payload:     task.Payload,
			ProviderPnr: task.ProviderPnr,
			PsgName:     task.PsgName,
			PsgPhone:    task.PsgPhone,
			SegNo:       task.SegNo,
			Status:      task.Status,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
		})
	}

	return entities, nil
}

// GetTaskByProviderPnrAndPsgNameV1 finds task schedules by provider PNR and passenger name (V1)
func (r *GormTaskScheduleRepository) GetTaskByProviderPnrAndPsgNameV1(ctx context.Context, providerPnr string, psgName string) ([]*entity.TaskSchedule, error) {
	var tasks []TaskSchedules
	result := r.db.WithContext(ctx).Unscoped().
		Where("provider_pnr = ?", providerPnr).
		Where("psg_name LIKE ?", "%"+psgName+"%").
		Find(&tasks)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert to domain entities
	var entities []*entity.TaskSchedule
	for _, task := range tasks {
		entities = append(entities, &entity.TaskSchedule{
			ID:          task.ID,
			IdAsynq:     task.IdAsynq,
			Type:        task.Type,
			QueueName:   task.QueueName,
			Payload:     task.Payload,
			ProviderPnr: task.ProviderPnr,
			PsgName:     task.PsgName,
			PsgPhone:    task.PsgPhone,
			SegNo:       task.SegNo,
			Status:      task.Status,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
		})
	}

	return entities, nil
}

// GetTaskByProviderPnrAndPsgName finds task schedules by provider PNR, passenger name, and segment number
func (r *GormTaskScheduleRepository) GetTaskByProviderPnrAndPsgName(ctx context.Context, providerPnr string, psgName string, segNo int) ([]*entity.TaskSchedule, error) {
	var tasks []TaskSchedules
	result := r.db.WithContext(ctx).Unscoped().
		Where("provider_pnr = ?", providerPnr).
		Where("segment_no = ?", segNo).
		Where("status = ?", "HK").
		Where("psg_name LIKE ?", "%"+psgName+"%").
		Find(&tasks)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert to domain entities
	var entities []*entity.TaskSchedule
	for _, task := range tasks {
		entities = append(entities, &entity.TaskSchedule{
			ID:          task.ID,
			IdAsynq:     task.IdAsynq,
			Type:        task.Type,
			QueueName:   task.QueueName,
			Payload:     task.Payload,
			ProviderPnr: task.ProviderPnr,
			PsgName:     task.PsgName,
			PsgPhone:    task.PsgPhone,
			SegNo:       task.SegNo,
			Status:      task.Status,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
		})
	}
	return entities, nil
}
