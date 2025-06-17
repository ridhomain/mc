package utils

import (
	"context"

	"mailcast-service-v2/internal/domain/repository"
	"mailcast-service-v2/pkg/logger"
)

// Global parser instance for backward compatibility
var globalParser *EmailParser

// InitializeGlobalParser initializes the global parser with dependencies
func InitializeGlobalParser(timezoneRepo repository.TimezoneRepository, logger logger.Logger) {
	globalParser = NewEmailParser(timezoneRepo, logger)
}

// Standalone functions for backward compatibility
func ExtractPhoneList(body string) []PhoneInfo {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.ExtractPhoneList(body)
}

func ExtractSchedule(ctx context.Context, body string) []FlightSchedule {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.ExtractSchedule(ctx, body)
}

func FormatSegments(segments []FlightSchedule) string {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.FormatSegments(segments)
}

func IsScheduleChanged(body string) bool {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.IsScheduleChanged(body)
}

func ExtractPccId(body string) Pcc {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.ExtractPccId(body)
}

func ExtractProviderPnr(body string) ProviderPnr {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.ExtractProviderPnr(body)
}

func ExtractAirlinesPnr(body string) AirlinesPnr {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.ExtractAirlinesPnr(body)
}

func ExtractPassengerList(body string) string {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.ExtractPassengerList(body)
}

func ExtractPassengerLastnameList(body string) string {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.ExtractPassengerLastnameList(body)
}

// func HasMatchingPassenger(listDb, listParam string) bool {
// 	if globalParser == nil {
// 		panic("Global parser not initialized. Call InitializeGlobalParser first.")
// 	}
// 	return globalParser.HasMatchingPassenger(listDb, listParam)
// }

func FormatPhoneList(phoneList []PhoneInfo) string {
	if globalParser == nil {
		panic("Global parser not initialized. Call InitializeGlobalParser first.")
	}
	return globalParser.FormatPhoneList(phoneList)
}
