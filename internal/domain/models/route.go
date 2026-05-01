package models

import "time"

type Route struct {
	ID         string
	AirportID  string
	DaysOfWeek []int
	Time       time.Time
}
