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
	lastEmailAt, _ := s.emailRepo.GetLastEmail(ctx)
	var fetchFrom time.Time
	var hasLastEmail bool

	if lastEmailAt != nil && !lastEmailAt.ReceivedAt.IsZero() {
		fetchFrom = lastEmailAt.ReceivedAt
		hasLastEmail = true
		s.logger.Info("Using last received email time",
			"lastReceivedEmailTime", fetchFrom.Format("2006-01-02 15:04:05 UTC"))
	} else {
		// Default starting point
		fetchFrom = time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC)
		hasLastEmail = false
		s.logger.Info("No previous emails, using default start date",
			"startDate", fetchFrom.Format("2006-01-02 15:04:05 UTC"))
	}

	queryDate := fetchFrom
	if hasLastEmail {
		// Go back 3 day to catch any emails we might have missed
		queryDate = fetchFrom.AddDate(0, 0, -3)
	}

	query := fmt.Sprintf("after:%s", queryDate.Format("2006/01/02"))
	s.logger.Info("Querying Gmail",
		"query", query,
		"actualCutoffTime", fetchFrom.Format("2006-01-02 15:04:05 UTC"))

	req := s.gmailService.Users.Messages.List("me").Q(query)
	resp, err := req.Do()
	if err != nil {
		s.logger.Error("Failed to list messages", "error", err)
		return err
	}

	if len(resp.Messages) == 0 {
		s.logger.Info("No new messages found")
		return nil
	}

	emailIDs := make([]string, len(resp.Messages))
	for i, msg := range resp.Messages {
		emailIDs[i] = msg.Id
	}

	existingEmails, err := s.emailRepo.FindByEmailIDs(ctx, emailIDs)
	if err != nil {
		s.logger.Error("Failed to batch check existing emails", "error", err)
		existingEmails = make(map[string]*entity.Email)
	}

	newEmailsCount := 0
	skippedOldCount := 0
	skippedExistingCount := 0

	for _, msg := range resp.Messages {
		// Skip if already in database
		if _, exists := existingEmails[msg.Id]; exists {
			s.logger.Debug("Email already exists in database", "emailID", msg.Id)
			skippedExistingCount++
			continue
		}

		fullMsg, err := s.gmailService.Users.Messages.Get("me", msg.Id).Do()
		if err != nil {
			s.logger.Error("Failed to get message", "emailID", msg.Id, "error", err)
			continue
		}

		messageTime := time.Unix(0, fullMsg.InternalDate*int64(time.Millisecond))

		if hasLastEmail && (messageTime.Before(fetchFrom) || messageTime.Equal(fetchFrom)) {
			s.logger.Debug("Message timestamp not after last received email time",
				"messageID", msg.Id,
				"messageTime", messageTime.Format("2006-01-02 15:04:05 UTC"),
				"lastReceivedTime", fetchFrom.Format("2006-01-02 15:04:05 UTC"),
				"difference", fetchFrom.Sub(messageTime).String())
			skippedOldCount++
			continue // Actually skip it this time!
		}

		// Convert to domain entity
		email, err := s.convertToEmail(fullMsg)
		if err != nil {
			s.logger.Error("Failed to convert message", "emailID", msg.Id, "error", err)
			continue
		}

		// Apply subject filter
		if !s.FilterPattern(email.Subject) {
			s.logger.Debug("Email doesn't match subject filter", "subject", email.Subject)
			continue
		}

		s.logger.Info("Processing new email",
			"subject", email.Subject,
			"emailID", email.EmailID,
			"receivedAt", email.ReceivedAt.Format("2006-01-02 15:04:05 UTC"))

		// Save to repository
		err = s.emailRepo.Save(ctx, email)
		if err != nil {
			s.logger.Error("Failed to save email", "emailID", msg.Id, "error", err)
			continue
		}

		newEmailsCount++
	}

	s.logger.Info("Email fetch completed",
		"totalFromGmail", len(resp.Messages),
		"alreadyInDB", skippedExistingCount,
		"skippedOld", skippedOldCount,
		"newEmails", newEmailsCount)

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
