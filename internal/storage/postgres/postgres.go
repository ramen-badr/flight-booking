package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"flight-booking/internal/config"
	"flight-booking/internal/domain/models"
	"flight-booking/internal/storage"
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

func formatSearchPattern(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return "%" + trimmed + "%"
}

func (s *Storage) GetCities() ([]string, error) {
	const op = "storage.postgres.GetCities"

	var res []string

	query := `
		SELECT DISTINCT city
		FROM (
			SELECT dep.city AS city
			FROM bookings.routes r
			JOIN bookings.airports dep ON dep.airport_code = r.departure_airport
			UNION
			SELECT arr.city AS city
			FROM bookings.routes r
			JOIN bookings.airports arr ON arr.airport_code = r.arrival_airport
		) cities
		ORDER BY city
	`

	if err := s.db.Select(&res, query); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetAirports(city *string) ([]models.Airport, error) {
	const op = "storage.postgres.GetAirports"

	var res []models.Airport

	var cityFilter *string
	if city != nil {
		pattern := formatSearchPattern(*city)
		if pattern != "" {
			cityFilter = &pattern
		}
	}

	query := `
		SELECT DISTINCT
			a.airport_code AS id,
			a.airport_name AS name,
			a.city AS city_name
		FROM bookings.airports a
		JOIN bookings.routes r ON r.departure_airport = a.airport_code OR r.arrival_airport = a.airport_code
		WHERE $1::text IS NULL OR a.city ILIKE $1
		ORDER BY a.airport_code
	`

	if err := s.db.Select(&res, query, cityFilter); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetAirportCodes(point string) ([]string, error) {
	const op = "storage.postgres.GetAirportCodes"

	var res []string

	pattern := formatSearchPattern(point)
	if pattern == "" {
		pattern = "%"
	}

	query := `
		SELECT airport_code
		FROM bookings.airports
		WHERE airport_code ILIKE $1 OR city ILIKE $1
		ORDER BY airport_code
	`

	if err := s.db.Select(&res, query, pattern); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetInboundSchedule(airportID string) ([]models.Route, error) {
	const op = "storage.postgres.GetInboundSchedule"

	var res []models.Route

	query := `
		SELECT
		    route_no AS id,
		    departure_airport AS airport_id,
			days_of_week,
			scheduled_time + duration AS time
		FROM bookings.routes
		WHERE arrival_airport = $1
		ORDER BY route_no
	`

	if err := s.db.Select(&res, query, airportID); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetOutboundSchedule(airportID string) ([]models.Route, error) {
	const op = "storage.postgres.GetOutboundSchedule"

	var res []models.Route

	query := `
		SELECT
		    route_no AS id,
		    arrival_airport AS airport_id,
			days_of_week,
			scheduled_time AS time
		FROM bookings.routes
		WHERE departure_airport = $1
		ORDER BY route_no
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
			f.scheduled_arrival
		FROM bookings.flights f
		JOIN bookings.routes r USING (route_no)
		WHERE f.scheduled_departure >= $1 AND f.scheduled_departure < $2
		  AND EXISTS (SELECT 1 FROM bookings.seats s WHERE s.airplane_code = r.airplane_code AND s.fare_conditions = $3)
	`

	if err := s.db.Select(&res, query, departureDate, departureDate.AddDate(0, 0, 1), seatType); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) GetFlightPrices(flightIDs []int, seatType models.SeatType) (map[int]decimal.Decimal, error) {
	const op = "storage.postgres.GetFlightPrices"

	if len(flightIDs) == 0 {
		return map[int]decimal.Decimal{}, nil
	}

	var rows []struct {
		FlightID int
		Amount   decimal.Decimal
	}

	query := `
		SELECT DISTINCT ON (s.flight_id)
			s.flight_id,
			s.price AS amount
		FROM bookings.segments s
		JOIN bookings.tickets t ON t.ticket_no = s.ticket_no
		JOIN bookings.bookings b ON b.book_ref = t.book_ref
		WHERE s.flight_id = ANY($1) AND s.fare_conditions = $2
		ORDER BY s.flight_id, b.book_date DESC
	`

	if err := s.db.Select(&rows, query, flightIDs, seatType); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	res := make(map[int]decimal.Decimal, len(rows))
	for _, row := range rows {
		res[row.FlightID] = row.Amount
	}

	return res, nil
}

func (s *Storage) GetPricing(flightIDs []int, seatType models.SeatType) ([]models.Pricing, error) {
	const op = "storage.postgres.GetPricing"

	if len(flightIDs) == 0 {
		return []models.Pricing{}, nil
	}

	var res []models.Pricing

	// actual_price is based on the most recent booking for each flight.
	query := `
		WITH actual AS (
			SELECT flight_id, actual_price
			FROM (
				SELECT
					s.flight_id,
					s.price AS actual_price,
					ROW_NUMBER() OVER (PARTITION BY s.flight_id ORDER BY b.book_date DESC) AS row_num
				FROM bookings.segments s
				JOIN bookings.tickets t ON t.ticket_no = s.ticket_no
				JOIN bookings.bookings b ON b.book_ref = t.book_ref
				WHERE s.flight_id = ANY($1) AND s.fare_conditions = $2
			) ranked
			WHERE row_num = 1
		),
		predicted AS (
			SELECT f.route_no, AVG(s.price) AS predicted_price
			FROM bookings.flights f
			JOIN bookings.segments s ON s.flight_id = f.flight_id
			WHERE s.fare_conditions = $2
			GROUP BY f.route_no
		)
		SELECT
			f.flight_id,
			a.actual_price,
			p.predicted_price,
			CASE
				WHEN a.actual_price IS NOT NULL AND p.predicted_price IS NOT NULL AND p.predicted_price > 0
				THEN a.actual_price / p.predicted_price
			END AS ratio
		FROM bookings.flights f
		JOIN bookings.routes r ON r.route_no = f.route_no AND r.validity @> f.scheduled_departure
		LEFT JOIN actual a ON a.flight_id = f.flight_id
		LEFT JOIN predicted p ON p.route_no = f.route_no
		WHERE f.flight_id = ANY($1)
		  AND EXISTS (SELECT 1 FROM bookings.seats s WHERE s.airplane_code = r.airplane_code AND s.fare_conditions = $2)
		ORDER BY f.flight_id
	`

	if err := s.db.Select(&res, query, flightIDs, seatType); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return res, nil
}

func (s *Storage) SaveBooking(req models.Booking) error {
	const op = "storage.postgres.SaveBooking"

	if len(req.FlightIDs) == 0 {
		return fmt.Errorf("%s: no flights provided", op)
	}

	var totalAmount decimal.Decimal
	flightPrices := make([]decimal.Decimal, len(req.FlightIDs))
	for index, flightID := range req.FlightIDs {
		price, ok := req.FlightPrices[flightID]
		if !ok {
			return fmt.Errorf("%s: %w: flight %d", op, storage.ErrFlightNotFound, flightID)
		}
		flightPrices[index] = price
		totalAmount = totalAmount.Add(price)
	}

	tx, err := s.db.BeginTxx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	queryBooking := `
		INSERT INTO bookings.bookings (book_ref, book_date, total_amount) 
		VALUES ($1, bookings.now(), $2)
	`

	if _, err = tx.Exec(queryBooking, req.ID, totalAmount); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	queryTicket := `
		INSERT INTO bookings.tickets (ticket_no, book_ref, passenger_id, passenger_name, outbound)
		VALUES ($1, $2, $3, $4, $5)
	`

	if _, err = tx.Exec(queryTicket, req.TicketID, req.ID, req.PassengerID, req.PassengerName, true); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	queryFlight := `
		INSERT INTO bookings.segments (ticket_no, flight_id, fare_conditions, price)
		VALUES ($1, $2, $3, $4)
	`

	for index, flightID := range req.FlightIDs {
		if _, err = tx.Exec(queryFlight, req.TicketID, flightID, req.SeatType, flightPrices[index]); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Storage) GetTicketSeatType(ticketID string, flightID int) (models.SeatType, error) {
	const op = "storage.postgres.GetTicketSeatType"

	var seatType models.SeatType

	query := `
		SELECT fare_conditions
		FROM bookings.segments
		WHERE ticket_no = $1 AND flight_id = $2
	`

	if err := s.db.Get(&seatType, query, ticketID, flightID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrTicketNotFound)
		}
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return seatType, nil
}

func (s *Storage) GetSeatTypeForFlightSeat(flightID int, seatID string) (models.SeatType, error) {
	const op = "storage.postgres.GetSeatTypeForFlightSeat"

	var seatType models.SeatType

	query := `
		SELECT s.fare_conditions
		FROM bookings.flights f
		JOIN bookings.routes r ON r.route_no = f.route_no AND r.validity @> f.scheduled_departure
		JOIN bookings.seats s ON s.airplane_code = r.airplane_code
		WHERE f.flight_id = $1 AND s.seat_no = $2
	`

	if err := s.db.Get(&seatType, query, flightID, seatID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrSeatNotFound)
		}
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return seatType, nil
}

func (s *Storage) IsSeatTaken(flightID int, seatID string) (bool, error) {
	const op = "storage.postgres.IsSeatTaken"

	var exists bool

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM bookings.boarding_passes
			WHERE flight_id = $1 AND seat_no = $2
		)
	`

	if err := s.db.Get(&exists, query, flightID, seatID); err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}

func (s *Storage) GetAvailableSeat(flightID int, seatType models.SeatType) (string, error) {
	const op = "storage.postgres.GetAvailableSeat"

	var seatID string

	query := `
		SELECT s.seat_no
		FROM bookings.flights f
		JOIN bookings.routes r ON r.route_no = f.route_no AND r.validity @> f.scheduled_departure
		JOIN bookings.seats s ON s.airplane_code = r.airplane_code
		LEFT JOIN bookings.boarding_passes bp ON bp.flight_id = f.flight_id AND bp.seat_no = s.seat_no
		WHERE f.flight_id = $1 AND s.fare_conditions = $2 AND bp.seat_no IS NULL
		ORDER BY s.seat_no
		LIMIT 1
	`

	if err := s.db.Get(&seatID, query, flightID, seatType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrNoAvailableSeats)
		}
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return seatID, nil
}

func (s *Storage) HasBoardingPass(ticketID string, flightID int) (bool, error) {
	const op = "storage.postgres.HasBoardingPass"

	var exists bool

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM bookings.boarding_passes
			WHERE ticket_no = $1 AND flight_id = $2
		)
	`

	if err := s.db.Get(&exists, query, ticketID, flightID); err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}

func (s *Storage) SaveBoardingPass(ticketID string, flightID int, seatID string) (int, error) {
	const op = "storage.postgres.SaveBoardingPass"

	tx, err := s.db.BeginTxx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	if _, err = tx.Exec("SELECT pg_advisory_xact_lock($1)", flightID); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	var boardingID int

	if err = tx.Get(&boardingID, "SELECT COALESCE(MAX(boarding_no), 0) + 1 FROM bookings.boarding_passes WHERE flight_id = $1", flightID); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	query := `
		INSERT INTO bookings.boarding_passes (ticket_no, flight_id, seat_no, boarding_no, boarding_time)
		VALUES ($1, $2, $3, $4, bookings.now())
		RETURNING boarding_no
	`

	if err = tx.QueryRow(query, ticketID, flightID, seatID, boardingID).Scan(&boardingID); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return boardingID, tx.Commit()
}
