package storage

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"flight-booking/internal/domain/models"
)

var (
	ErrTicketNotFound   = errors.New("ticket not found")
	ErrSeatNotFound     = errors.New("seat not found")
	ErrNoAvailableSeats = errors.New("no available seats")
	ErrFlightNotFound   = errors.New("flight not found")
)

type Storage interface {
	GetCities() ([]string, error)
	GetAirports(city *string) ([]models.Airport, error)
	GetAirportCodes(point string) ([]string, error)
	GetInboundSchedule(airportID string) ([]models.Route, error)
	GetOutboundSchedule(airportID string) ([]models.Route, error)
	GetFlights(departureDate time.Time, seatType models.SeatType) ([]models.Flight, error)
	GetFlightPrices(flightIDs []int, seatType models.SeatType) (map[int]decimal.Decimal, error)
	GetPricing(flightIDs []int, seatType models.SeatType) ([]models.Pricing, error)
	SaveBooking(req models.Booking) error
	GetTicketSeatType(ticketID string, flightID int) (models.SeatType, error)
	GetSeatTypeForFlightSeat(flightID int, seatID string) (models.SeatType, error)
	IsSeatTaken(flightID int, seatID string) (bool, error)
	GetAvailableSeat(flightID int, seatType models.SeatType) (string, error)
	HasBoardingPass(ticketID string, flightID int) (bool, error)
	SaveBoardingPass(ticketID string, flightID int, seatID string) (int, error)
}
