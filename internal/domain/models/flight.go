package models

import "time"

type Flight struct {
	ID                 int
	RouteID            string
	DepartureAirport   string
	ArrivalAirport     string
	ScheduledDeparture time.Time
	ScheduledArrival   time.Time
}
