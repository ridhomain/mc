package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"
	"mailcast-service-v2/internal/usecase"
	"mailcast-service-v2/pkg/logger"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailServiceV2 handles Gmail API with immediate processing
type GmailServiceV2 struct {
	gmailService *gmail.Service
	emailRepo    repository.EmailRepository
	orchestrator *usecase.EmailOrchestrator
	logger       logger.Logger
	pollInterval time.Duration
}

// NewGmailServiceV2 creates a new Gmail service with orchestrator
func NewGmailServiceV2(
	ctx context.Context,
	tokenSource oauth2.TokenSource,
	emailRepo repository.EmailRepository,
	orchestrator *usecase.EmailOrchestrator,
	logger logger.Logger,
	pollInterval time.Duration,
) (*GmailServiceV2, error) {
	service, err := gmail.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, err
	}

	return &GmailServiceV2{
		gmailService: service,
		emailRepo:    emailRepo,
		orchestrator: orchestrator,
		logger:       logger,
		pollInterval: pollInterval,
	}, nil
}

// StartPolling polls Gmail and processes emails immediately
func (s *GmailServiceV2) StartPolling(ctx context.Context) {
	// Process any pending emails on startup
	if err := s.orchestrator.ProcessPendingEmails(ctx); err != nil {
		s.logger.Error("Failed to process pending emails on startup", "error", err)
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Gmail polling stopped")
			return
		case <-ticker.C:
			s.logger.Info("Polling Gmail for new emails")
			if err := s.FetchAndProcessEmails(ctx); err != nil {
				s.logger.Error("Error polling Gmail", "error", err)
			}
		}
	}
}

// FetchAndProcessEmails fetches new emails and processes them immediately
func (s *GmailServiceV2) FetchAndProcessEmails(ctx context.Context) error {
	// Get last email timestamp
	lastEmail, err := s.emailRepo.GetLastEmail(ctx)
	if err != nil {
		s.logger.Error("Failed to get last email", "error", err)
	}

	var fetchFrom time.Time
	if lastEmail != nil {
		fetchFrom = lastEmail.ReceivedAt
	} else {
		fetchFrom = time.Now().AddDate(0, -6, 0) // 6 months ago
	}

	// Query Gmail
	query := fmt.Sprintf("after:%s", fetchFrom.Format("2006/01/02"))
	req := s.gmailService.Users.Messages.List("me").Q(query)
	resp, err := req.Do()
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	if len(resp.Messages) == 0 {
		s.logger.Debug("No new messages found")
		return nil
	}

	// Extract message IDs for batch checking
	emailIDs := make([]string, len(resp.Messages))
	for i, msg := range resp.Messages {
		emailIDs[i] = msg.Id
	}

	// Check which emails already exist
	existingEmails, err := s.emailRepo.FindByEmailIDs(ctx, emailIDs)
	if err != nil {
		s.logger.Error("Failed to check existing emails", "error", err)
		existingEmails = make(map[string]*entity.Email)
	}

	// Process new emails
	newCount := 0
	processedCount := 0

	for _, msg := range resp.Messages {
		// Skip if already exists
		if _, exists := existingEmails[msg.Id]; exists {
			continue
		}

		// Fetch full message
		fullMsg, err := s.gmailService.Users.Messages.Get("me", msg.Id).Do()
		if err != nil {
			s.logger.Error("Failed to get message", "msgId", msg.Id, "error", err)
			continue
		}

		// Convert to domain entity
		email, err := s.convertToEmail(fullMsg)
		if err != nil {
			s.logger.Error("Failed to convert message", "msgId", msg.Id, "error", err)
			continue
		}

		// Save email
		if err := s.emailRepo.Save(ctx, email); err != nil {
			s.logger.Error("Failed to save email", "emailID", email.EmailID, "error", err)
			continue
		}
		newCount++

		// Process immediately
		if err := s.orchestrator.ProcessEmail(ctx, email); err != nil {
			s.logger.Error("Failed to process email", "emailID", email.EmailID, "error", err)
		} else {
			processedCount++
		}
	}

	s.logger.Info("Email fetch and process completed",
		"totalMessages", len(resp.Messages),
		"newEmails", newCount,
		"processedEmails", processedCount)

	return nil
}

// convertToEmail converts Gmail message to domain entity
func (s *GmailServiceV2) convertToEmail(msg *gmail.Message) (*entity.Email, error) {
	email := &entity.Email{
		EmailID:       msg.Id,
		Labels:        msg.LabelIds,
		ProcessStatus: entity.StatusPending,
	}

	// Extract headers
	for _, header := range msg.Payload.Headers {
		switch header.Name {
		case "From":
			email.From = header.Value
		case "To":
			email.To = header.Value
		case "Subject":
			email.Subject = header.Value
		}
	}

	// Extract body
	if msg.Payload.Body != nil && msg.Payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err != nil {
			return nil, err
		}
		email.Body = string(data)
	}

	// Handle multipart messages
	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/plain" && part.Body != nil {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				email.Body = string(data)
			}
		} else if part.MimeType == "text/html" && part.Body != nil {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				email.HTMLBody = string(data)
			}
		} else if part.Filename != "" && part.Body != nil {
			data, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err == nil {
				attachment := entity.Attachment{
					Filename:    part.Filename,
					ContentType: part.MimeType,
					Data:        data,
				}
				email.Attachments = append(email.Attachments, attachment)
			}
		}
	}

	email.ReceivedAt = time.Unix(0, msg.InternalDate*1000000)
	return email, nil
}
