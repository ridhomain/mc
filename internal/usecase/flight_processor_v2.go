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

// NewFlightProcessor creates a new flight processor
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
	fp.logger.Info("Starting flight message processing")
	extractedData := make(map[string]interface{})

	phoneList := fp.emailParser.ExtractPhoneList(body)
	extractedData["phoneCount"] = len(phoneList)
	isScheduleChanged := fp.emailParser.IsScheduleChanged(body)
	extractedData["isScheduleChanged"] = isScheduleChanged
	pnrList := fp.emailParser.ExtractProviderPnr(body)
	extractedData["providerPnr"] = pnrList.ProviderPnr
	passengerList := fp.emailParser.ExtractPassengerLastnameList(body)
	extractedData["passengers"] = passengerList

	msgPhoneList := fp.emailParser.FormatPhoneList(phoneList)
	schedules := fp.emailParser.ExtractSchedule(ctx, body)
	extractedData["scheduleCount"] = len(schedules)

	var processError error

	for _, phoneInfo := range phoneList {
		fp.logger.Info("Processing phone", "phone", phoneInfo.Phone, "name", phoneInfo.Name)
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

			if isScheduleChanged {
				fp.handleScheduleChange(i, msg, schedule, phoneInfo, pnrList.ProviderPnr, passengerList)

			} else {
				fp.logger.Info("Processing regular schedule", "segment", i)
				fp.handleRegularSchedule(i, phoneInfo, msg, schedule.DepartDateTime, prevArrivalDateTime, segmentDetails, pnrList.ProviderPnr, passengerList, schedule, msgPhoneList)
			}

			prevArrivalDateTime = schedule.ArriveDateTime
		}
	}

	// Mark email as processed with metadata
	status := "processed"
	errorDetail := ""
	if processError != nil {
		status = "failed"
		errorDetail = processError.Error()
	}

	err := fp.emailRepo.MarkAsProcessedByEmailID(ctx, emailID, status, "flight", errorDetail, extractedData)
	if err != nil {
		fp.logger.Error("Failed to mark email as processed", "emailID", emailID, "error", err)
	} else {
		fp.logger.Info("Email marked as processed", "emailID", emailID, "status", status)
	}

	return nil
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

// handleRegularSchedule handles regular flight schedule processing
func (fp *FlightProcessorV2) handleRegularSchedule(i int, phoneInfo utils.PhoneInfo, msg string, departDateTime time.Time, prevArrivalDateTime time.Time, segmentDetails string, providerPnr string, passengerList string, schedule utils.FlightSchedule, msgPhoneList string) map[string]interface{} {
	dateTimeNow := time.Now()

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
	}

	if i == 0 {
		// First segment - send immediate notification with segment details
		msgWithSegment := fmt.Sprintf("%s\n\nSegment Details:\n%s", msg, segmentDetails)
		messageData["message_with_segments"] = msgWithSegment
		messageData["immediate_send"] = true
		messageData["scheduled_time"] = time.Now().Add(2 * time.Second)

		if !pastDate {
			// Schedule 24 hours before departure
			scheduledAt := departDateTime.Add(-24 * time.Hour)
			if scheduledAt.Before(time.Now()) {
				messageData["reminder_time"] = time.Now().Add(10 * time.Second)
			} else {
				messageData["reminder_time"] = scheduledAt
			}
		}
	} else {
		if !pastDate {
			scheduledAt := departDateTime.Add(-24 * time.Hour)
			if scheduledAt.Before(time.Now()) {
				messageData["scheduled_time"] = time.Now().Add(10 * time.Second)
			} else {
				messageData["scheduled_time"] = scheduledAt
			}
		}
	}

	scheduledAt := schedule.DepartDateTime.Add(-24 * time.Hour)
	daisiPayload := &entity.Payload{
		Text:       msg,
		Phone:      phoneInfo.Phone,
		Type:       entity.FlightNotification,
		ScheduleAt: scheduledAt,
		CreatedAt:  time.Now(),
		SentAt:     time.Time{},
		Status:     "pending",
	}
	fp.whatsappRepo.SendPayload(context.Background(), daisiPayload)
	fp.logger.Info("Regular schedule processed", "segment", i, "phone", phoneInfo.Phone)
	return messageData
}

// handleScheduleChange handles flight schedule changes
func (fp *FlightProcessorV2) handleScheduleChange(i int, msg string, schedule utils.FlightSchedule, phoneInfo utils.PhoneInfo, providerPnr string, passengerList string) map[string]interface{} {
	if schedule.Status != "HK" {
		scheduledAt := schedule.DepartDateTime.Add(-24 * time.Hour)

		messageData := map[string]interface{}{
			"segment_index":  i,
			"phone":          phoneInfo.Phone,
			"name":           phoneInfo.Name,
			"message":        msg,
			"provider_pnr":   providerPnr,
			"passenger_list": passengerList,
			"segment_number": schedule.SegNo,
			"status":         schedule.Status,
			"scheduled_time": scheduledAt,
			"change_type":    "schedule_change",
		}

		fp.logger.Info("Schedule change processed", "segment", i, "phone", phoneInfo.Phone, "status", schedule.Status)

		daisiPayload := &entity.Payload{
			Text:       msg,
			Phone:      phoneInfo.Phone,
			Type:       entity.FlightNotification,
			ScheduleAt: scheduledAt,
			CreatedAt:  time.Now(),
			SentAt:     time.Time{},
			Status:     "pending",
		}
		fp.whatsappRepo.SendPayload(context.Background(), daisiPayload)
		return messageData
	}

	return nil
}

func (fp *FlightProcessorV2) ProcessPendingEmails(ctx context.Context) error {
	emails, err := fp.emailRepo.FindUnprocessed(ctx, 100)
	if err != nil {
		fp.logger.Error("Failed to get unprocessed emails", "error", err)
		return err
	}

	fp.logger.Info("Found unprocessed emails", "count", len(emails))

	for _, email := range emails {
		err := fp.ProcessFlightMessage(ctx, email.HTMLBody, email.ID)
		if err != nil {
			fp.logger.Error("Failed to process email", "emailID", email.ID, "error", err)
			// Continue with the next email
		}
	}

	return nil
}
