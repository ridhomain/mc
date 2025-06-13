package entity

import "errors"

type SendMailcastMessage struct {
	CompanyID   string  `json:"companyId" binding:"required"`
	AgentID     string  `json:"agentId" binding:"required"`
	PhoneNumber string  `json:"phoneNumber" binding:"required"`
	Message     Message `json:"message" binding:"required"`
	ScheduleAt  string  `json:"scheduleAt" binding:"omitempty,datetime"`
	Type        string  `json:"type" binding:"required,oneof=text image document"`
}

// Message can be one of three types: text, image, or document
type Message struct {
	// Text message type
	Text string `json:"text,omitempty"`

	// Image message type
	Image   interface{} `json:"image,omitempty"`
	Caption string      `json:"caption,omitempty"`

	// Document message type
	Document interface{} `json:"document,omitempty"`
	FileName string      `json:"fileName,omitempty"`
	Mimetype string      `json:"mimetype,omitempty"`
}

// Custom validation for Message struct to enforce anyOf constraint
func (m Message) Validate() error {
	// Check if it's a text message
	if m.Text != "" && m.Image == nil && m.Document == nil {
		return nil
	}

	// Check if it's an image message
	if m.Image != nil && m.Text == "" && m.Document == nil {
		return nil
	}

	// Check if it's a document message
	if m.Document != nil && m.FileName != "" && m.Mimetype != "" && m.Text == "" && m.Image == nil {
		return nil
	}

	return errors.New("message must be either text, image, or document type with required fields")
}

type SendMailcastMessageResponse struct {
	Status     string  `json:"status"`
	ScheduleAt *string `json:"scheduleAt,omitempty"`
}
