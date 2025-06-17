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

// EmailParserV2 handles parsing email content with repository dependencies
type EmailParserV2 struct {
	timezoneRepo repository.TimezoneRepository
	logger       logger.Logger
}

// NewEmailParserV2 creates a new email parser with dependencies
func NewEmailParserV2(timezoneRepo repository.TimezoneRepository, logger logger.Logger) *EmailParserV2 {
	return &EmailParserV2{
		timezoneRepo: timezoneRepo,
		logger:       logger,
	}
}

// cleanHTMLText removes HTML tags and cleans up text
func (p *EmailParserV2) cleanHTMLText(text string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	cleaned := re.ReplaceAllString(text, "")

	// Replace HTML entities
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&#39;", "'")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")

	// Clean up whitespace
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// ExtractPhoneList extracts phone information from email body (updated for new format)
func (p *EmailParserV2) ExtractPhoneList(body string) []PhoneInfo {
	// Clean HTML first
	cleanBody := p.cleanHTMLText(body)

	// Updated regex to handle both /EN- and - separators, and the new format
	re := regexp.MustCompile(`/(\d{10,14})(?:/EN-|-)\d*([^/]+?)(?:\s*/\s*|$)`)
	matches := re.FindAllStringSubmatch(cleanBody, -1)

	var phoneList []PhoneInfo
	for _, match := range matches {
		if len(match) >= 3 {
			phoneInfo := PhoneInfo{
				Phone: match[1],                    // The phone number part
				Name:  strings.TrimSpace(match[2]), // The name part
			}
			phoneList = append(phoneList, phoneInfo)
		}
	}

	p.logger.Info("Extracted phone list", "count", len(phoneList), "phones", phoneList)
	return phoneList
}

// ExtractScheduleFromHTML extracts flight schedule from HTML table format
func (p *EmailParserV2) ExtractScheduleFromHTML(ctx context.Context, body string) []FlightSchedule {
	var schedules []FlightSchedule

	// Look for flight tables in HTML
	tableRegex := regexp.MustCompile(`(?s)<table[^>]*>.*?</table>`)
	tables := tableRegex.FindAllString(body, -1)

	for _, table := range tables {
		// Skip if it's not a flight details table
		if !strings.Contains(table, "flight details") {
			continue
		}

		// Extract rows from the table
		rowRegex := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
		rows := rowRegex.FindAllStringSubmatch(table, -1)

		for _, row := range rows {
			if len(row) < 2 {
				continue
			}

			// Extract cells from the row
			cellRegex := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
			cells := cellRegex.FindAllStringSubmatch(row[1], -1)

			// Skip header rows or rows without enough cells
			if len(cells) < 8 {
				continue
			}

			// Clean cell contents
			var cleanCells []string
			for _, cell := range cells {
				cleaned := p.cleanHTMLText(cell[1])
				cleanCells = append(cleanCells, cleaned)
			}

			// Skip if first cell doesn't look like a flight number
			flightRegex := regexp.MustCompile(`^[A-Z]{1,3}\d+$`)
			if !flightRegex.MatchString(cleanCells[0]) {
				continue
			}

			// Parse the flight information
			schedule, err := p.parseFlightRow(ctx, cleanCells)
			if err != nil {
				p.logger.Error("Error parsing flight row", "error", err, "cells", cleanCells)
				continue
			}

			if schedule != nil {
				schedules = append(schedules, *schedule)
			}
		}
	}

	p.logger.Info("Extracted schedules from HTML", "count", len(schedules))
	return schedules
}

// parseFlightRow parses a single flight row from HTML table
func (p *EmailParserV2) parseFlightRow(ctx context.Context, cells []string) (*FlightSchedule, error) {
	if len(cells) < 8 {
		return nil, fmt.Errorf("insufficient cells: %d", len(cells))
	}

	flightNo := cells[0]  // Flight
	class := cells[1]     // Class
	dateStr := cells[2]   // Date
	fromStr := cells[3]   // From
	toStr := cells[4]     // To
	status := cells[5]    // Status
	departStr := cells[6] // Depart
	arriveStr := cells[7] // Arrive

	// Extract airport codes from location strings (e.g., "CGK(Jakarta)" -> "CGK")
	fromCode := p.extractAirportCode(fromStr)
	toCode := p.extractAirportCode(toStr)

	// Get timezone information
	fromAirport, err := p.timezoneRepo.GetByAirportCode(ctx, fromCode)
	if err != nil {
		return nil, fmt.Errorf("error getting departure airport timezone for %s: %w", fromCode, err)
	}

	toAirport, err := p.timezoneRepo.GetByAirportCode(ctx, toCode)
	if err != nil {
		return nil, fmt.Errorf("error getting arrival airport timezone for %s: %w", toCode, err)
	}

	// Load timezone locations
	departLocation, err := time.LoadLocation(fromAirport.TzName)
	if err != nil {
		return nil, fmt.Errorf("error loading departure location %s: %w", fromAirport.TzName, err)
	}

	arriveLocation, err := time.LoadLocation(toAirport.TzName)
	if err != nil {
		return nil, fmt.Errorf("error loading arrival location %s: %w", toAirport.TzName, err)
	}

	// Parse date and time
	departDateTime, err := p.parseDateTime(dateStr, departStr, departLocation)
	if err != nil {
		return nil, fmt.Errorf("error parsing departure datetime: %w", err)
	}

	arriveDateTime, err := p.parseDateTime(dateStr, arriveStr, arriveLocation)
	if err != nil {
		return nil, fmt.Errorf("error parsing arrival datetime: %w", err)
	}

	schedule := &FlightSchedule{
		SegNo:          1, // You might need to track this differently
		FlightNo:       flightNo,
		Class:          class,
		From:           fromCode,
		To:             toCode,
		DepartDateTime: departDateTime,
		ArriveDateTime: arriveDateTime,
		Status:         status,
	}

	return schedule, nil
}

// extractAirportCode extracts airport code from location string like "CGK(Jakarta)"
func (p *EmailParserV2) extractAirportCode(location string) string {
	re := regexp.MustCompile(`^([A-Z]{3,4})`)
	matches := re.FindStringSubmatch(location)
	if len(matches) > 1 {
		return matches[1]
	}
	return location // fallback to original string
}

// parseDateTime parses date and time from HTML table format
func (p *EmailParserV2) parseDateTime(dateStr, timeStr string, location *time.Location) (time.Time, error) {
	// Extract time from format like "05:30(Sun)"
	timeRegex := regexp.MustCompile(`(\d{2}):(\d{2})`)
	timeMatches := timeRegex.FindStringSubmatch(timeStr)
	if len(timeMatches) < 3 {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}

	// For now, return a placeholder - you'll need to implement proper date parsing
	// based on your specific date format in the HTML
	currentYear := time.Now().Year()

	// Parse month from "01MAR" format
	monthMap := map[string]time.Month{
		"JAN": time.January, "FEB": time.February, "MAR": time.March,
		"APR": time.April, "MAY": time.May, "JUN": time.June,
		"JUL": time.July, "AUG": time.August, "SEP": time.September,
		"OCT": time.October, "NOV": time.November, "DEC": time.December,
	}

	dayStr := dateStr[:2]
	monthStr := dateStr[2:]

	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day: %s", dayStr)
	}

	month, exists := monthMap[monthStr]
	if !exists {
		return time.Time{}, fmt.Errorf("invalid month: %s", monthStr)
	}

	hour, _ := strconv.Atoi(timeMatches[1])
	minute, _ := strconv.Atoi(timeMatches[2])

	return time.Date(currentYear, month, day, hour, minute, 0, 0, location), nil
}

// ExtractSchedule - updated to handle both text and HTML formats
func (p *EmailParserV2) ExtractSchedule(ctx context.Context, body string) []FlightSchedule {
	// First try HTML format
	if strings.Contains(body, "<table") {
		return p.ExtractScheduleFromHTML(ctx, body)
	}

	// Fall back to original text parsing
	return p.extractScheduleFromText(ctx, body)
}

// extractScheduleFromText - original text parsing method
func (p *EmailParserV2) extractScheduleFromText(ctx context.Context, body string) []FlightSchedule {
	lines := strings.Split(body, "\n")
	var schedules []FlightSchedule

	// Adjusted regular expression to allow for more flexible spacing between columns
	regex := regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+([A-Z])\s+([A-Z]{3,4})\s+([A-Z]{3,4})\s+(\d{2}\s+\w+\s+\d{4}\s+\d{2}:\d{2})\s+(\d{2}\s+\w+\s+\d{4}\s+\d{2}:\d{2})\s+(\S+)$`)

	scheduleStart := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		p.logger.Debug("Processing line", "line", line)

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
func (p *EmailParserV2) FormatSegments(segments []FlightSchedule) string {
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
func (p *EmailParserV2) IsScheduleChanged(body string) bool {
	return strings.Contains(body, "color:red")
}

// ExtractPccId extracts PCC ID from email body
func (p *EmailParserV2) ExtractPccId(body string) Pcc {
	cleanBody := p.cleanHTMLText(body)
	re := regexp.MustCompile(`(?i)PCC\s*:\s*(\S+)`)
	match := re.FindStringSubmatch(cleanBody)

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
func (p *EmailParserV2) ExtractProviderPnr(body string) ProviderPnr {
	cleanBody := p.cleanHTMLText(body)
	re := regexp.MustCompile(`(?i)Provider PNR\s*:\s*(\S+)`)
	match := re.FindStringSubmatch(cleanBody)

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

// ExtractPassengerList extracts full passenger list from email body
func (p *EmailParserV2) ExtractPassengerList(body string) string {
	p.logger.Info("Starting passenger list extraction")

	cleanBody := p.cleanHTMLText(body)

	// Regex to capture everything under "Passenger List :" until the next section
	reBlock := regexp.MustCompile(`(?s)Passenger List\s*:\s*(.*?)(?:\n\s*Phone list|\n\s*Schedule|\n\s*$)`)
	match := reBlock.FindStringSubmatch(cleanBody)

	if len(match) < 2 {
		p.logger.Warn("Passenger list not found in email body")
		return ""
	}

	var sb strings.Builder
	lines := strings.Split(match[1], "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "-") {
			sb.WriteString(line + "\n")
		}
	}

	result := sb.String()
	p.logger.Info("Passenger FirstName:", "count", len(result), "content", result)
	return result
}

// ExtractPassengerLastnameList extracts passenger last names from email body
func (p *EmailParserV2) ExtractPassengerLastnameList(body string) string {

	cleanBody := p.cleanHTMLText(body)

	// Look for the lastname section specifically
	re := regexp.MustCompile(`(?i)Last Name:\s*([A-Z\s/]+)`)
	match := re.FindStringSubmatch(cleanBody)

	if len(match) > 1 {
		// Split by / and clean up
		names := strings.Split(match[1], "/")
		var cleanNames []string
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name != "" {
				cleanNames = append(cleanNames, name)
			}
		}
		result := strings.Join(cleanNames, ", ")
		p.logger.Info("Passenger Lastname:", "count", len(result), "content", result)
		return result
	}

	p.logger.Info("No passenger lastnames found")
	return ""
}

// HasMatchingPassenger checks if any passenger from listDb exists in listParam
func (p *EmailParserV2) HasMatchingPassenger(listDb, listParam string) bool {
	passengersA := strings.Split(listDb, ",")
	passengersB := strings.Split(listParam, ",")

	// Normalize and trim each entry
	normalize := func(list []string) []string {
		var result []string
		for _, passenger := range list {
			result = append(result, strings.TrimSpace(passenger))
		}
		return result
	}

	passengersA = normalize(passengersA)
	passengersB = normalize(passengersB)

	// Check if any passenger in A exists in B
	for _, a := range passengersA {
		for _, b := range passengersB {
			if a == b {
				p.logger.Info("Matching passenger found", "passenger", a)
				return true
			}
		}
	}

	p.logger.Info("No matching passengers found")
	return false
}

// FormatPhoneList formats phone list for display
func (p *EmailParserV2) FormatPhoneList(phoneList []PhoneInfo) string {
	var builder strings.Builder

	for _, phone := range phoneList {
		line := fmt.Sprintf("%s - %s\n", phone.Phone, phone.Name)
		builder.WriteString(line)
	}

	return builder.String()
}
