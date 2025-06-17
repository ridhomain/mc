package utils

import "time"

// PhoneInfo represents phone and name information
type PhoneInfo struct {
	Phone string
	Name  string
}

// FlightSchedule represents a flight schedule extracted from email
type FlightSchedule struct {
	SegNo          int
	FlightNo       string
	Class          string
	From           string
	To             string
	DepartDateTime time.Time
	ArriveDateTime time.Time
	Status         string
}

// Pcc represents PCC information
type Pcc struct {
	PccId string
}

// ProviderPnr represents Provider PNR information
type ProviderPnr struct {
	ProviderPnr string
}

// AirlinesPnr represents Airlines PNR information
type AirlinesPnr struct {
	AirlinesPnr string
}

// Constants
const (
	DATE_LAYOUT = "02 Jan 2006 15:04"
)
