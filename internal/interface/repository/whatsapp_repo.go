package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"
	"mailcast-service-v2/pkg/logger"
	"mailcast-service-v2/pkg/utils"
)

// WhatsappRepository handles sending payloads to WhatsApp service
type WhatsappRepository struct {
	logger      logger.Logger
	baseURL     string
	bearerToken string
	companyID   string
	agentID     string
}

// NewWhatsappRepository creates a new WhatsApp repository
func NewWhatsappRepository(logger logger.Logger) repository.WhatsappRepository {
	baseURL := os.Getenv("WHATSAPP_SERVICE_URL")
	if baseURL == "" {
		baseURL = "https://whatsapp-service.daisi.dev"
	}

	return &WhatsappRepository{
		logger:      logger,
		baseURL:     baseURL,
		bearerToken: os.Getenv("TOKEN"),
		companyID:   os.Getenv("COMPANY_ID"),
		agentID:     os.Getenv("AGENT_ID"),
	}
}

type WhatsAppImageMessage struct {
	URL string `json:"url"`
}

// SendPayload sends a payload to the WhatsApp service and returns task ID
func (r *WhatsappRepository) SendPayload(ctx context.Context, payload *entity.Payload) (string, error) {
	// Convert scheduleAt to UTC and format as ISO string
	scheduleAtUTC := payload.ScheduleAt.UTC().Format(time.RFC3339)

	// Build the message based on whether image is present
	var msg entity.SendMailcastMessage

	if payload.Image != nil && payload.Image.URL != "" {
		// Image message with caption
		msg = entity.SendMailcastMessage{
			CompanyID:   r.companyID,
			AgentID:     r.agentID,
			PhoneNumber: payload.Phone,
			Message: entity.Message{
				Image:   WhatsAppImageMessage{URL: payload.Image.URL},
				Caption: payload.Text,
			},
			ScheduleAt: scheduleAtUTC,
			Type:       "image",
		}
	} else {
		// Text-only message
		msg = entity.SendMailcastMessage{
			CompanyID:   r.companyID,
			AgentID:     r.agentID,
			PhoneNumber: payload.Phone,
			Message: entity.Message{
				Text: payload.Text,
			},
			ScheduleAt: scheduleAtUTC,
			Type:       "text",
		}
	}

	if err := msg.Message.Validate(); err != nil {
		return "", fmt.Errorf("invalid message: %w", err)
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	r.logger.Info("Sending payload to WhatsApp service",
		"payload", string(jsonData),
		"scheduleAtLocal", payload.ScheduleAt.Format(time.RFC3339),
		"scheduleAtUTC", scheduleAtUTC)

	url := fmt.Sprintf("%s/api/v1/mailcast/send-message", r.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.bearerToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errorBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorBody)
		return "", fmt.Errorf("WhatsApp service returned status %d: %v", resp.StatusCode, errorBody)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			TaskID     string `json:"taskId"`
			Status     string `json:"status"`
			ScheduleAt string `json:"scheduleAt"`
		} `json:"data"`
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	taskID := response.Data.TaskID

	r.logger.Info("Task created successfully",
		"taskId", taskID,
		"phone", payload.Phone,
		"scheduleAt", scheduleAtUTC,
		"messageType", msg.Type)

	return taskID, nil
}

// RescheduleTask updates the schedule time of an existing task
func (r *WhatsappRepository) RescheduleTask(ctx context.Context, taskID string, newScheduleTime time.Time, reason, msg string) error {
	// Build request body for reschedule - matching the actual API
	requestBody := map[string]interface{}{
		"scheduleAt": newScheduleTime.Format(time.RFC3339),
		"message": entity.Message{
			Image:   WhatsAppImageMessage{URL: utils.IMAGE_CHANGE},
			Caption: msg,
		},
	}

	// Add reason if provided
	if reason != "" {
		requestBody["reason"] = reason
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal reschedule request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/tasks/%s/reschedule", r.baseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create reschedule request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.bearerToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send reschedule request: %w", err)
	}
	defer resp.Body.Close()

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string    `json:"id"`
			ScheduledAt time.Time `json:"scheduledAt"`
			Status      string    `json:"status"`
			// Metadata    struct {
			// 	RescheduleHistory []struct {
			// 		From          time.Time `json:"from"`
			// 		To            time.Time `json:"to"`
			// 		Reason        string    `json:"reason"`
			// 		RescheduledAt time.Time `json:"rescheduledAt"`
			// 	} `json:"rescheduleHistory"`
			// } `json:"metadata"`
		} `json:"data"`
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("failed to reschedule task: %s (code: %s)", response.Error.Message, response.Error.Code)
	}

	r.logger.Info("Task rescheduled successfully",
		"taskId", taskID,
		"newScheduleTime", response.Data.ScheduledAt,
		"reason", reason)

	return nil
}
