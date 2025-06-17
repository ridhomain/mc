package templates

import (
	"context"
	"regexp"
	"strings"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/usecase"
	"mailcast-service-v2/pkg/logger"
)

// FlightNotificationHandlerV2 handles flight notification emails
type FlightNotificationHandlerV2 struct {
	flightProcessor *usecase.FlightProcessorV2
	logger          logger.Logger
}

// NewFlightNotificationHandler creates a new flight notification handler
func NewFlightNotificationHandlerV2(flightProcessor *usecase.FlightProcessorV2, logger logger.Logger) *FlightNotificationHandlerV2 {
	return &FlightNotificationHandlerV2{
		flightProcessor: flightProcessor,
		logger:          logger,
	}
}

// CanHandle determines if this handler can process the given email subject
func (h *FlightNotificationHandlerV2) CanHandle(subject string) bool {
	return strings.Contains(subject, "PNR")
}

// Process processes the email and returns a payload
func (h *FlightNotificationHandlerV2) Process(ctx context.Context, email *entity.Email) error {
	body := email.HTMLBody

	h.logger.Info("Processing flight notification email", body)
	// Use the flight processor to handle the complex flight logic
	err := h.flightProcessor.ProcessFlightMessage(ctx, body, email.ID)
	if err != nil {
		h.logger.Error("Failed to process flight message", "error", err)
		return err
	}

	return nil
}

// Helper function to extract recipient ID
func extractRecipientIDV2(to string) string {
	re := regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`)
	match := re.FindString(to)
	return match
}
