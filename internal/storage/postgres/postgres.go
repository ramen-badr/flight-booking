package postgres

import (
	"fmt"
	"time"

	"github.com/iancoleman/strcase"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"flight-booking/internal/config"
	"flight-booking/internal/domain/models"
)

type Storage struct {
	db *sqlx.DB
}

func New(cfg config.Storage) (*Storage, error) {
	const op = "storage.postgres.New"

	db, err := sqlx.Connect("pgx", fmt.Sprintf("postgres://%s:%s@%s/%s", cfg.User, cfg.Password, cfg.Address, cfg.Name))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	db.MapperFunc(strcase.ToSnake)

	return &Storage{db: db}, nil
}

func (s *Storage) GetCities() ([]string, error) {
	const op = "storage.postgres.GetCities"

	var res []string

	query := `
		SELECT DISTINCT city
		FROM bookings.airports
		ORDER BY city
	`

	if err := s.db.Select(&res, query); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetAirports(city *string) ([]string, error) {
	const op = "storage.postgres.GetAirports"

	var res []string

	query := `
		SELECT 
		    airport_name,
		FROM bookings.airports
		WHERE $1 IS NULL OR city = $1
		ORDER BY airport_name
	`

	if err := s.db.Select(&res, query, city); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetInboundRoutes(airportID string) ([]models.Route, error) {
	const op = "storage.postgres.GetInboundRoutes"

	var res []models.Route

	query := `
		SELECT 
		    route_no AS id,
		    departure_airport AS airport_id,
			days_of_week,
			scheduled_time + duration AS time, 
		FROM bookings.routes
		WHERE arrival_airport = $1
	`

	if err := s.db.Select(&res, query, airportID); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetOutboundRoutes(airportID string) ([]models.Route, error) {
	const op = "storage.postgres.GetOutboundRoutes"

	var res []models.Route

	query := `
		SELECT 
		    route_no AS id,
		    arrival_airport AS airport_id,
			days_of_week,
			scheduled_time AS time, 
		FROM bookings.routes
		WHERE arrival_airport = $1
	`

	if err := s.db.Select(&res, query, airportID); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetFlights(departureDate time.Time, seatType models.SeatType) ([]models.Flight, error) {
	const op = "storage.postgres.GetFlights"

	var res []models.Flight

	query := `
		SELECT 
			f.flight_id AS id, 
			f.route_no AS route_id, 
			r.departure_airport, 
			r.arrival_airport, 
			f.scheduled_departure, 
			f.scheduled_arrival,
		FROM bookings.flights f
		JOIN bookings.routes r USING (route_no)
		WHERE f.scheduled_departure >= $1 AND f.scheduled_departure < $2
		  AND EXISTS (SELECT 1 FROM bookings.seats s WHERE s.airplane_code = r.airplane_code AND s.fare_conditions = $3)
	`

	if err := s.db.Select(&res, query, departureDate, departureDate.Add(3*24*time.Hour), seatType); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) SaveBooking(req models.Booking) error {
	const op = "storage.postgres.SaveBooking"

	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	queryBooking := `
		INSERT INTO bookings.bookings (book_ref, book_date, total_amount) 
		VALUES ($1, bookings.now(), $2)
	`

	if _, err = tx.Exec(queryBooking, req.ID, req.TotalAmount); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	queryTicket := `
		INSERT INTO bookings.tickets (ticket_no, book_ref, passenger_id, passenger_name) 
		VALUES ($1, $2, $3, $4)
	`

	if _, err = tx.Exec(queryTicket, req.TicketID, req.ID, req.PassengerID, req.PassengerName); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	queryFlight := `
		INSERT INTO bookings.ticket_flights (ticket_no, flight_id, fare_conditions, amount) 
		VALUES ($1, $2, $3, $4)
	`

	for _, flightID := range req.FlightIDs {
		if _, err = tx.Exec(queryFlight, req.TicketID, flightID, req.SeatType, req.TotalAmount.DivRound(decimal.NewFromInt(int64(len(req.FlightIDs))), 2)); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	return fmt.Errorf("%s: %w", op, tx.Commit())
}

func (s *Storage) SaveBoardingPass(ticketID string, flightID int, seatID string) error {
	const op = "storage.postgres.SaveBoardingPass"

	query := `
		INSERT INTO bookings.boarding_passes (ticket_no, flight_id, seat_no, boarding_no) 
		VALUES ($1, $2, $3, (SELECT COALESCE(MAX(boarding_no), 0) + 1 FROM bookings.boarding_passes WHERE flight_id = $2))
	`

	if _, err := s.db.Exec(query, ticketID, flightID, seatID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
