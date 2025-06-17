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

// FlightProcessor handles flight email processing logic
type FlightProcessor struct {
	airlineRepo      repository.AirlineRepository
	timezoneRepo     repository.TimezoneRepository
	emailRepo        repository.EmailRepository
	whatsappRepo     repository.WhatsappRepository
	flightRecordRepo repository.FlightRecordRepository
	emailParser      *utils.EmailParser
	logger           logger.Logger
}

// NewFlightProcessor creates a new flight processor
func NewFlightProcessor(
	airlineRepo repository.AirlineRepository,
	timezoneRepo repository.TimezoneRepository,
	emailRepo repository.EmailRepository,
	whatsappRepo repository.WhatsappRepository,
	flightRecordRepo repository.FlightRecordRepository,
	logger logger.Logger,
	emailParser *utils.EmailParser,
) *FlightProcessor {
	return &FlightProcessor{
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
func (fp *FlightProcessor) ProcessFlightMessage(ctx context.Context, body string, emailID string) error {
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

	// Check if it's a schedule change
	isScheduleChanged := fp.emailParser.IsScheduleChanged(body)
	extractedData["isScheduleChanged"] = isScheduleChanged

	// Extract PNR
	pnrList := fp.emailParser.ExtractProviderPnr(body)
	extractedData["providerPnr"] = pnrList.ProviderPnr

	airlinesPnr := fp.emailParser.ExtractAirlinesPnr(body)
	extractedData["airlinesPnr"] = airlinesPnr.AirlinesPnr

	// Extract passengers
	passengerList := fp.emailParser.ExtractPassengerLastnameList(body)
	extractedData["passengers"] = passengerList

	msgPhoneList := fp.emailParser.FormatPhoneList(phoneList)

	// Extract schedules
	schedules := fp.emailParser.ExtractSchedule(ctx, body)
	processSteps.SchedulesParsed = true
	extractedData["scheduleCount"] = len(schedules)

	// Update steps after schedule parsing
	fp.emailRepo.UpdateProcessStepsByEmailID(ctx, emailID, processSteps)

	// Calculate total messages to send
	totalMessages := 0
	for range phoneList {
		for i := range schedules {
			if i == 0 && !isScheduleChanged {
				totalMessages += 2 // Welcome + reminder for first segment
			} else {
				totalMessages += 1
			}
		}
	}
	processSteps.TotalMessages = totalMessages

	// Process each phone and schedule combination
	messagesQueued := 0
	var messages []map[string]interface{}

	for _, phoneInfo := range phoneList {
		fp.logger.Info("Processing phone", "phone", phoneInfo.Phone, "name", phoneInfo.Name)

		// Check for existing flight record
		bookingKey := fp.createBookingKey(phoneInfo.Name, pnrList.ProviderPnr)
		localIsScheduleChanged := isScheduleChanged

		if fp.flightRecordRepo != nil && len(schedules) > 0 {
			existingRecord, err := fp.flightRecordRepo.FindByBookingKey(ctx, bookingKey)
			if err == nil && existingRecord != nil {
				if !existingRecord.DepartureUTC.Equal(schedules[0].DepartDateTime) {
					localIsScheduleChanged = true
					fp.logger.Info("Schedule change detected via flight record",
						"old", existingRecord.DepartureUTC,
						"new", schedules[0].DepartDateTime)
				}
			}

			// Update or create flight record
			if len(schedules) > 0 {
				flightRecord := &entity.FlightRecord{
					BookingKey:        bookingKey,
					ProviderPNR:       pnrList.ProviderPnr,
					PassengerName:     phoneInfo.Name,
					PhoneNumber:       phoneInfo.Phone,
					FlightNumber:      schedules[0].FlightNo,
					DepartureUTC:      schedules[0].DepartDateTime,
					ArrivalUTC:        schedules[0].ArriveDateTime,
					DepartureAirport:  schedules[0].From,
					ArrivalAirport:    schedules[0].To,
					IsScheduleChanged: localIsScheduleChanged,
				}

				if existingRecord != nil && localIsScheduleChanged {
					flightRecord.OldDepartureUTC = &existingRecord.DepartureUTC
				}

				if err := fp.flightRecordRepo.Upsert(ctx, flightRecord); err != nil {
					fp.logger.Error("Failed to save flight record", "error", err)
				}
			}
		}

		segmentDetails := fp.emailParser.FormatSegments(schedules)
		var prevArrivalDateTime time.Time

		for i, schedule := range schedules {
			fp.logger.Info("Processing schedule", schedule.DepartDateTime.String(), "segment", i)
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

			if localIsScheduleChanged {
				fp.logger.Info("Processing schedule change", "segment", i)
				if fp.flightRecordRepo != nil {
					messageData = fp.handleScheduleChange(i, msg, schedule, phoneInfo, pnrList.ProviderPnr, passengerList)
					if messageData != nil {
						messagesQueued++
					}
				}
			} else {
				fp.logger.Info("Processing regular schedule", "segment", i)
				messageData = fp.handleRegularSchedule(i, phoneInfo, msg, schedule.DepartDateTime, prevArrivalDateTime, segmentDetails, pnrList.ProviderPnr, passengerList, schedule, msgPhoneList)
				if messageData != nil {
					// Count messages sent (welcome + reminder for first segment)
					if i == 0 {
						messagesQueued += 2
					} else {
						messagesQueued += 1
					}
				}
			}

			if messageData != nil {
				messages = append(messages, messageData)
			}

			// Update progress
			processSteps.MessagesQueued = messagesQueued
			fp.emailRepo.UpdateProcessStepsByEmailID(ctx, emailID, processSteps)

			prevArrivalDateTime = schedule.ArriveDateTime
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

// ProcessPendingEmails processes unprocessed emails with safety checks
func (fp *FlightProcessor) ProcessPendingEmails(ctx context.Context) error {
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
		err := fp.ProcessFlightMessage(ctx, email.Body, email.EmailID)
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

// prepareMessageAndLocations prepares message and location data
func (fp *FlightProcessor) prepareMessageAndLocations(ctx context.Context, schedule utils.FlightSchedule, name string, msgPhoneList string) (string, *time.Location, *time.Location, error) {
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

// handleRegularSchedule handles regular flight schedule processing
func (fp *FlightProcessor) handleRegularSchedule(
	i int,
	phoneInfo utils.PhoneInfo,
	msg string,
	departDateTime time.Time,
	prevArrivalDateTime time.Time,
	segmentDetails string,
	providerPnr string,
	passengerList string,
	schedule utils.FlightSchedule,
	msgPhoneList string,
) map[string]interface{} {
	dateTimeNow := time.Now()
	bookingKey := fp.createBookingKey(phoneInfo.Name, providerPnr)

	// Check if flight is in the past
	pastDate := departDateTime.Before(dateTimeNow) && prevArrivalDateTime.Before(dateTimeNow)

	messageData := map[string]interface{}{
		"segment_index":  i,
		"phone":          phoneInfo.Phone,
		"name":           phoneInfo.Name,
		"message":        msg,
		"depart_time":    departDateTime,
		"provider_pnr":   providerPnr,
		"passenger_list": passengerList,
		"segment_number": schedule.SegNo,
		"status":         schedule.Status,
		"phone_list":     msgPhoneList,
		"past_date":      pastDate,
		"booking_key":    bookingKey,
	}

	if i == 0 {
		// First segment - use MSG_TEMPLATE_1ST for welcome message
		welcomeMsg := fmt.Sprintf(utils.MSG_TEMPLATE_1ST,
			phoneInfo.Name,
			msgPhoneList,
			segmentDetails,
		)

		messageData["message_with_segments"] = welcomeMsg
		messageData["immediate_send"] = true

		// Send immediate welcome message with MSG_TEMPLATE_1ST
		immediatePayload := &entity.Payload{
			Text:       welcomeMsg,
			Phone:      phoneInfo.Phone,
			Type:       entity.FlightNotification,
			ScheduleAt: time.Now().Add(2 * time.Second),
			CreatedAt:  time.Now(),
			Status:     "pending",
			Metadata: map[string]interface{}{
				"bookingKey":   bookingKey,
				"providerPnr":  providerPnr,
				"segmentIndex": i,
				"messageType":  "welcome",
			},
		}
		immediatePayload.SetImageURL(utils.IMAGE_WA_NOTIF)

		welcomeTaskID, err := fp.whatsappRepo.SendPayload(context.Background(), immediatePayload)
		if err != nil {
			fp.logger.Error("Failed to send immediate message", "error", err)
		} else {
			fp.logger.Info("Sent welcome message", "taskId", welcomeTaskID, "phone", phoneInfo.Phone)
			messageData["welcome_task_id"] = welcomeTaskID
		}

		if !pastDate {
			// Schedule 24 hours before departure - use regular MSG_TEMPLATE
			scheduledAt := departDateTime.Add(-24 * time.Hour)
			if scheduledAt.Before(time.Now()) {
				scheduledAt = time.Now().Add(10 * time.Second)
			}
			messageData["reminder_time"] = scheduledAt

			// Send reminder with regular template (MSG_TEMPLATE)
			reminderPayload := &entity.Payload{
				Text:       msg,
				Phone:      phoneInfo.Phone,
				Type:       entity.FlightNotification,
				ScheduleAt: scheduledAt,
				CreatedAt:  time.Now(),
				Status:     "pending",
				Metadata: map[string]interface{}{
					"bookingKey":   bookingKey,
					"providerPnr":  providerPnr,
					"segmentIndex": i,
					"messageType":  "reminder",
				},
			}
			reminderPayload.SetImageURL(fp.getRotatingImage(i))

			reminderTaskID, err := fp.whatsappRepo.SendPayload(context.Background(), reminderPayload)
			if err != nil {
				fp.logger.Error("Failed to schedule reminder", "error", err)
			} else {
				fp.logger.Info("Scheduled reminder", "taskId", reminderTaskID, "scheduledAt", scheduledAt)
				messageData["reminder_task_id"] = reminderTaskID

				// Update flight record with task ID
				if fp.flightRecordRepo != nil {
					fp.flightRecordRepo.UpdateTaskInfo(context.Background(), bookingKey, reminderTaskID, scheduledAt)
				}
			}
		}
	} else {
		// Other segments - only schedule reminder if not past
		if !pastDate {
			scheduledAt := departDateTime.Add(-24 * time.Hour)
			if scheduledAt.Before(time.Now()) {
				scheduledAt = time.Now().Add(10 * time.Second)
			}
			messageData["scheduled_time"] = scheduledAt

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
					"segmentIndex": i,
					"messageType":  "reminder",
				},
			}
			payload.SetImageURL(fp.getRotatingImage(i))

			taskID, err := fp.whatsappRepo.SendPayload(context.Background(), payload)
			if err != nil {
				fp.logger.Error("Failed to schedule message", "error", err)
			} else {
				fp.logger.Info("Scheduled message", "taskId", taskID, "scheduledAt", scheduledAt)
				messageData["task_id"] = taskID
			}
		}
	}

	fp.logger.Info("Regular schedule processed", "segment", i, "phone", phoneInfo.Phone)
	return messageData
}

// handleScheduleChange handles flight schedule changes
func (fp *FlightProcessor) handleScheduleChange(
	i int,
	msg string,
	schedule utils.FlightSchedule,
	phoneInfo utils.PhoneInfo,
	providerPnr string,
	passengerList string,
) map[string]interface{} {
	bookingKey := fp.createBookingKey(phoneInfo.Name, providerPnr)

	if schedule.Status != "HK" {
		newScheduledTime := schedule.DepartDateTime.Add(-24 * time.Hour)
		if newScheduledTime.Before(time.Now()) {
			newScheduledTime = time.Now().Add(10 * time.Second)
		}

		// Try to reschedule existing task if available
		rescheduled := false
		if fp.flightRecordRepo != nil {
			existingRecord, err := fp.flightRecordRepo.FindByBookingKey(context.Background(), bookingKey)
			if err == nil && existingRecord != nil && existingRecord.LastTaskID != "" {
				// Reschedule the existing task
				err = fp.whatsappRepo.RescheduleTask(
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

	return nil
}

func (fp *FlightProcessor) createBookingKey(passengerName, providerPnr string) string {
	normalized := strings.ToUpper(strings.TrimSpace(passengerName))
	return fmt.Sprintf("%s:%s", normalized, strings.ToUpper(providerPnr))
}

func (fp *FlightProcessor) getRotatingImage(index int) string {
	images := []string{
		utils.IMAGES_ADS_MAIN_1,
		utils.IMAGES_ADS_MAIN_2,
		utils.IMAGES_ADS_MAIN_3,
		utils.IMAGES_ADS_MAIN_4,
		utils.IMAGES_ADS_MAIN_5,
	}
	return images[index%len(images)]
}
