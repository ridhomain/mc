package entity

import "time"

// TaskSchedule represents a scheduled task in the system
type TaskSchedule struct {
	ID          uint
	IdAsynq     string
	Type        string
	QueueName   string
	Payload     string
	ProviderPnr string
	PsgName     string
	PsgPhone    string
	SegNo       int
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}
