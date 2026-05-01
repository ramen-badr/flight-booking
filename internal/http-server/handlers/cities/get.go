package cities

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	"flight-booking/internal/lib/api/response"
	"flight-booking/internal/lib/logger/sLogger"
	"flight-booking/internal/storage"
)

func Get(log *slog.Logger, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.cities.get.New"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		cities, err := store.GetCities()
		if err != nil {
			log.Error("failed to get cities", sLogger.Error(err))
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		log.Info("cities got")

		render.JSON(w, r, struct {
			response.Response
			Cities []string `json:"cities,omitempty"`
		}{
			Response: response.OK(),
			Cities:   cities,
		})
	}
}
