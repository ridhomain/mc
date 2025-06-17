package templates

import (
	"context"
	"regexp"
	"strings"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/usecase"
	"mailcast-service-v2/pkg/logger"
)

// FlightNotificationHandler handles flight notification emails
type FlightNotificationHandler struct {
	flightProcessor *usecase.FlightProcessor
	logger          logger.Logger
}

// NewFlightNotificationHandler creates a new flight notification handler
func NewFlightNotificationHandler(flightProcessor *usecase.FlightProcessor, logger logger.Logger) *FlightNotificationHandler {
	return &FlightNotificationHandler{
		flightProcessor: flightProcessor,
		logger:          logger,
	}
}

// CanHandle determines if this handler can process the given email subject
func (h *FlightNotificationHandler) CanHandle(subject string) bool {
	subjectUpper := strings.ToUpper(subject)
	return strings.Contains(subjectUpper, "PREFLIGHT INFO GALILEO")
}

// Process processes the email and returns a payload
func (h *FlightNotificationHandler) Process(ctx context.Context, email *entity.Email) error {
	// Use the email body (prefer HTML if available)
	body := email.HTMLBody
	if body == "" {
		body = email.Body
	}

	// Use the flight processor to handle the complex flight logic
	err := h.flightProcessor.ProcessFlightMessage(ctx, body, email.EmailID)
	if err != nil {
		h.logger.Error("Failed to process flight message", "error", err)
		return err
	}

	return nil
}

// Helper function to extract recipient ID
func extractRecipientID(to string) string {
	re := regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`)
	match := re.FindString(to)
	return match
}
