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

	// Log a sample of the body for debugging
	if len(body) > 500 {
		fp.logger.Debug("Email body sample", "sample", body[:500])
	} else {
		fp.logger.Debug("Email body", "body", body)
	}

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

	// Extract passengers
	passengerList := fp.emailParser.ExtractPassengerLastnameList(body)
	extractedData["passengers"] = passengerList

	msgPhoneList := fp.emailParser.FormatPhoneList(phoneList)

	// Extract schedules - parser will handle old vs new
	scheduleData := fp.emailParser.ExtractScheduleWithChanges(ctx, body)
	schedules := scheduleData.NewSchedules
	changedSegments := scheduleData.ChangedSegments

	processSteps.SchedulesParsed = true
	extractedData["scheduleCount"] = len(schedules)
	extractedData["hasOldSchedules"] = scheduleData.HasOldSchedules
	extractedData["changedSegments"] = changedSegments

	// Update steps after schedule parsing
	fp.emailRepo.UpdateProcessStepsByEmailID(ctx, emailID, processSteps)

	// Check if this is first time booking
	isFirstTimeBooking := false
	if fp.flightRecordRepo != nil && len(phoneList) > 0 && len(schedules) > 0 {
		testKey := fp.createBookingKey(phoneList[0].Name, pnrList.ProviderPnr, schedules[0].SegNo)
		_, err := fp.flightRecordRepo.FindByBookingKey(ctx, testKey)
		if err != nil {
			isFirstTimeBooking = true
		}
	}

	// Calculate total messages to send
	totalMessages := 0
	for range phoneList {
		for i, schedule := range schedules {
			if changedSegments[schedule.SegNo] {
				// Schedule change - only reminder
				totalMessages += 1
			} else if isFirstTimeBooking && i == 0 {
				// First time booking, first segment - welcome + reminder
				totalMessages += 2
			} else if isFirstTimeBooking {
				// First time booking, other segments - only reminder
				totalMessages += 1
			}
			// If not first time and no change, no messages needed
		}
	}
	processSteps.TotalMessages = totalMessages

	// Process each phone and schedule combination
	messagesQueued := 0
	var messages []map[string]interface{}
	flightRecordsCreated := 0

	for _, phoneInfo := range phoneList {
		fp.logger.Info("Processing phone", "phone", phoneInfo.Phone, "name", phoneInfo.Name)

		for segmentIdx, schedule := range schedules {
			// Create booking key with segment number
			bookingKey := fp.createBookingKey(phoneInfo.Name, pnrList.ProviderPnr, schedule.SegNo)

			// Check if this segment has changed
			isScheduleChanged := changedSegments[schedule.SegNo]

			// Always update or create flight record
			flightRecord := &entity.FlightRecord{
				BookingKey:        bookingKey,
				ProviderPNR:       pnrList.ProviderPnr,
				AirlinesPNR:       airlinesPnr.AirlinesPnr,
				PassengerName:     phoneInfo.Name,
				PhoneNumber:       phoneInfo.Phone,
				FlightNumber:      schedule.FlightNo,
				DepartureUTC:      schedule.DepartDateTime,
				ArrivalUTC:        schedule.ArriveDateTime,
				DepartureAirport:  schedule.From,
				ArrivalAirport:    schedule.To,
				IsScheduleChanged: isScheduleChanged,
			}

			// If this is a change, get the existing record for old departure time
			var existingRecord *entity.FlightRecord
			if isScheduleChanged && fp.flightRecordRepo != nil {
				existingRecord, _ = fp.flightRecordRepo.FindByBookingKey(ctx, bookingKey)
				if existingRecord != nil {
					flightRecord.OldDepartureUTC = &existingRecord.DepartureUTC
				}
			}

			if fp.flightRecordRepo != nil {
				if err := fp.flightRecordRepo.Upsert(ctx, flightRecord); err != nil {
					fp.logger.Error("Failed to save flight record", "error", err)
				} else {
					flightRecordsCreated++
				}
			}

			// Prepare message
			msg, location, arrivalLocation, err := fp.prepareMessageAndLocations(ctx, schedule, phoneInfo.Name, msgPhoneList)
			if err != nil {
				fp.logger.Error("Failed to prepare message", "error", err)
				processError = err
				continue
			}

			if location == nil || arrivalLocation == nil {
				fp.logger.Warn("Skipping schedule due to missing location info")
				continue
			}

			var messageData map[string]interface{}

			if isScheduleChanged {
				fp.logger.Info("Processing schedule change", "segment", segmentIdx, "segNo", schedule.SegNo)
				messageData = fp.handleScheduleChange(segmentIdx, msg, schedule, phoneInfo, pnrList.ProviderPnr, passengerList, bookingKey, existingRecord)
				if messageData != nil {
					messagesQueued++
				}
			} else if isFirstTimeBooking {
				fp.logger.Info("Processing first time booking", "segment", segmentIdx)
				segmentDetails := fp.emailParser.FormatSegments(schedules)

				// For first segment, send immediate welcome message
				if segmentIdx == 0 {
					welcomeMsg := fmt.Sprintf(utils.MSG_TEMPLATE_1ST,
						phoneInfo.Name,
						msgPhoneList,
						segmentDetails,
					)

					// Send immediate welcome message
					immediatePayload := &entity.Payload{
						Text:       welcomeMsg,
						Phone:      phoneInfo.Phone,
						Type:       entity.FlightNotification,
						ScheduleAt: time.Now().Add(2 * time.Second),
						CreatedAt:  time.Now(),
						Status:     "pending",
						Metadata: map[string]interface{}{
							"bookingKey":   bookingKey,
							"providerPnr":  pnrList.ProviderPnr,
							"segmentIndex": segmentIdx,
							"messageType":  "welcome",
						},
					}
					immediatePayload.SetImageURL(utils.IMAGE_WA_NOTIF)

					welcomeTaskID, err := fp.whatsappRepo.SendPayload(ctx, immediatePayload)
					if err != nil {
						fp.logger.Error("Failed to send immediate message", "error", err)
					} else {
						fp.logger.Info("Sent welcome message", "taskId", welcomeTaskID, "phone", phoneInfo.Phone)
						messagesQueued++
					}
				}

				// Schedule reminder for all segments
				if !fp.isFlightInPast(schedule.DepartDateTime) {
					scheduledAt := schedule.DepartDateTime.Add(-24 * time.Hour)
					if scheduledAt.Before(time.Now()) {
						scheduledAt = time.Now().Add(10 * time.Second)
					}

					reminderPayload := &entity.Payload{
						Text:       msg,
						Phone:      phoneInfo.Phone,
						Type:       entity.FlightNotification,
						ScheduleAt: scheduledAt,
						CreatedAt:  time.Now(),
						Status:     "pending",
						Metadata: map[string]interface{}{
							"bookingKey":   bookingKey,
							"providerPnr":  pnrList.ProviderPnr,
							"segmentIndex": segmentIdx,
							"messageType":  "reminder",
						},
					}
					reminderPayload.SetImageURL(fp.getRotatingImage(segmentIdx))

					reminderTaskID, err := fp.whatsappRepo.SendPayload(ctx, reminderPayload)
					if err != nil {
						fp.logger.Error("Failed to schedule reminder", "error", err)
					} else {
						fp.logger.Info("Scheduled reminder", "taskId", reminderTaskID, "scheduledAt", scheduledAt)
						messagesQueued++

						// Update flight record with task ID
						if fp.flightRecordRepo != nil {
							fp.flightRecordRepo.UpdateTaskInfo(ctx, bookingKey, reminderTaskID, scheduledAt)
						}
					}
				}

				messageData = map[string]interface{}{
					"segment_index": segmentIdx,
					"phone":         phoneInfo.Phone,
					"name":          phoneInfo.Name,
					"booking_key":   bookingKey,
				}
			} else {
				// Not first time and no change - just update the record, no messages
				fp.logger.Info("No action needed - existing booking with no changes",
					"segment", segmentIdx,
					"phone", phoneInfo.Phone)
			}

			if messageData != nil {
				messages = append(messages, messageData)
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
		} else {
			finalStatus = entity.StatusCompleted // Partial success
			errorDetail = fmt.Sprintf("Partially completed: %d/%d messages sent. Error: %v",
				messagesQueued, totalMessages, processError)
		}
	} else if len(phoneList) == 0 || len(schedules) == 0 {
		finalStatus = entity.StatusSkipped
		errorDetail = "No valid phone numbers or schedules found"
	}

	// Mark email as processed with final status
	extractedData["messagesQueued"] = messagesQueued
	extractedData["totalMessages"] = totalMessages
	extractedData["flightRecordsCreated"] = flightRecordsCreated
	extractedData["expectedFlightRecords"] = len(phoneList) * len(schedules)

	err = fp.emailRepo.MarkAsProcessedByEmailID(ctx, emailID, finalStatus, "flight", errorDetail, extractedData)
	if err != nil {
		fp.logger.Error("Failed to mark email as processed", "error", err)
		return err
	}

	fp.logger.Info("Flight message processing completed",
		"emailId", emailID,
		"status", finalStatus,
		"messagesQueued", messagesQueued,
		"totalMessages", totalMessages,
		"flightRecordsCreated", flightRecordsCreated,
		"changedSegmentsCount", len(changedSegments))

	return processError
}

// prepareMessageAndLocations prepares message and location data
func (fp *FlightProcessorV2) prepareMessageAndLocations(ctx context.Context, schedule utils.FlightSchedule, name string, msgPhoneList string) (string, *time.Location, *time.Location, error) {
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

	msg := fmt.Sprintf(utils.MSG_TEMPLATE,
		name,
		msgPhoneList,
		airlineEntity.Name,
		schedule.FlightNo,
		schedule.From, fmt.Sprintf("%s | %s", fromAirport.AirportName, fromAirport.CityName),
		schedule.To, fmt.Sprintf("%s | %s", toAirport.AirportName, toAirport.CityName),
		departFormatted,
		arriveFormatted,
	)

	return msg, location, arrivalLocation, nil
}

// handleScheduleChange handles flight schedule changes
func (fp *FlightProcessorV2) handleScheduleChange(
	i int,
	msg string,
	schedule utils.FlightSchedule,
	phoneInfo utils.PhoneInfo,
	providerPnr string,
	passengerList string,
	bookingKey string,
	existingRecord *entity.FlightRecord,
) map[string]interface{} {
	if schedule.Status != "HK" {
		fp.logger.Info("Skipping cancelled flight segment", "status", schedule.Status, "segment", i)
		return nil
	}

	newScheduledTime := schedule.DepartDateTime.Add(-24 * time.Hour)
	if newScheduledTime.Before(time.Now()) {
		newScheduledTime = time.Now().Add(10 * time.Second)
	}

	// Try to reschedule existing task if available
	rescheduled := false
	if existingRecord != nil && existingRecord.LastTaskID != "" {
		// Reschedule the existing task
		err := fp.whatsappRepo.RescheduleTask(
			context.Background(),
			existingRecord.LastTaskID,
			newScheduledTime,
			fmt.Sprintf("Flight schedule changed from %s to %s",
				existingRecord.DepartureUTC.Format("15:04"),
				schedule.DepartDateTime.Format("15:04")),
		)

		if err != nil {
			fp.logger.Error("Failed to reschedule task", "taskId", existingRecord.LastTaskID, "error", err)
		} else {
			fp.logger.Info("Rescheduled existing task",
				"taskId", existingRecord.LastTaskID,
				"oldTime", existingRecord.DepartureUTC,
				"newTime", schedule.DepartDateTime)
			rescheduled = true

			// Update flight record with new schedule
			fp.flightRecordRepo.UpdateTaskInfo(context.Background(), bookingKey, existingRecord.LastTaskID, newScheduledTime)
		}
	}

	// If reschedule failed or no existing task, create new one
	if !rescheduled {
		messageData := map[string]interface{}{
			"segment_index":  i,
			"phone":          phoneInfo.Phone,
			"name":           phoneInfo.Name,
			"message":        msg,
			"provider_pnr":   providerPnr,
			"passenger_list": passengerList,
			"segment_number": schedule.SegNo,
			"status":         schedule.Status,
			"scheduled_time": newScheduledTime,
			"change_type":    "schedule_change",
			"booking_key":    bookingKey,
		}

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
				"segmentIndex":   i,
				"messageType":    "schedule_change",
				"scheduleStatus": schedule.Status,
			},
		}
		// Set image using the new structure
		payload.SetImageURL(utils.IMAGE_CHANGE)

		taskID, err := fp.whatsappRepo.SendPayload(context.Background(), payload)
		if err != nil {
			fp.logger.Error("Failed to send schedule change", "error", err)
		} else {
			fp.logger.Info("New schedule change task created", "taskId", taskID)
			messageData["task_id"] = taskID

			if fp.flightRecordRepo != nil {
				fp.flightRecordRepo.UpdateTaskInfo(context.Background(), bookingKey, taskID, newScheduledTime)
			}
		}

		return messageData
	}

	return map[string]interface{}{
		"rescheduled":   true,
		"booking_key":   bookingKey,
		"segment_index": i,
	}
}

// createBookingKey creates a unique key with segment number
func (fp *FlightProcessorV2) createBookingKey(passengerName, providerPnr string, segmentNumber int) string {
	normalized := strings.ToUpper(strings.TrimSpace(passengerName))
	return fmt.Sprintf("%s:%s:%d", normalized, strings.ToUpper(providerPnr), segmentNumber)
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

// isFlightInPast checks if flight has already departed
func (fp *FlightProcessorV2) isFlightInPast(departTime time.Time) bool {
	return departTime.Before(time.Now())
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
