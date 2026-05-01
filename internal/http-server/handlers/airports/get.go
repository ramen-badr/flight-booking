package airports

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	"flight-booking/internal/lib/api/response"
	"flight-booking/internal/lib/logger/sLogger"
	"flight-booking/internal/lib/pointer"
	"flight-booking/internal/storage"
)

func Get(log *slog.Logger, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.airports.get.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		city := r.URL.Query().Get("city")

		airports, err := store.GetAirports(pointer.NilIfZeroValue(city))
		if err != nil {
			log.Error("failed to get airports", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		log.Info("airports got")

		render.JSON(w, r, struct {
			response.Response
			Airports []string `json:"airports,omitempty"`
		}{
			Response: response.OK(),
			Airports: airports,
		})
	}
}
