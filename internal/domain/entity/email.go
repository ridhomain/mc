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
	EmailID          string                 `bson:"emailId"`
	From             string                 `bson:"from"`
	To               string                 `bson:"to"`
	Subject          string                 `bson:"subject"`
	Body             string                 `bson:"body"`
	HTMLBody         string                 `bson:"htmlBody"`
	ReceivedAt       time.Time              `bson:"receivedAt"`
	Attachments      []Attachment           `bson:"attachments"`
	Labels           []string               `bson:"labels"`
	ProcessedAt      time.Time              `bson:"processedAt"`
	ProcessStatus    string                 `bson:"processStatus"`
	ProcessorType    string                 `bson:"processorType"`
	ProcessStartedAt time.Time              `bson:"processStartedAt"`
	ProcessSteps     ProcessSteps           `bson:"processSteps"`
	ErrorDetail      string                 `bson:"errorDetail"`
	ExtractedData    map[string]interface{} `bson:"extractedData"`
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string `bson:"filename"`
	ContentType string `bson:"contentType"`
	Data        []byte `bson:"data"`
}

type ProcessSteps struct {
	PhonesExtracted bool `bson:"phonesExtracted"`
	SchedulesParsed bool `bson:"schedulesParsed"`
	MessagesQueued  int  `bson:"messagesQueued"` // How many messages queued to WhatsApp Service
	TotalMessages   int  `bson:"totalMessages"`  // Total messages to send
}
