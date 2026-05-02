package models

import "github.com/shopspring/decimal"

type Booking struct {
	ID            string
	TicketID      string
	PassengerID   string
	PassengerName string
	SeatType      SeatType
	TotalAmount   decimal.Decimal
	FlightIDs     []int
	FlightPrices  []decimal.Decimal
}
