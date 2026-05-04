package models

import "github.com/shopspring/decimal"

type Pricing struct {
	FlightID       int              `json:"flightId"`
	ActualPrice    *decimal.Decimal `json:"actualPrice,omitempty"`
	PredictedPrice *decimal.Decimal `json:"predictedPrice,omitempty"`
	Ratio          *decimal.Decimal `json:"ratio,omitempty"`
}
