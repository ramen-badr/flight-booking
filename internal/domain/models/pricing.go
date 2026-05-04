package models

import "github.com/shopspring/decimal"

type Pricing struct {
	FlightID       int
	ActualPrice    *decimal.Decimal
	PredictedPrice *decimal.Decimal
	Ratio          *decimal.Decimal
}
