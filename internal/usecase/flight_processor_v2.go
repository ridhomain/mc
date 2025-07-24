package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mailcast-service-v2/internal/domain/entity"
	"mailcast-service-v2/internal/domain/repository"
	"mailcast-service-v2/pkg/logger"
	"mailcast-service-v2/pkg/utils"
)

// FlightProcessorV2 handles flight email processing logic
type FlightProcessorV2 struct {
	airlineRepo      repository.AirlineRepository
	timezoneRepo     repository.TimezoneRepository
	emailRepo        repository.EmailRepository
	whatsappRepo     repository.WhatsappRepository
	flightRecordRepo repository.FlightRecordRepository
	emailParser      *utils.EmailParserV2
	logger           logger.Logger
}

// NewFlightProcessorV2 creates a new flight processor
func NewFlightProcessorV2(
	airlineRepo repository.AirlineRepository,
	timezoneRepo repository.TimezoneRepository,
	emailRepo repository.EmailRepository,
	whatsappRepo repository.WhatsappRepository,
	flightRecordRepo repository.FlightRecordRepository,
	logger logger.Logger,
	emailParser *utils.EmailParserV2,
) *FlightProcessorV2 {
	return &FlightProcessorV2{
		airlineRepo:      airlineRepo,
		timezoneRepo:     timezoneRepo,
		emailRepo:        emailRepo,
		whatsappRepo:     whatsappRepo,
		flightRecordRepo: flightRecordRepo,
		logger:           logger,
		emailParser:      emailParser,
	}
}

// ProcessFlightMessage processes flight notification messages
func (fp *FlightProcessorV2) ProcessFlightMessage(ctx context.Context, body string, emailID string) error {
	fp.logger.Info("Starting flight message processing", "emailId", emailID)

	// Mark as PROCESSING
	err := fp.emailRepo.UpdateStatusByEmailID(ctx, emailID, entity.StatusProcessing, time.Now())
	if err != nil {
		fp.logger.Error("Failed to update status to PROCESSING", "error", err)
		return err
	}

	// Track processing steps
	processSteps := entity.ProcessSteps{}
	extractedData := make(map[string]interface{})
	var processError error

	// Extract phone list
	phoneList := fp.emailParser.ExtractPhoneList(body)
	processSteps.PhonesExtracted = true
	extractedData["phoneCount"] = len(phoneList)

	// Update steps after phone extraction
	fp.emailRepo.UpdateProcessStepsByEmailID(ctx, emailID, processSteps)

	// Extract PNR
	pnrList := fp.emailParser.ExtractProviderPnr(body)
	extractedData["providerPnr"] = pnrList.ProviderPnr

	airlinesPnr := fp.emailParser.ExtractAirlinesPnr(body)
	extractedData["airlinesPnr"] = airlinesPnr.AirlinesPnr

	// Extract passengers - complete list with LASTNAME/FIRSTNAME TITLE format
	passengerList := fp.emailParser.ExtractPassengerLastnameList(body)
	extractedData["passengers"] = passengerList

	// Extract both old and new schedules - always use new schedule
	scheduleData := fp.emailParser.ExtractSchedules(ctx, body)
	schedules := scheduleData.NewSchedules

	processSteps.SchedulesParsed = true
	extractedData["scheduleCount"] = len(schedules)
	extractedData["changedSegments"] = scheduleData.ChangedSegments

	fp.logger.Info("Schedule analysis",
		"totalSegments", len(schedules),
		"changedSegments", len(scheduleData.ChangedSegments))

	// Update steps after schedule parsing
	fp.emailRepo.UpdateProcessStepsByEmailID(ctx, emailID, processSteps)

	// Calculate total messages to send
	totalMessages := 0
	messagesQueued := 0
	flightRecordsCreated := 0

	// Process each phone and schedule combination
	for _, phoneInfo := range phoneList {
		fp.logger.Info("Processing passenger", "phone", phoneInfo.Phone, "name", phoneInfo.Name)

		for segmentIdx, schedule := range schedules {
			// Generate booking key with complete passenger name
			bookingKey := fp.createBookingKey(phoneInfo.Name, pnrList.ProviderPnr, schedule.SegNo)

			// Check if this is a first-time booking based on database
			var existingRecord *entity.FlightRecord
			isFirstBooking := false

			if fp.flightRecordRepo != nil {
				existingRecord, err = fp.flightRecordRepo.FindByBookingKey(ctx, bookingKey)
				if err != nil || existingRecord == nil {
					isFirstBooking = true
				}
			}

			segmentHasChanged := false

			// Check if this specific segment has changed
			// segmentHasChanged := scheduleData.ChangedSegments[schedule.SegNo]

			// fp.logger.Info("Segment processing decision",
			// 	"segment", schedule.SegNo,
			// 	"isFirstBooking", isFirstBooking,
			// 	"segmentHasChanged", segmentHasChanged,
			// 	"bookingKey", bookingKey)

			// Process based on booking status and changes
			if isFirstBooking {
				// FIRST BOOKING - Send welcome + schedule reminder
				fp.logger.Info("Processing first booking", "segment", schedule.SegNo)

				// Count messages
				if segmentIdx == 0 {
					totalMessages += 2 // Welcome + reminder
				} else {
					totalMessages += 1 // Just reminder
				}

				// Send welcome message for first segment only
				if segmentIdx == 0 {
					if err := fp.sendWelcomeMessage(ctx, phoneInfo, schedules, phoneList, bookingKey, pnrList.ProviderPnr); err != nil {
						fp.logger.Error("Failed to send welcome message", "error", err)
						processError = err
					} else {
						messagesQueued++
					}
				}

				// Schedule reminder for all segments if not in past
				if !fp.isFlightInPast(schedule.DepartDateTime) {
					if err := fp.scheduleReminder(ctx, phoneInfo, schedule, bookingKey, pnrList.ProviderPnr, segmentIdx, phoneList); err != nil {
						fp.logger.Error("Failed to schedule reminder", "error", err)
						processError = err
					} else {
						messagesQueued++
					}
				}

			} else {
				// Determine Schedule Changes
				if !schedule.DepartDateTime.Equal(existingRecord.DepartureUTC) {
					segmentHasChanged = true
				}

				// SCHEDULE CHANGE - Reschedule existing task or create new one
				fp.logger.Info("Processing schedule change", "segment", schedule.SegNo)
				totalMessages += 1

				if err := fp.handleScheduleChange(ctx, phoneInfo, schedule, bookingKey, pnrList.ProviderPnr, existingRecord, phoneList); err != nil {
					fp.logger.Error("Failed to handle schedule change", "error", err)
					processError = err
				} else {
					messagesQueued++
				}
			}

			// Always update or create flight record
			flightRecord := &entity.FlightRecord{
				BookingKey:        bookingKey,
				ProviderPNR:       pnrList.ProviderPnr,
				AirlinesPNR:       airlinesPnr.AirlinesPnr,
				PassengerName:     phoneInfo.Name, // Complete name with LASTNAME/FIRSTNAME TITLE
				PhoneNumber:       phoneInfo.Phone,
				FlightNumber:      schedule.FlightNo,
				DepartureUTC:      schedule.DepartDateTime,
				ArrivalUTC:        schedule.ArriveDateTime,
				DepartureAirport:  schedule.From,
				ArrivalAirport:    schedule.To,
				IsScheduleChanged: segmentHasChanged,
			}

			// Store old departure time if changed
			if segmentHasChanged && existingRecord != nil {
				flightRecord.OldDepartureUTC = &existingRecord.DepartureUTC
			}

			if fp.flightRecordRepo != nil {
				if err := fp.flightRecordRepo.Upsert(ctx, flightRecord); err != nil {
					fp.logger.Error("Failed to save flight record", "error", err)
				} else {
					flightRecordsCreated++
				}
			}

			// Update progress
			processSteps.MessagesQueued = messagesQueued
			fp.emailRepo.UpdateProcessStepsByEmailID(ctx, emailID, processSteps)
		}
	}

	// Determine final status
	finalStatus := entity.StatusCompleted
	errorDetail := ""

	if processError != nil {
		if messagesQueued == 0 {
			finalStatus = entity.StatusFailed
			errorDetail = fmt.Sprintf("Failed to process: %v", processError)
		} else {
			finalStatus = entity.StatusCompleted
			errorDetail = fmt.Sprintf("Partially completed: %d/%d messages sent. Error: %v",
				messagesQueued, totalMessages, processError)
		}
	} else if len(phoneList) == 0 || len(schedules) == 0 {
		finalStatus = entity.StatusSkipped
		errorDetail = "No valid phone numbers or schedules found"
	}

	// Mark email as processed
	extractedData["messagesQueued"] = messagesQueued
	extractedData["totalMessages"] = totalMessages
	extractedData["flightRecordsCreated"] = flightRecordsCreated

	err = fp.emailRepo.MarkAsProcessedByEmailID(ctx, emailID, finalStatus, "flight", errorDetail, extractedData)
	if err != nil {
		fp.logger.Error("Failed to mark email as processed", "error", err)
		return err
	}

	fp.logger.Info("Flight message processing completed",
		"emailId", emailID,
		"status", finalStatus,
		"messagesQueued", messagesQueued,
		"totalMessages", totalMessages)

	return processError
}

// prepareMessageAndLocations prepares message and location data
func (fp *FlightProcessorV2) prepareMessageAndLocations(ctx context.Context, schedule utils.FlightSchedule, currentPassenger utils.PhoneInfo, phoneList []utils.PhoneInfo) (string, *time.Location, *time.Location, error) {
	departFormatted := schedule.DepartDateTime.Format("2006-01-02 15:04:05")
	arriveFormatted := schedule.ArriveDateTime.Format("2006-01-02 15:04:05")

	prefix := strings.ReplaceAll(schedule.FlightNo, "/", "")
	if len(prefix) >= 2 {
		prefix = prefix[:2]
	}

	// Get airline information
	airlineEntity, err := fp.airlineRepo.GetByCode(ctx, prefix)
	if err != nil {
		fp.logger.Error("Failed to get airline", "code", prefix, "error", err)
		return "", nil, nil, err
	}

	// Get timezone information
	fromAirport, err := fp.timezoneRepo.GetByAirportCode(ctx, schedule.From)
	if err != nil {
		fp.logger.Error("Failed to get departure timezone", "code", schedule.From, "error", err)
		return "", nil, nil, err
	}

	toAirport, err := fp.timezoneRepo.GetByAirportCode(ctx, schedule.To)
	if err != nil {
		fp.logger.Error("Failed to get arrival timezone", "code", schedule.To, "error", err)
		return "", nil, nil, err
	}

	location, err := time.LoadLocation(fromAirport.TzName)
	if err != nil {
		fp.logger.Error("Error loading departure location", "error", err)
		return "", nil, nil, err
	}

	arrivalLocation, err := time.LoadLocation(toAirport.TzName)
	if err != nil {
		fp.logger.Error("Error loading arrival location", "error", err)
		return "", nil, nil, err
	}

	// Format phone list with all passengers
	phoneListFormatted := fp.emailParser.FormatPhoneList(phoneList)

	msg := fmt.Sprintf(utils.MSG_TEMPLATE,
		currentPassenger.Name, // Dear [CURRENT PASSENGER NAME]
		phoneListFormatted,    // All passengers with phone numbers
		airlineEntity.Name,
		schedule.FlightNo,
		schedule.From, fmt.Sprintf("%s | %s", fromAirport.AirportName, fromAirport.CityName),
		schedule.To, fmt.Sprintf("%s | %s", toAirport.AirportName, toAirport.CityName),
		departFormatted,
		arriveFormatted,
	)

	return msg, location, arrivalLocation, nil
}

// createBookingKey creates a unique key for flight record
func (fp *FlightProcessorV2) createBookingKey(passengerName, providerPnr string, segmentNumber int) string {
	normalized := strings.ToUpper(strings.TrimSpace(passengerName))
	return fmt.Sprintf("%s:%s:%d", normalized, strings.ToUpper(providerPnr), segmentNumber)
}

// sendWelcomeMessage sends the initial welcome message for first-time bookings
func (fp *FlightProcessorV2) sendWelcomeMessage(ctx context.Context, phoneInfo utils.PhoneInfo, schedules []utils.FlightSchedule, phoneList []utils.PhoneInfo, bookingKey string, providerPnr string) error {
	segmentDetails := fp.emailParser.FormatSegments(schedules)

	// Format phone list with all passengers
	phoneListFormatted := fp.emailParser.FormatPhoneList(phoneList)

	welcomeMsg := fmt.Sprintf(utils.MSG_TEMPLATE_1ST,
		phoneInfo.Name,     // Dear [CURRENT PASSENGER NAME]
		phoneListFormatted, // All passengers with phone numbers
		segmentDetails,
	)

	payload := &entity.Payload{
		Text:       welcomeMsg,
		Phone:      phoneInfo.Phone,
		Type:       entity.FlightNotification,
		ScheduleAt: time.Now().Add(2 * time.Second),
		CreatedAt:  time.Now(),
		Status:     "pending",
		Metadata: map[string]interface{}{
			"bookingKey":  bookingKey,
			"providerPnr": providerPnr,
			"messageType": "welcome",
		},
	}
	payload.SetImageURL(utils.IMAGE_WA_NOTIF)

	taskID, err := fp.whatsappRepo.SendPayload(ctx, payload)
	if err != nil {
		return err
	}

	fp.logger.Info("Sent welcome message", "taskId", taskID, "phone", phoneInfo.Phone)
	return nil
}

// scheduleReminder schedules a reminder message 24 hours before departure
func (fp *FlightProcessorV2) scheduleReminder(ctx context.Context, phoneInfo utils.PhoneInfo, schedule utils.FlightSchedule, bookingKey string, providerPnr string, segmentIdx int, phoneList []utils.PhoneInfo) error {
	// Prepare the message
	msg, _, _, err := fp.prepareMessageAndLocations(ctx, schedule, phoneInfo, phoneList)
	if err != nil {
		return err
	}

	scheduledAt := schedule.DepartDateTime.Add(-24 * time.Hour)
	if scheduledAt.Before(time.Now()) {
		scheduledAt = time.Now().Add(10 * time.Second)
	}

	payload := &entity.Payload{
		Text:       msg,
		Phone:      phoneInfo.Phone,
		Type:       entity.FlightNotification,
		ScheduleAt: scheduledAt,
		CreatedAt:  time.Now(),
		Status:     "pending",
		Metadata: map[string]interface{}{
			"bookingKey":   bookingKey,
			"providerPnr":  providerPnr,
			"segmentIndex": segmentIdx,
			"messageType":  "reminder",
		},
	}
	payload.SetImageURL(fp.getRotatingImage(segmentIdx))

	taskID, err := fp.whatsappRepo.SendPayload(ctx, payload)
	if err != nil {
		return err
	}

	// Update flight record with task ID
	if fp.flightRecordRepo != nil {
		fp.flightRecordRepo.UpdateTaskInfo(ctx, bookingKey, taskID, scheduledAt)
	}

	fp.logger.Info("Scheduled reminder", "taskId", taskID, "scheduledAt", scheduledAt)

	return nil
}

// handleScheduleChange handles flight schedule changes
func (fp *FlightProcessorV2) handleScheduleChange(ctx context.Context, phoneInfo utils.PhoneInfo, schedule utils.FlightSchedule, bookingKey string, providerPnr string, existingRecord *entity.FlightRecord, phoneList []utils.PhoneInfo) error {
	// TODO - Handle cancel Task

	// Prepare the message
	msg, _, _, err := fp.prepareMessageAndLocations(ctx, schedule, phoneInfo, phoneList)
	if err != nil {
		return err
	}

	newScheduledTime := schedule.DepartDateTime.Add(-24 * time.Hour)
	if newScheduledTime.Before(time.Now()) {
		newScheduledTime = time.Now().Add(10 * time.Second)
	}

	// Try to reschedule existing task if available
	if existingRecord != nil {
		err := fp.whatsappRepo.RescheduleTask(
			ctx,
			existingRecord.LastTaskID,
			newScheduledTime,
			fmt.Sprintf("Flight schedule changed from %s to %s",
				existingRecord.DepartureUTC.Format("15:04"),
				schedule.DepartDateTime.Format("15:04")),
			msg,
		)

		if err == nil {
			fp.logger.Info("Rescheduled existing task",
				"taskId", existingRecord.LastTaskID,
				"oldTime", existingRecord.DepartureUTC,
				"newTime", schedule.DepartDateTime)

			// Update flight record
			fp.flightRecordRepo.UpdateTaskInfo(ctx, bookingKey, existingRecord.LastTaskID, newScheduledTime)
			return nil
		}

		fp.logger.Error("Failed to reschedule task, creating new one", "error", err)
	}

	// Create new task if reschedule failed or no existing task
	payload := &entity.Payload{
		Text:       msg,
		Phone:      phoneInfo.Phone,
		Type:       entity.FlightNotification,
		ScheduleAt: newScheduledTime,
		CreatedAt:  time.Now(),
		Status:     "pending",
		Metadata: map[string]interface{}{
			"bookingKey":     bookingKey,
			"providerPnr":    providerPnr,
			"messageType":    "schedule_change",
			"scheduleStatus": schedule.Status,
		},
	}
	payload.SetImageURL(utils.IMAGE_CHANGE)

	taskID, err := fp.whatsappRepo.SendPayload(ctx, payload)
	if err != nil {
		return err
	}

	fp.logger.Info("Created new schedule change task", "taskId", taskID)

	if fp.flightRecordRepo != nil {
		fp.flightRecordRepo.UpdateTaskInfo(ctx, bookingKey, taskID, newScheduledTime)
	}

	return nil
}

// isFlightInPast checks if flight has already departed
func (fp *FlightProcessorV2) isFlightInPast(departTime time.Time) bool {
	return departTime.Before(time.Now())
}

// getRotatingImage returns a rotating image based on index
func (fp *FlightProcessorV2) getRotatingImage(index int) string {
	images := []string{
		utils.IMAGES_ADS_MAIN_1,
		utils.IMAGES_ADS_MAIN_2,
		utils.IMAGES_ADS_MAIN_3,
		utils.IMAGES_ADS_MAIN_4,
		utils.IMAGES_ADS_MAIN_5,
	}
	return images[index%len(images)]
}

// ProcessPendingEmails processes unprocessed emails with safety checks
func (fp *FlightProcessorV2) ProcessPendingEmails(ctx context.Context) error {
	// First, reset any stale processing emails
	if err := fp.emailRepo.ResetProcessingEmails(ctx); err != nil {
		fp.logger.Error("Failed to reset stale processing emails", "error", err)
	}

	emails, err := fp.emailRepo.FindUnprocessed(ctx, 100)
	if err != nil {
		fp.logger.Error("Failed to get unprocessed emails", "error", err)
		return err
	}

	fp.logger.Info("Found unprocessed emails", "count", len(emails))

	successCount := 0
	failCount := 0

	for _, email := range emails {
		// Use HTML body if available, otherwise fall back to plain text
		body := email.HTMLBody
		if body == "" {
			body = email.Body
		}

		err := fp.ProcessFlightMessage(ctx, body, email.EmailID)
		if err != nil {
			fp.logger.Error("Failed to process email", "emailID", email.EmailID, "error", err)
			failCount++
		} else {
			successCount++
		}
	}

	fp.logger.Info("Email processing batch completed",
		"total", len(emails),
		"success", successCount,
		"failed", failCount)

	return nil
}
