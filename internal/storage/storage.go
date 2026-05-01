package storage

import (
	"time"

	"flight-booking/internal/domain/models"
)

type Storage interface {
	GetCities() ([]string, error)
	GetAirports(city *string) ([]models.Airport, error)
	GetAirportCodes(point string) ([]string, error)
	GetInboundSchedule(airportID string) ([]models.Schedule, error)
	GetOutboundSchedule(airportID string) ([]models.Schedule, error)
	GetFlights(departureDate time.Time, seatType models.SeatType) ([]models.Flight, error)
	SaveBooking(req models.Booking) error
	GetTicketSeatType(ticketID string, flightID int) (models.SeatType, error)
	GetSeatTypeForFlightSeat(flightID int, seatID string) (models.SeatType, error)
	IsSeatTaken(flightID int, seatID string) (bool, error)
	GetAvailableSeat(flightID int, seatType models.SeatType) (string, error)
	HasBoardingPass(ticketID string, flightID int) (bool, error)
	SaveBoardingPass(ticketID string, flightID int, seatID string) (int, error)
}
