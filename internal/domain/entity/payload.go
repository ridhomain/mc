package entity

import (
	"time"
)

// PayloadType defines the type of the payload
type PayloadType string

const (
	FlightNotification PayloadType = "flight_notification"
	HappyBirthday      PayloadType = "happy_birthday"
)

// Payload represents the structured data extracted from an email
type Payload struct {
	ID         string                 `json:"id,omitempty"`
	Type       PayloadType            `json:"type"`
	Phone      string                 `json:"phone"`
	Text       string                 `json:"text"`
	Image      string                 `json:"image,omitempty"`
	ScheduleAt time.Time              `json:"scheduleAt"`
	CreatedAt  time.Time              `json:"createdAt"`
	SentAt     time.Time              `json:"sentAt,omitempty"`
	Status     string                 `json:"status"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
