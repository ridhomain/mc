package entity

import (
	"time"
)

// Email represents an email message from Gmail
type Email struct {
	ID            string
	MessageID     string
	From          string
	To            string
	Subject       string
	Body          string
	HTMLBody      string
	ReceivedAt    time.Time
	Attachments   []Attachment
	Labels        []string
	ProcessedAt   time.Time
	ProcessStatus string                 // "sent", "failed", or "" if not processed
	ProcessorType string                 `bson:"processorType,omitempty"` // "flight", "happybirthday", etc.
	ErrorDetail   string                 `bson:"errorDetail,omitempty"`   // Specific error message if failed
	ExtractedData map[string]interface{} `bson:"extractedData,omitempty"` // Processed data based on processortype
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}
