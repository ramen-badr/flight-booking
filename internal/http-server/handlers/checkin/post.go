package checkin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	"flight-booking/internal/lib/api/response"
	"flight-booking/internal/lib/logger/sLogger"
	"flight-booking/internal/storage"
)

type checkInRequest struct {
	TicketID string `json:"ticketId"`
	FlightID int    `json:"flightId"`
	SeatID   string `json:"seatId"`
}

func Create(log *slog.Logger, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.checkin.create.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req checkInRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid request body"))
			return
		}

		if req.TicketID == "" || req.FlightID == 0 {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("ticket id and flight id are required"))
			return
		}

		alreadyCheckedIn, err := store.HasBoardingPass(req.TicketID, req.FlightID)
		if err != nil {
			log.Error("failed to check boarding pass", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}
		if alreadyCheckedIn {
			render.Status(r, http.StatusConflict)
			render.JSON(w, r, response.Error("boarding pass already exists"))
			return
		}

		ticketSeatType, err := store.GetTicketSeatType(req.TicketID, req.FlightID)
		if err != nil {
			if errors.Is(err, storage.ErrTicketNotFound) {
				render.Status(r, http.StatusNotFound)
				render.JSON(w, r, response.Error("ticket not found for flight"))
				return
			}
			log.Error("failed to get ticket seat type", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		seatID := req.SeatID
		if seatID != "" {
			seatType, err := store.GetSeatTypeForFlightSeat(req.FlightID, seatID)
			if err != nil {
				if errors.Is(err, storage.ErrSeatNotFound) {
					render.Status(r, http.StatusNotFound)
					render.JSON(w, r, response.Error("seat not found"))
					return
				}
				log.Error("failed to get seat type", sLogger.Error(err))
				render.Status(r, http.StatusInternalServerError)
				render.JSON(w, r, response.Error("internal server error"))
				return
			}

			if seatType != ticketSeatType {
				render.Status(r, http.StatusBadRequest)
				render.JSON(w, r, response.Error("seat class does not match ticket"))
				return
			}

			taken, err := store.IsSeatTaken(req.FlightID, seatID)
			if err != nil {
				log.Error("failed to check seat availability", sLogger.Error(err))
				render.Status(r, http.StatusInternalServerError)
				render.JSON(w, r, response.Error("internal server error"))
				return
			}
			if taken {
				render.Status(r, http.StatusConflict)
				render.JSON(w, r, response.Error("seat already taken"))
				return
			}
		} else {
			seatID, err = store.GetAvailableSeat(req.FlightID, ticketSeatType)
			if err != nil {
				if errors.Is(err, storage.ErrNoAvailableSeats) {
					render.Status(r, http.StatusNotFound)
					render.JSON(w, r, response.Error("no available seats"))
					return
				}
				log.Error("failed to get available seat", sLogger.Error(err))
				render.Status(r, http.StatusInternalServerError)
				render.JSON(w, r, response.Error("internal server error"))
				return
			}
		}

		boardingID, err := store.SaveBoardingPass(req.TicketID, req.FlightID, seatID)
		if err != nil {
			log.Error("failed to save boarding pass", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		log.Info("check-in completed")

		render.JSON(w, r, struct {
			response.Response
			SeatID     string `json:"seatId"`
			BoardingID int    `json:"boardingId"`
		}{
			Response:   response.OK(),
			SeatID:     seatID,
			BoardingID: boardingID,
		})
	}
}
