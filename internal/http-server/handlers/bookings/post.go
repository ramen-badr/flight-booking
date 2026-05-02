package bookings

import (
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/shopspring/decimal"

	"flight-booking/internal/domain/models"
	"flight-booking/internal/lib/api/response"
	"flight-booking/internal/lib/logger/sLogger"
	"flight-booking/internal/storage"
)

type createBookingRequest struct {
	PassengerID   string `json:"passengerId"`
	PassengerName string `json:"passengerName"`
	SeatType      string `json:"seatType"`
	FlightIDs     []int  `json:"flightIds"`
}

func Create(log *slog.Logger, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.bookings.create.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req createBookingRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid request body"))
			return
		}

		if req.PassengerID == "" || req.PassengerName == "" || req.SeatType == "" || len(req.FlightIDs) == 0 {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("missing required fields"))
			return
		}

		seatType, err := parseSeatType(req.SeatType)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid booking class"))
			return
		}

		priceByFlightID, err := store.GetFlightPrices(req.FlightIDs, seatType)
		if err != nil {
			log.Error("failed to get flight prices", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		totalAmount := decimal.Zero
		flightPrices := make([]decimal.Decimal, 0, len(req.FlightIDs))
		for _, flightID := range req.FlightIDs {
			price, ok := priceByFlightID[flightID]
			if !ok {
				render.Status(r, http.StatusNotFound)
				render.JSON(w, r, response.Error("flight not found"))
				return
			}
			flightPrices = append(flightPrices, price)
			totalAmount = totalAmount.Add(price)
		}

		bookingID, err := randomString(6)
		if err != nil {
			log.Error("failed to generate booking id", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		ticketID, err := randomString(13)
		if err != nil {
			log.Error("failed to generate ticket id", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		err = store.SaveBooking(models.Booking{
			ID:            bookingID,
			TicketID:      ticketID,
			PassengerID:   req.PassengerID,
			PassengerName: req.PassengerName,
			SeatType:      seatType,
			TotalAmount:   totalAmount,
			FlightIDs:     req.FlightIDs,
			FlightPrices:  flightPrices,
		})
		if err != nil {
			log.Error("failed to save booking", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		log.Info("booking created")

		render.JSON(w, r, struct {
			response.Response
			ID       string `json:"id"`
			TicketID string `json:"ticketId"`
		}{
			Response: response.OK(),
			ID:       bookingID,
			TicketID: ticketID,
		})
	}
}

func randomString(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	result := make([]byte, 0, length)
	maxByte := byte(255 - (256 % len(charset)))

	for len(result) < length {
		buffer := make([]byte, length-len(result))
		if _, err := rand.Read(buffer); err != nil {
			return "", err
		}
		for _, value := range buffer {
			if value > maxByte {
				continue
			}
			result = append(result, charset[int(value)%len(charset)])
			if len(result) == length {
				break
			}
		}
	}

	return string(result), nil
}

func parseSeatType(value string) (models.SeatType, error) {
	switch models.SeatType(value) {
	case models.Economy, models.Comfort, models.Business:
		return models.SeatType(value), nil
	default:
		return "", errors.New("unsupported seat type")
	}
}
