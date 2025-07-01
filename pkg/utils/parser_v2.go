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

// ScheduleData contains both old and new schedules with change detection
type ScheduleData struct {
	OldSchedules    []FlightSchedule
	NewSchedules    []FlightSchedule
	HasOldSchedules bool
	ChangedSegments map[int]bool // segment number -> has changed
}

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

// ExtractPhoneList extracts phone information from email body with complete names
func (p *EmailParserV2) ExtractPhoneList(body string) []PhoneInfo {
	p.logger.Info("Starting ExtractPhoneList")

	// Clean HTML first
	cleanBody := p.cleanHTMLText(body)

	// Extract last names and first names separately
	lastNames := p.extractLastNames(cleanBody)
	firstNames := p.extractFirstNames(cleanBody)

	p.logger.Debug("Extracted name components",
		"lastNamesCount", len(lastNames),
		"firstNamesCount", len(firstNames),
		"lastNames", lastNames,
		"firstNames", firstNames)

	// Extract phones from phone list section
	phones := p.extractPhones(cleanBody)

	p.logger.Debug("Extracted phones",
		"phonesCount", len(phones),
		"phones", phones)

	// Combine using index-based matching
	var phoneList []PhoneInfo

	// The number of entries should be consistent across all lists
	maxLen := len(phones)
	if len(lastNames) < maxLen {
		maxLen = len(lastNames)
	}
	if len(firstNames) < maxLen {
		maxLen = len(firstNames)
	}

	p.logger.Info("Processing phone list with index matching", "maxLen", maxLen)

	for i := 0; i < maxLen; i++ {
		var phone, lastName, firstName string

		if i < len(phones) {
			phone = phones[i]
		}
		if i < len(lastNames) {
			lastName = lastNames[i]
		}
		if i < len(firstNames) {
			firstName = firstNames[i]
		}

		// Skip if no phone number
		if phone == "" {
			p.logger.Warn("Skipping entry - no phone", "index", i)
			continue
		}

		// Construct full name in format: LASTNAME/FIRSTNAME TITLE
		fullName := ""
		if lastName != "" && firstName != "" {
			fullName = fmt.Sprintf("%s/%s", lastName, firstName)
		} else if lastName != "" {
			fullName = lastName
		} else if firstName != "" {
			fullName = firstName
		}

		p.logger.Debug("Constructed phone entry",
			"index", i,
			"phone", phone,
			"lastName", lastName,
			"firstName", firstName,
			"fullName", fullName)

		phoneInfo := PhoneInfo{
			Phone: phone,
			Name:  fullName,
		}
		phoneList = append(phoneList, phoneInfo)
	}

	p.logger.Info("Phone list extraction completed",
		"totalEntries", len(phoneList),
		"phoneList", phoneList)

	return phoneList
}

// extractPhones extracts just the phone numbers from the phone list section
func (p *EmailParserV2) extractPhones(body string) []string {
	p.logger.Debug("Extracting phones from body")

	// Look for phone list section
	phoneListIndex := strings.Index(body, "Phone list:")
	if phoneListIndex == -1 {
		p.logger.Warn("Phone list section not found")
		return nil
	}

	// Extract phone list section until next section or double newline
	phoneSection := body[phoneListIndex:]

	// Find the end of phone section
	endMarkers := []string{"Schedule change :", "Old flight details", "New flight details", "=", "SegNo"}
	minEndIndex := len(phoneSection)

	for _, marker := range endMarkers {
		if idx := strings.Index(phoneSection, marker); idx > 0 && idx < minEndIndex {
			minEndIndex = idx
		}
	}

	phoneSection = phoneSection[:minEndIndex]
	p.logger.Debug("Phone section extracted", "sectionLength", len(phoneSection))

	// Extract phone numbers only
	// Match pattern: / {phone} /EN-{number} {name}
	re := regexp.MustCompile(`/\s*(\d{10,14})\s*(?:/EN-|-)\d*[^/\n]+`)
	matches := re.FindAllStringSubmatch(phoneSection, -1)

	var phones []string
	for _, match := range matches {
		if len(match) > 1 {
			phone := strings.TrimSpace(match[1])

			// Convert phone number starting with 0 to 62
			if strings.HasPrefix(phone, "0") {
				phone = "62" + phone[1:]
			}

			phones = append(phones, phone)
		}
	}

	p.logger.Debug("Phones extracted", "count", len(phones), "phones", phones)
	return phones
}

// extractLastNames extracts last names from the email body
func (p *EmailParserV2) extractLastNames(body string) []string {
	p.logger.Debug("Extracting last names")

	// Look for the lastname section
	re := regexp.MustCompile(`(?i)Last Name:\s*([^-\n]+?)(?:\s*-|First Name:|$)`)
	match := re.FindStringSubmatch(body)

	if len(match) > 1 {
		// Split by / and clean up
		nameString := strings.TrimSpace(match[1])
		names := strings.Split(nameString, "/")
		var cleanNames []string
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name != "" && !strings.Contains(strings.ToLower(name), "first name") {
				cleanNames = append(cleanNames, name)
			}
		}
		p.logger.Debug("Last names extracted", "count", len(cleanNames), "names", cleanNames)
		return cleanNames
	}

	p.logger.Warn("No last names found")
	return []string{}
}

// extractFirstNames extracts first names with titles from the email body
func (p *EmailParserV2) extractFirstNames(body string) []string {
	p.logger.Debug("Extracting first names")

	// Look for First Name section
	re := regexp.MustCompile(`(?i)First Name:\s*([^-\n]+?)(?:\s*-|Phone list:|$)`)
	match := re.FindStringSubmatch(body)

	if len(match) > 1 {
		// Split by / and clean up
		nameString := strings.TrimSpace(match[1])
		names := strings.Split(nameString, "/")
		var cleanNames []string
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name != "" && !strings.Contains(strings.ToLower(name), "phone") {
				cleanNames = append(cleanNames, name)
			}
		}
		p.logger.Debug("First names extracted", "count", len(cleanNames), "names", cleanNames)
		return cleanNames
	}

	p.logger.Warn("No first names found")
	return []string{}
}

// ExtractCompletePassengerList extracts and formats passenger names as "LASTNAME/FIRSTNAME TITLE"
func (p *EmailParserV2) ExtractCompletePassengerList(body string) string {
	p.logger.Info("Starting ExtractCompletePassengerList")

	cleanBody := p.cleanHTMLText(body)

	// Extract last names and first names
	lastNames := p.extractLastNames(cleanBody)
	firstNames := p.extractFirstNames(cleanBody)

	p.logger.Debug("Name components for passenger list",
		"lastNamesCount", len(lastNames),
		"firstNamesCount", len(firstNames))

	// Combine into proper format using index matching
	var formattedPassengers []string

	maxLen := len(lastNames)
	if len(firstNames) > maxLen {
		maxLen = len(firstNames)
	}

	for i := 0; i < maxLen; i++ {
		var formatted string

		// Get last name
		lastName := ""
		if i < len(lastNames) {
			lastName = strings.TrimSpace(lastNames[i])
		}

		// Get first name (includes title)
		firstName := ""
		if i < len(firstNames) {
			firstName = strings.TrimSpace(firstNames[i])
		}

		// Format as "LASTNAME/FIRSTNAME TITLE"
		if lastName != "" && firstName != "" {
			formatted = fmt.Sprintf("%s/%s", lastName, firstName)
		} else if lastName != "" {
			formatted = lastName
		} else if firstName != "" {
			formatted = firstName
		}

		if formatted != "" {
			formattedPassengers = append(formattedPassengers, formatted)
		}
	}

	result := strings.Join(formattedPassengers, ", ")
	p.logger.Info("Complete passenger list extracted",
		"passengerCount", len(formattedPassengers),
		"formatted", result)

	return result
}

// FormatPhoneList formats phone list for display in messages
func (p *EmailParserV2) FormatPhoneList(phoneList []PhoneInfo) string {
	p.logger.Debug("Formatting phone list", "count", len(phoneList))

	var builder strings.Builder

	for _, phone := range phoneList {
		line := fmt.Sprintf("%s - %s\n", phone.Phone, phone.Name)
		builder.WriteString(line)
	}

	result := builder.String()
	p.logger.Debug("Phone list formatted", "resultLength", len(result))

	return result
}

// ExtractPassengerLastnameList - backward compatibility
func (p *EmailParserV2) ExtractPassengerLastnameList(body string) string {
	return p.ExtractCompletePassengerList(body)
}

// ExtractScheduleWithChanges extracts both old and new schedules and detects changes
func (p *EmailParserV2) ExtractScheduleWithChanges(ctx context.Context, body string) ScheduleData {
	result := ScheduleData{
		ChangedSegments: make(map[int]bool),
	}

	// First try HTML format
	if strings.Contains(body, "<table") {
		// Extract old schedules if present
		if strings.Contains(body, "Old flight details") {
			result.HasOldSchedules = true
			result.OldSchedules = p.extractSchedulesFromTable(ctx, body, "Old flight details")
		}

		// Extract new schedules
		result.NewSchedules = p.extractSchedulesFromTable(ctx, body, "New flight details")

		// If no new schedules found but we have tables, try extracting any flight details
		if len(result.NewSchedules) == 0 {
			result.NewSchedules = p.extractSchedulesFromTable(ctx, body, "flight details")
		}

		// Compare schedules if we have both
		if result.HasOldSchedules && len(result.OldSchedules) > 0 && len(result.NewSchedules) > 0 {
			result.ChangedSegments = p.compareSchedules(result.OldSchedules, result.NewSchedules)
		}
	} else {
		// Fall back to text parsing
		result.NewSchedules = p.extractScheduleFromText(ctx, body)
	}

	p.logger.Info("Schedule extraction completed",
		"hasOld", result.HasOldSchedules,
		"oldCount", len(result.OldSchedules),
		"newCount", len(result.NewSchedules),
		"changedSegments", result.ChangedSegments)

	return result
}

// extractSchedulesFromTable extracts schedules from a specific table
func (p *EmailParserV2) extractSchedulesFromTable(ctx context.Context, body string, tableIdentifier string) []FlightSchedule {
	var schedules []FlightSchedule
	segmentCounter := 1

	// Look for all flight detail tables
	tableRegex := regexp.MustCompile(`(?s)<table[^>]*>.*?</table>`)
	tables := tableRegex.FindAllString(body, -1)

	for _, table := range tables {
		// Check if this is the table we're looking for
		if !strings.Contains(table, tableIdentifier) {
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
			schedule, err := p.parseFlightRow(ctx, cleanCells, segmentCounter)
			if err != nil {
				p.logger.Error("Error parsing flight row", "error", err, "cells", cleanCells)
				continue
			}

			if schedule != nil {
				schedules = append(schedules, *schedule)
				segmentCounter++
			}
		}

		// If we found schedules in this table, we're done
		if len(schedules) > 0 {
			break
		}
	}

	return schedules
}

// compareSchedules compares old and new schedules and returns changed segments
func (p *EmailParserV2) compareSchedules(oldSchedules, newSchedules []FlightSchedule) map[int]bool {
	changed := make(map[int]bool)

	// Create a map of old schedules by segment number for easy lookup
	oldMap := make(map[int]FlightSchedule)
	for _, old := range oldSchedules {
		oldMap[old.SegNo] = old
	}

	// Compare each new schedule with its corresponding old schedule
	for _, new := range newSchedules {
		if old, exists := oldMap[new.SegNo]; exists {
			// Compare departure times
			if !old.DepartDateTime.Equal(new.DepartDateTime) {
				changed[new.SegNo] = true
				p.logger.Info("Schedule change detected",
					"segment", new.SegNo,
					"oldDepart", old.DepartDateTime,
					"newDepart", new.DepartDateTime)
			}
			// Could also compare arrival times if needed
			if !old.ArriveDateTime.Equal(new.ArriveDateTime) {
				changed[new.SegNo] = true
				p.logger.Info("Arrival time change detected",
					"segment", new.SegNo,
					"oldArrive", old.ArriveDateTime,
					"newArrive", new.ArriveDateTime)
			}
		}
	}

	return changed
}

// ExtractSchedule - backward compatibility method
func (p *EmailParserV2) ExtractSchedule(ctx context.Context, body string) []FlightSchedule {
	data := p.ExtractScheduleWithChanges(ctx, body)
	return data.NewSchedules
}

// ExtractScheduleFromHTML - backward compatibility method
func (p *EmailParserV2) ExtractScheduleFromHTML(ctx context.Context, body string) []FlightSchedule {
	return p.extractSchedulesFromTable(ctx, body, "flight details")
}

// parseFlightRow parses a single flight row from HTML table
func (p *EmailParserV2) parseFlightRow(ctx context.Context, cells []string, segmentNumber int) (*FlightSchedule, error) {
	if len(cells) < 8 {
		return nil, fmt.Errorf("insufficient cells: %d", len(cells))
	}

	flightNo := cells[0]  // Flight
	class := cells[1]     // Class
	dateStr := cells[2]   // Date (e.g., "21JUN")
	fromStr := cells[3]   // From (e.g., "CGK(Jakarta)")
	toStr := cells[4]     // To (e.g., "SIN(Singapore)")
	status := cells[5]    // Status
	departStr := cells[6] // Depart (e.g., "05:30(Sun)")
	arriveStr := cells[7] // Arrive (e.g., "08:15(Sun)")

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

	// Handle cases where arrival is next day
	if arriveDateTime.Before(departDateTime) {
		arriveDateTime = arriveDateTime.AddDate(0, 0, 1)
	}

	schedule := &FlightSchedule{
		SegNo:          segmentNumber,
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
	// Extract time from format like "05:30(Sun)" or "08:25"
	timeRegex := regexp.MustCompile(`(\d{2}):(\d{2})`)
	timeMatches := timeRegex.FindStringSubmatch(timeStr)
	if len(timeMatches) < 3 {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}

	hour, _ := strconv.Atoi(timeMatches[1])
	minute, _ := strconv.Atoi(timeMatches[2])

	// Parse date from "21JUN" format
	if len(dateStr) < 5 {
		return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
	}

	dayStr := dateStr[:2]
	monthStr := dateStr[2:]

	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day: %s", dayStr)
	}

	monthMap := map[string]time.Month{
		"JAN": time.January, "FEB": time.February, "MAR": time.March,
		"APR": time.April, "MAY": time.May, "JUN": time.June,
		"JUL": time.July, "AUG": time.August, "SEP": time.September,
		"OCT": time.October, "NOV": time.November, "DEC": time.December,
	}

	month, exists := monthMap[monthStr]
	if !exists {
		return time.Time{}, fmt.Errorf("invalid month: %s", monthStr)
	}

	// Use current year as default, adjust if necessary
	currentYear := time.Now().Year()
	dateTime := time.Date(currentYear, month, day, hour, minute, 0, 0, location)

	// If the date is more than 6 months in the past, assume it's next year
	if time.Since(dateTime) > 180*24*time.Hour {
		dateTime = dateTime.AddDate(1, 0, 0)
	}

	return dateTime, nil
}

// extractScheduleFromText - original text parsing method (keeping for backward compatibility)
func (p *EmailParserV2) extractScheduleFromText(ctx context.Context, body string) []FlightSchedule {
	lines := strings.Split(body, "\n")
	var schedules []FlightSchedule

	// Adjusted regular expression to allow for more flexible spacing between columns
	regex := regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+([A-Z])\s+([A-Z]{3,4})\s+([A-Z]{3,4})\s+(\d{2}\s+\w+\s+\d{4}\s+\d{2}:\d{2})\s+(\d{2}\s+\w+\s+\d{4}\s+\d{2}:\d{2})\s+(\S+)$`)

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
			}
		}
	}

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

// IsScheduleChanged - DEPRECATED, kept for compatibility
func (p *EmailParserV2) IsScheduleChanged(body string) bool {
	// This method is deprecated as schedule changes are now detected
	// by comparing old and new schedules directly
	return false
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

func (p *EmailParserV2) ExtractProviderPnr(body string) ProviderPnr {
	cleanBody := p.cleanHTMLText(body)
	re := regexp.MustCompile(`(?i)Provider PNR\s*:\s*(\S+)`)
	match := re.FindStringSubmatch(cleanBody)

	var pnrList ProviderPnr
	if len(match) > 1 {
		// Simply remove "Airlines" from the captured PNR
		pnr := strings.ReplaceAll(match[1], "Airlines", "")
		pnr = strings.TrimSpace(pnr)

		pnrList = ProviderPnr{
			ProviderPnr: pnr,
		}
		p.logger.Info("Provider PNR found", "pnr", pnr)
		return pnrList
	}

	p.logger.Info("Provider PNR not found")
	return pnrList
}

func (p *EmailParserV2) ExtractAirlinesPnr(body string) AirlinesPnr {
	cleanBody := p.cleanHTMLText(body)

	re := regexp.MustCompile(`(?i)Airlines\s+PNR\s*:\s*([^\n]+?)(?:\s*Passenger|$)`)
	match := re.FindStringSubmatch(cleanBody)

	var pnrList AirlinesPnr
	if len(match) > 1 {
		pnrString := strings.TrimSpace(match[1])

		pnrString = strings.TrimRight(pnrString, " \t\r\n")

		pnrList = AirlinesPnr{
			AirlinesPnr: pnrString,
		}
		p.logger.Info("Airlines PNR found", "pnr", pnrList.AirlinesPnr)
		return pnrList
	}

	p.logger.Info("Airlines PNR not found")
	return pnrList
}

// ParseInt converts string to int
// func ParseInt(value string) int {
// 	parsedValue, _ := strconv.Atoi(value)
// 	return parsedValue
// }
