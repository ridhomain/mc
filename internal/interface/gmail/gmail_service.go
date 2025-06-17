package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"
	"mailcast-service-v2/pkg/logger"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailService handles interaction with the Gmail API
type GmailService struct {
	gmailService *gmail.Service
	emailRepo    repository.EmailRepository
	logger       logger.Logger
	pollInterval time.Duration
}

// NewGmailService creates a new Gmail service
func NewGmailService(ctx context.Context, tokenSource oauth2.TokenSource, emailRepo repository.EmailRepository, logger logger.Logger, pollInterval time.Duration) (*GmailService, error) {
	service, err := gmail.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, err
	}

	return &GmailService{
		gmailService: service,
		emailRepo:    emailRepo,
		logger:       logger,
		pollInterval: pollInterval,
	}, nil
}

// FetchEmails fetches new emails from Gmail
func (s *GmailService) FetchEmails(ctx context.Context) error {
	// Get messages from Gmail
	lastEmailAt, _ := s.emailRepo.GetLastEmail(ctx)
	var fetchFrom time.Time
	if lastEmailAt != nil {
		fetchFrom = lastEmailAt.ReceivedAt
	} else {
		fetchFrom = time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC)
	}
	fmt.Println("Fetching emails after:", fetchFrom.Format("2006/01/02"))
	req := s.gmailService.Users.Messages.List("me").Q(fmt.Sprintf("after:%s", fetchFrom.Format("2006/01/02")))
	resp, err := req.Do()
	if err != nil {
		s.logger.Error("Failed to list messages", "error", err)
		return err
	}

	if len(resp.Messages) == 0 {
		s.logger.Info("No new messages found")
		return nil
	}

	// Extract all message IDs for batch checking
	emailIDs := make([]string, len(resp.Messages))
	for i, msg := range resp.Messages {
		emailIDs[i] = msg.Id
	}

	// Batch check which emails already exist
	existingEmails, err := s.emailRepo.FindByEmailIDs(ctx, emailIDs)
	if err != nil {
		s.logger.Error("Failed to batch check existing emails", "error", err)
		// Fall back to individual processing if batch check fails
		existingEmails = make(map[string]*entity.Email)
	}

	// Process only new emails
	newEmailsCount := 0
	for _, msg := range resp.Messages {
		// Skip if email already exists
		if _, exists := existingEmails[msg.Id]; exists {
			s.logger.Debug("Email already exists, skipping", "emailID", msg.Id)
			continue
		}

		// Get the full message
		fullMsg, err := s.gmailService.Users.Messages.Get("me", msg.Id).Do()
		if err != nil {
			s.logger.Error("Failed to get message", "emailID", msg.Id, "error", err)
			continue
		}

		messageTime := time.Unix(0, fullMsg.InternalDate*int64(time.Millisecond))
		if messageTime.Before(fetchFrom) {
			s.logger.Info("Message is older than poll interval", "messageID", msg.Id, "messageTime", messageTime)
			// continue
		}

		// Convert to our domain entity
		email, err := s.convertToEmail(fullMsg)
		if err != nil {
			s.logger.Error("Failed to convert message", "emailID", msg.Id, "error", err)
			continue
		}

		if !s.FilterPattern(email.Subject) {
			continue
		}
		s.logger.Info("Email subject does match filter", "subject", email.Subject)

		// Save to repository
		err = s.emailRepo.Save(ctx, email)
		if err != nil {
			s.logger.Error("Failed to save email", "emailID", msg.Id, "error", err)
			continue
		}

		s.logger.Info("Email fetched and saved", "emailID", email.EmailID)
		newEmailsCount++
	}

	s.logger.Info("Email fetch completed",
		"totalMessages", len(resp.Messages),
		"existingEmails", len(existingEmails),
		"newEmailsSaved", newEmailsCount)

	return nil
}

// StartPolling starts polling Gmail for new emails
func (s *GmailService) StartPolling(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Gmail polling stopped")
			return
		case <-ticker.C:
			s.logger.Info("Polling Gmail for new emails")
			if err := s.FetchEmails(ctx); err != nil {
				s.logger.Error("Error polling Gmail", "error", err)
			}
		}
	}
}

func (s *GmailService) FilterPattern(subject string) bool {
	if strings.Contains(subject, "PNR") {
		return true
	}
	return false
}

// convertToEmail converts a Gmail message to our domain entity
func (s *GmailService) convertToEmail(msg *gmail.Message) (*entity.Email, error) {
	email := &entity.Email{
		// ID:        msg.Id,
		EmailID: msg.Id,
		Labels:  msg.LabelIds,
	}

	// Extract header information
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

	// Extract message body
	if msg.Payload.Body != nil && msg.Payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err != nil {
			return nil, err
		}
		email.Body = string(data)
	}

	// Handle multipart messages
	if len(msg.Payload.Parts) > 0 {
		for _, part := range msg.Payload.Parts {
			if part.MimeType == "text/plain" && part.Body != nil {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					continue
				}
				email.Body = string(data)
			} else if part.MimeType == "text/html" && part.Body != nil {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					continue
				}
				email.HTMLBody = string(data)
			} else if part.Filename != "" && part.Body != nil {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					continue
				}

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
