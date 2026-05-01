package models

import "time"

type Schedule struct {
	FlightNo    string
	Origin      string
	Destination string
	DaysOfWeek  []int
	Time        time.Time
}
