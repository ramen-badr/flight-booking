package storage

import (
	"time"

	"flight-booking/internal/domain/models"
)

type Storage interface {
	GetCities() ([]string, error)
	GetAirports(city *string) ([]string, error)
	GetInboundRoutes(airportID string) ([]models.Route, error)
	GetOutboundRoutes(airportID string) ([]models.Route, error)
	GetFlights(departureDate time.Time, seatType models.SeatType) ([]models.Flight, error)
	SaveBooking(req models.Booking) error
	SaveBoardingPass(ticketID string, flightID int, seatID string) error
}
