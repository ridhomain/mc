// internal/domain/entity/payload.go
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

// ImagePayload represents the image structure for WhatsApp API
type ImagePayload struct {
	URL string `json:"url" bson:"url"`
}

// Payload represents the structured data extracted from an email
type Payload struct {
	ID         string                 `json:"id,omitempty" bson:"_id,omitempty"`
	Type       PayloadType            `json:"type" bson:"type"`
	Phone      string                 `json:"phone" bson:"phone"`
	Text       string                 `json:"text" bson:"text"`
	Image      *ImagePayload          `json:"image,omitempty" bson:"image,omitempty"`
	ScheduleAt time.Time              `json:"scheduleAt" bson:"scheduleAt"`
	CreatedAt  time.Time              `json:"createdAt" bson:"createdAt"`
	SentAt     time.Time              `json:"sentAt,omitempty" bson:"sentAt,omitempty"`
	Status     string                 `json:"status" bson:"status"`
	Metadata   map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

func (p *Payload) SetImageURL(url string) {
	if url != "" {
		p.Image = &ImagePayload{URL: url}
	}
}
