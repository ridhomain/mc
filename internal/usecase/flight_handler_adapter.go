package usecase

import (
	"context"
	"strings"

	"mailcast-service-v2/internal/domain/entity"
)

// FlightHandlerV1Adapter adapts FlightProcessor V1 to TemplateHandler interface
type FlightHandlerV1Adapter struct {
	processor interface {
		ProcessFlightMessage(ctx context.Context, body string, emailID string) error
	}
	name     string
	patterns []string
}

// NewFlightHandlerV1Adapter creates a new V1 adapter
func NewFlightHandlerV1Adapter(processor interface {
	ProcessFlightMessage(ctx context.Context, body string, emailID string) error
}, name string, patterns []string) *FlightHandlerV1Adapter {
	return &FlightHandlerV1Adapter{
		processor: processor,
		name:      name,
		patterns:  patterns,
	}
}

// CanHandle checks if this handler can process the email
func (a *FlightHandlerV1Adapter) CanHandle(subject string) bool {
	for _, pattern := range a.patterns {
		if strings.Contains(strings.ToLower(subject), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// Process processes the email using plain text body
func (a *FlightHandlerV1Adapter) Process(ctx context.Context, email *entity.Email) error {
	body := email.Body
	if body == "" {
		body = email.HTMLBody
	}

	return a.processor.ProcessFlightMessage(ctx, body, email.EmailID)
}
