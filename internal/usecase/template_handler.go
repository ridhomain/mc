package usecase

import (
	"context"

	"mailcast-service-v2/internal/domain/entity"
)

// TemplateHandler defines the interface for email template handlers
type TemplateHandler interface {
	// CanHandle determines if this handler can process the given email subject
	CanHandle(subject string) bool

	// Process processes the email and returns a payload
	Process(ctx context.Context, email *entity.Email) error
}

// SubjectRouter routes emails to the appropriate handler based on subject
type SubjectRouter interface {
	// Register registers a handler for specific subject patterns
	Register(handler TemplateHandler)

	// GetHandler returns the appropriate handler for a given subject
	GetHandler(subject string) TemplateHandler
}
