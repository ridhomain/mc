package utils

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mailcast-service-v2/internal/domain/repository"
	"mailcast-service-v2/pkg/logger"
)

// EmailParser handles parsing email content with repository dependencies
type EmailParser struct {
	timezoneRepo repository.TimezoneRepository
	logger       logger.Logger
}

// NewEmailParser creates a new email parser with dependencies
func NewEmailParser(timezoneRepo repository.TimezoneRepository, logger logger.Logger) *EmailParser {
	return &EmailParser{
		timezoneRepo: timezoneRepo,
		logger:       logger,
	}
}

// ParseInt converts string to int
func ParseInt(value string) int {
	parsedValue, _ := strconv.Atoi(value)
	return parsedValue
}

// ExtractPhoneList extracts phone information from email body
func (p *EmailParser) ExtractPhoneList(body string) []PhoneInfo {
	// Updated regex pattern to match both formats
	// Log here
	p.logger.Info("Extracted phone list", "body", body)

	re := regexp.MustCompile(`(?m)^(\d{10,14})(?:/EN-|-)\d+([^\n]+)$`)
	matches := re.FindAllStringSubmatch(body, -1)

	var phoneList []PhoneInfo
	for _, match := range matches {
		if len(match) == 3 {
			phone := match[1]

			// Convert phone number starting with 0 to 62
			if strings.HasPrefix(phone, "0") {
				phone = "62" + phone[1:]
			}

			phoneInfo := PhoneInfo{
				Phone: phone,
				Name:  strings.TrimSpace(match[2]),
			}
			phoneList = append(phoneList, phoneInfo)
		}
	}

	p.logger.Info("Extracted phone list", "count", len(phoneList))
	return phoneList
}

// ExtractSchedule extracts flight schedule information from email body
func (p *EmailParser) ExtractSchedule(ctx context.Context, body string) []FlightSchedule {
	normalizedBody := strings.ReplaceAll(body, "\r\n", "\n")
	normalizedBody = strings.ReplaceAll(normalizedBody, "\r", "\n")
	lines := strings.Split(normalizedBody, "\n")
	var schedules []FlightSchedule

	// Adjusted regular expression to allow for more flexible spacing between columns
	regex := regexp.MustCompile(`(?m)^\*?\s*(\d+)\s+/?\*?(\S+)\s+([A-Z])\s+([A-Z]{3,4})\s+([A-Z]{3,4})\s+(\d{2}\s+\w+\s+\d{4}\s+\d{2}:\d{2})\s+(\d{2}\s+\w+\s+\d{4}\s+\d{2}:\d{2})\s+(\S+)\*?`)

	scheduleStart := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "SegNo FlightNo Class From  To") {
			scheduleStart = true
			continue
		}

		if scheduleStart {
			match := regex.FindStringSubmatch(line)
			if len(match) == 9 {
				segNo := match[1]
				flightNo := match[2]
				class := match[3]
				from := match[4]
				to := match[5]
				departDateTimeStr := match[6]
				arriveDateTimeStr := match[7]
				status := match[8]

				// Get timezone information using repository
				fromAirport, err := p.timezoneRepo.GetByAirportCode(ctx, from)
				if err != nil {
					p.logger.Error("Error getting departure airport timezone", "airport", from, "error", err)
					continue
				}

				toAirport, err := p.timezoneRepo.GetByAirportCode(ctx, to)
				if err != nil {
					p.logger.Error("Error getting arrival airport timezone", "airport", to, "error", err)
					continue
				}

				// Load timezone locations
				location, err := time.LoadLocation(fromAirport.TzName)
				if err != nil {
					p.logger.Error("Error loading departure location", "timezone", fromAirport.TzName, "error", err)
					continue
				}

				arrivalLocation, err := time.LoadLocation(toAirport.TzName)
				if err != nil {
					p.logger.Error("Error loading arrival location", "timezone", toAirport.TzName, "error", err)
					continue
				}

				// Parse the date strings into time.Time
				departDateTime, err := time.ParseInLocation(DATE_LAYOUT, departDateTimeStr, location)
				if err != nil {
					p.logger.Error("Error parsing depart datetime", "datetime", departDateTimeStr, "error", err)
					continue
				}

				arriveDateTime, err := time.ParseInLocation(DATE_LAYOUT, arriveDateTimeStr, arrivalLocation)
				if err != nil {
					p.logger.Error("Error parsing arrive datetime", "datetime", arriveDateTimeStr, "error", err)
					continue
				}

				// Populate the schedule struct
				schedule := FlightSchedule{
					SegNo:          ParseInt(segNo),
					FlightNo:       flightNo,
					Class:          class,
					From:           from,
					To:             to,
					DepartDateTime: departDateTime,
					ArriveDateTime: arriveDateTime,
					Status:         status,
				}
				schedules = append(schedules, schedule)
				p.logger.Info("Parsed schedule", "segNo", schedule.SegNo, "flight", schedule.FlightNo)
			}
		}
	}

	p.logger.Info("Extracted schedules", "count", len(schedules))
	return schedules
}

// FormatSegments formats flight segments for display
func (p *EmailParser) FormatSegments(segments []FlightSchedule) string {
	var result strings.Builder
	for _, segment := range segments {
		line := fmt.Sprintf("%d     %s    %s     %s   %s   %s %s %s\n",
			segment.SegNo, segment.FlightNo, segment.Class, segment.From, segment.To,
			segment.DepartDateTime.Format(DATE_LAYOUT), segment.ArriveDateTime.Format(DATE_LAYOUT), segment.Status)
		result.WriteString(line)
	}
	return result.String()
}

// IsScheduleChanged checks if the email indicates a schedule change
func (p *EmailParser) IsScheduleChanged(body string) bool {
	return strings.Contains(strings.ToLower(body), "schedule change")
}

// ExtractPccId extracts PCC ID from email body
func (p *EmailParser) ExtractPccId(body string) Pcc {
	re := regexp.MustCompile(`(?m)^PCC\s*:\s*(\S+)`)
	match := re.FindStringSubmatch(body)

	var pccList Pcc
	if len(match) > 1 {
		pccList = Pcc{
			PccId: match[1],
		}
		p.logger.Info("PCC Code found", "pcc", match[1])
		return pccList
	}

	p.logger.Info("PCC not found")
	return pccList
}

// ExtractProviderPnr extracts Provider PNR from email body
func (p *EmailParser) ExtractProviderPnr(body string) ProviderPnr {
	re := regexp.MustCompile(`(?m)^Provider PNR\s*:\s*(\S+)`)
	match := re.FindStringSubmatch(body)

	var pnrList ProviderPnr
	if len(match) > 1 {
		pnrList = ProviderPnr{
			ProviderPnr: match[1],
		}
		p.logger.Info("Provider PNR found", "pnr", match[1])
		return pnrList
	}

	p.logger.Info("Provider PNR not found")
	return pnrList
}

func (p *EmailParser) ExtractAirlinesPnr(body string) AirlinesPnr {
	re := regexp.MustCompile(`(?m)^Airlines PNR\s*:\s*(\S+)`)
	match := re.FindStringSubmatch(body)

	var pnrList AirlinesPnr
	if len(match) > 1 {
		pnrList = AirlinesPnr{
			AirlinesPnr: match[1],
		}
		p.logger.Info("Airlines PNR found", "pnr", match[1])
		return pnrList
	}

	p.logger.Info("Airlines PNR not found")
	return pnrList
}

// ExtractPassengerList extracts full passenger list from email body
func (p *EmailParser) ExtractPassengerList(body string) string {
	p.logger.Info("Starting passenger list extraction")

	// Regex to capture everything under "Passenger List :" until the next empty line
	reBlock := regexp.MustCompile(`(?s)Passenger List\s*:\s*(.*?)\n\s*\n`)
	match := reBlock.FindStringSubmatch(body)

	if len(match) < 2 {
		p.logger.Warn("Passenger list not found in email body")
		return ""
	}

	var sb strings.Builder
	lines := strings.Split(match[1], "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			sb.WriteString(line + "\n")
		}
	}

	result := sb.String()
	p.logger.Info("Passenger list extraction completed", "length", len(result))
	return result
}

// ExtractPassengerLastnameList extracts passenger last names from email body
func (p *EmailParser) ExtractPassengerLastnameList(body string) string {
	p.logger.Info("Starting passenger lastname extraction")

	// Regex to match lines under "Passenger List :"
	re := regexp.MustCompile(`(?m)^([A-Z]+),`)
	matches := re.FindAllStringSubmatch(body, -1)

	var results []string
	for _, match := range matches {
		results = append(results, match[1])
	}

	finalOutput := strings.Join(results, ", ")
	p.logger.Info("Passenger lastname extraction completed", "names", finalOutput)
	return finalOutput
}

// HasMatchingPassenger checks if any passenger from listDb exists in listParam
// func (p *EmailParser) HasMatchingPassenger(listDb, listParam string) bool {
// 	passengersA := strings.Split(listDb, ",")
// 	passengersB := strings.Split(listParam, ",")

// 	// Normalize and trim each entry
// 	normalize := func(list []string) []string {
// 		var result []string
// 		for _, passenger := range list {
// 			result = append(result, strings.TrimSpace(passenger))
// 		}
// 		return result
// 	}

// 	passengersA = normalize(passengersA)
// 	passengersB = normalize(passengersB)

// 	// Check if any passenger in A exists in B
// 	for _, a := range passengersA {
// 		for _, b := range passengersB {
// 			if a == b {
// 				p.logger.Info("Matching passenger found", "passenger", a)
// 				return true
// 			}
// 		}
// 	}

// 	p.logger.Info("No matching passengers found")
// 	return false
// }

// FormatPhoneList formats phone list for display
func (p *EmailParser) FormatPhoneList(phoneList []PhoneInfo) string {
	var builder strings.Builder

	for _, phone := range phoneList {
		line := fmt.Sprintf("%s - %s\n", phone.Phone, phone.Name)
		builder.WriteString(line)
	}

	return builder.String()
}
