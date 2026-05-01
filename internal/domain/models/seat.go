package models

import "fmt"

type SeatType string

const (
	Economy  SeatType = "Economy"
	Comfort  SeatType = "Comfort"
	Business SeatType = "Business"
)

func ParseSeatType(value string) (SeatType, error) {
	switch SeatType(value) {
	case Economy, Comfort, Business:
		return SeatType(value), nil
	default:
		return "", fmt.Errorf("unsupported seat type")
	}
}
