package airports

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	"flight-booking/internal/lib/api/response"
	"flight-booking/internal/lib/logger/sLogger"
	"flight-booking/internal/storage"
)

type inboundSchedule struct {
	ID         string `json:"id"`
	AirportID  string `json:"airportId"`
	DaysOfWeek []int  `json:"daysOfWeek"`
	Time       string `json:"time"`
}

func GetInbound(log *slog.Logger, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.airports.getInbound.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		airportID := chi.URLParam(r, "airportID")
		if airportID == "" {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("airport id is required"))
			return
		}

		schedule, err := store.GetInboundSchedule(airportID)
		if err != nil {
			log.Error("failed to get inbound schedule", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		items := make([]inboundSchedule, 0, len(schedule))
		for _, item := range schedule {
			items = append(items, inboundSchedule{
				ID:         item.ID,
				AirportID:  item.AirportID,
				DaysOfWeek: item.DaysOfWeek,
				Time:       item.Time.Format("15:04:05"),
			})
		}

		log.Info("inbound schedule got")

		render.JSON(w, r, struct {
			response.Response
			Schedule []inboundSchedule `json:"schedule,omitempty"`
		}{
			Response: response.OK(),
			Schedule: items,
		})
	}
}
