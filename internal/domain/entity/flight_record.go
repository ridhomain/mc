// internal/domain/entity/flight_record.go
package entity

import (
	"time"
)

type FlightRecord struct {
	ID                string     `bson:"_id,omitempty"`
	BookingKey        string     `bson:"bookingKey"` // {name}:{pnr}:{segmentNo} - unique index
	ProviderPNR       string     `bson:"providerPnr"`
	AirlinesPNR       string     `bson:"airlinesPnr"`
	PassengerName     string     `bson:"passengerName"`
	PhoneNumber       string     `bson:"phoneNumber"`
	FlightNumber      string     `bson:"flightNumber"`
	DepartureUTC      time.Time  `bson:"departureUtc"`
	ArrivalUTC        time.Time  `bson:"arrivalUtc"`
	DepartureAirport  string     `bson:"departureAirport"`
	ArrivalAirport    string     `bson:"arrivalAirport"`
	OldDepartureUTC   *time.Time `bson:"oldDepartureUtc,omitempty"`
	IsScheduleChanged bool       `bson:"isScheduleChanged"`
	LastTaskID        string     `bson:"lastTaskId,omitempty"`
	LastScheduledAt   *time.Time `bson:"lastScheduledAt,omitempty"`
	CreatedAt         time.Time  `bson:"createdAt"`
	UpdatedAt         time.Time  `bson:"updatedAt"`
}
