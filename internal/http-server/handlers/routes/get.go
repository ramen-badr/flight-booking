package routes

import (
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	"flight-booking/internal/domain/models"
	"flight-booking/internal/lib/api/response"
	"flight-booking/internal/lib/logger/sLogger"
	"flight-booking/internal/storage"
)

type flightResponse struct {
	ID                 int       `json:"id"`
	FlightNo           string    `json:"flightNo"`
	DepartureAirport   string    `json:"departureAirport"`
	ArrivalAirport     string    `json:"arrivalAirport"`
	ScheduledDeparture time.Time `json:"scheduledDeparture"`
	ScheduledArrival   time.Time `json:"scheduledArrival"`
}

type routeResponse struct {
	Flights []flightResponse `json:"flights"`
}

func Get(log *slog.Logger, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.routes.get.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		from := strings.TrimSpace(r.URL.Query().Get("from"))
		to := strings.TrimSpace(r.URL.Query().Get("to"))
		dateParam := strings.TrimSpace(r.URL.Query().Get("date"))
		seatClassParam := strings.TrimSpace(r.URL.Query().Get("class"))
		connectionsParam := strings.TrimSpace(r.URL.Query().Get("connections"))

		if from == "" || to == "" || dateParam == "" || seatClassParam == "" {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("from, to, date, and class are required"))
			return
		}

		seatType, err := models.ParseSeatType(seatClassParam)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid booking class"))
			return
		}

		departureDate, err := time.Parse("2006-01-02", dateParam)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid date format"))
			return
		}

		maxConnections, err := parseConnections(connectionsParam)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid connections value"))
			return
		}

		fromAirports, err := store.GetAirportCodes(from)
		if err != nil {
			log.Error("failed to resolve origin airports", sLogger.Error(err))
			render.JSON(w, r, response.Error("internal server error"))
			return
		}
		if len(fromAirports) == 0 {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("origin not found"))
			return
		}

		toAirports, err := store.GetAirportCodes(to)
		if err != nil {
			log.Error("failed to resolve destination airports", sLogger.Error(err))
			render.JSON(w, r, response.Error("internal server error"))
			return
		}
		if len(toAirports) == 0 {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("destination not found"))
			return
		}

		flights, err := store.GetFlights(departureDate, seatType)
		if err != nil {
			log.Error("failed to get flights", sLogger.Error(err))
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		flightByDeparture := make(map[string][]models.Flight)
		for _, flight := range flights {
			flightByDeparture[flight.DepartureAirport] = append(flightByDeparture[flight.DepartureAirport], flight)
		}

		for _, list := range flightByDeparture {
			sort.Slice(list, func(i, j int) bool {
				return list[i].ScheduledDeparture.Before(list[j].ScheduledDeparture)
			})
		}

		destinations := make(map[string]struct{}, len(toAirports))
		for _, airport := range toAirports {
			destinations[airport] = struct{}{}
		}

		maxSegments := len(flights)
		if maxConnections >= 0 {
			maxSegments = maxConnections + 1
		}

		routesFound := make([]routeResponse, 0)
		for _, start := range fromAirports {
			visited := map[string]bool{start: true}
			searchRoutes(start, maxSegments, time.Time{}, nil, visited, flightByDeparture, destinations, &routesFound)
		}

		log.Info("routes got")

		render.JSON(w, r, struct {
			response.Response
			Routes []routeResponse `json:"routes,omitempty"`
		}{
			Response: response.OK(),
			Routes:   routesFound,
		})
	}
}

func parseConnections(value string) (int, error) {
	if value == "" || strings.EqualFold(value, "unbound") {
		return -1, nil
	}

	connections, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if connections < 0 || connections > 3 {
		return 0, errors.New("unsupported connections value")
	}
	return connections, nil
}

func searchRoutes(
	currentAirport string,
	remainingSegments int,
	lastArrival time.Time,
	path []models.Flight,
	visited map[string]bool,
	flightsByDeparture map[string][]models.Flight,
	destinations map[string]struct{},
	routesFound *[]routeResponse,
) {
	if remainingSegments == 0 {
		return
	}

	flights := flightsByDeparture[currentAirport]
	for _, flight := range flights {
		if !lastArrival.IsZero() && flight.ScheduledDeparture.Before(lastArrival) {
			continue
		}
		if visited[flight.ArrivalAirport] {
			continue
		}

		newPath := append(append([]models.Flight(nil), path...), flight)

		newVisited := make(map[string]bool, len(visited)+1)
		for key, value := range visited {
			newVisited[key] = value
		}
		newVisited[flight.ArrivalAirport] = true

		if _, ok := destinations[flight.ArrivalAirport]; ok {
			*routesFound = append(*routesFound, routeResponse{
				Flights: toFlightResponse(newPath),
			})
		}

		if remainingSegments > 1 {
			searchRoutes(flight.ArrivalAirport, remainingSegments-1, flight.ScheduledArrival, newPath, newVisited, flightsByDeparture, destinations, routesFound)
		}
	}
}

func toFlightResponse(flights []models.Flight) []flightResponse {
	res := make([]flightResponse, 0, len(flights))
	for _, flight := range flights {
		res = append(res, flightResponse{
			ID:                 flight.ID,
			FlightNo:           flight.RouteID,
			DepartureAirport:   flight.DepartureAirport,
			ArrivalAirport:     flight.ArrivalAirport,
			ScheduledDeparture: flight.ScheduledDeparture,
			ScheduledArrival:   flight.ScheduledArrival,
		})
	}
	return res
}
