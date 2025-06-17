package entity

import (
	"time"
)

// Email Process Status
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
	StatusSkipped    = "SKIPPED"
)

// Email represents an email message from Gmail
type Email struct {
	ID               string
	EmailID          string
	From             string
	To               string
	Subject          string
	Body             string
	HTMLBody         string
	ReceivedAt       time.Time
	Attachments      []Attachment
	Labels           []string
	ProcessedAt      time.Time
	ProcessStatus    string                 // "PENDING", "PROCESSING", "COMPLETED", "FAILED", "SKIPPED"
	ProcessorType    string                 // "flight", "happybirthday", etc.
	ProcessStartedAt time.Time              // When processing started
	ProcessSteps     ProcessSteps           // Track what's been done
	ErrorDetail      string                 // Specific error message if failed
	ExtractedData    map[string]interface{} // Processed data based on processortype
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type ProcessSteps struct {
	PhonesExtracted bool
	SchedulesParsed bool
	MessagesQueued  int // How many messages queued to WhatsApp Service
	TotalMessages   int // Total messages to send
}
