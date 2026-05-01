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

type outboundSchedule struct {
	ID         string `json:"id"`
	AirportID  string `json:"airportId"`
	DaysOfWeek []int  `json:"daysOfWeek"`
	Time       string `json:"time"`
}

func GetOutbound(log *slog.Logger, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.airports.getOutbound.New"

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

		schedule, err := store.GetOutboundSchedule(airportID)
		if err != nil {
			log.Error("failed to get outbound schedule", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		items := make([]outboundSchedule, 0, len(schedule))
		for _, item := range schedule {
			items = append(items, outboundSchedule{
				ID:         item.ID,
				AirportID:  item.AirportID,
				DaysOfWeek: item.DaysOfWeek,
				Time:       item.Time.Format("15:04:05"),
			})
		}

		log.Info("outbound schedule got")

		render.JSON(w, r, struct {
			response.Response
			Schedule []outboundSchedule `json:"schedule,omitempty"`
		}{
			Response: response.OK(),
			Schedule: items,
		})
	}
}
