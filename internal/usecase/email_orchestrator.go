package usecase

import (
	"context"
	"fmt"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"
	"mailcast-service-v2/pkg/logger"
)

// EmailOrchestrator manages email processing with multiple handlers
type EmailOrchestrator struct {
	emailRepo repository.EmailRepository
	router    SubjectRouter
	logger    logger.Logger
}

// NewEmailOrchestrator creates a new email orchestrator
func NewEmailOrchestrator(
	emailRepo repository.EmailRepository,
	router SubjectRouter,
	logger logger.Logger,
) *EmailOrchestrator {
	return &EmailOrchestrator{
		emailRepo: emailRepo,
		router:    router,
		logger:    logger,
	}
}

// ProcessEmail processes a single email immediately after fetching
func (o *EmailOrchestrator) ProcessEmail(ctx context.Context, email *entity.Email) error {
	// Find appropriate handler based on subject
	handler := o.router.GetHandler(email.Subject)
	if handler == nil {
		o.logger.Debug("No handler found for email",
			"subject", email.Subject,
			"emailID", email.EmailID)

		// Mark as skipped - this is not an error, just no matching template
		return o.emailRepo.MarkAsProcessedByEmailID(
			ctx,
			email.EmailID,
			entity.StatusSkipped,
			"none",
			"No matching handler found",
			map[string]interface{}{
				"subject": email.Subject,
				"reason":  "no_matching_template",
			},
		)
	}

	// Get handler type name for tracking
	handlerType := fmt.Sprintf("%T", handler)
	o.logger.Info("Processing email with handler",
		"emailID", email.EmailID,
		"handler", handlerType,
		"subject", email.Subject)

	// Mark as processing
	if err := o.emailRepo.UpdateStatusByEmailID(ctx, email.EmailID, entity.StatusProcessing, time.Now()); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Process with the handler
	if err := handler.Process(ctx, email); err != nil {
		o.logger.Error("Handler failed to process email",
			"emailID", email.EmailID,
			"handler", handlerType,
			"error", err)

		// Mark as failed but don't return error - let other emails continue
		o.emailRepo.MarkAsProcessedByEmailID(
			ctx,
			email.EmailID,
			entity.StatusFailed,
			handlerType,
			err.Error(),
			nil,
		)
		return nil
	}

	o.logger.Info("Email processed successfully",
		"emailID", email.EmailID,
		"handler", handlerType)

	return nil
}

// ProcessPendingEmails processes any emails that were missed or failed
func (o *EmailOrchestrator) ProcessPendingEmails(ctx context.Context) error {
	// Reset stale processing emails
	if err := o.emailRepo.ResetProcessingEmails(ctx); err != nil {
		o.logger.Error("Failed to reset stale emails", "error", err)
	}

	// Get unprocessed emails
	emails, err := o.emailRepo.FindUnprocessed(ctx, 100)
	if err != nil {
		return fmt.Errorf("failed to find unprocessed emails: %w", err)
	}

	if len(emails) == 0 {
		return nil
	}

	o.logger.Info("Processing pending emails", "count", len(emails))

	for _, email := range emails {
		if err := o.ProcessEmail(ctx, email); err != nil {
			o.logger.Error("Failed to process pending email",
				"emailID", email.EmailID,
				"error", err)
		}
	}

	return nil
}
