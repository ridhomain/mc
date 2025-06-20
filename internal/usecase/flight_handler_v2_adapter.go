package usecase

import (
	"context"
	"strings"

	"mailcast-service-v2/internal/domain/entity"
)

// FlightHandlerV2Adapter adapts FlightProcessor V2 to TemplateHandler interface
type FlightHandlerV2Adapter struct {
	processor interface {
		ProcessFlightMessage(ctx context.Context, body string, emailID string) error
	}
	name     string
	patterns []string
}

// NewFlightHandlerV2Adapter creates a new V2 adapter
func NewFlightHandlerV2Adapter(processor interface {
	ProcessFlightMessage(ctx context.Context, body string, emailID string) error
}, name string, patterns []string) *FlightHandlerV2Adapter {
	return &FlightHandlerV2Adapter{
		processor: processor,
		name:      name,
		patterns:  patterns,
	}
}

// CanHandle checks if this handler can process the email
func (a *FlightHandlerV2Adapter) CanHandle(subject string) bool {
	for _, pattern := range a.patterns {
		if strings.Contains(strings.ToLower(subject), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// Process processes the email using HTML body
func (a *FlightHandlerV2Adapter) Process(ctx context.Context, email *entity.Email) error {
	// V2 uses HTML body (has cleanHTMLText method)
	body := email.HTMLBody
	if body == "" {
		// Fallback to plain text if no HTML available
		body = email.Body
	}
	return a.processor.ProcessFlightMessage(ctx, body, email.EmailID)
}
