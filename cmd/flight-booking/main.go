package main

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"flight-booking/internal/config"
	"flight-booking/internal/http-server/handlers/airports"
	"flight-booking/internal/http-server/handlers/bookings"
	"flight-booking/internal/http-server/handlers/checkin"
	"flight-booking/internal/http-server/handlers/cities"
	"flight-booking/internal/http-server/handlers/routes"
	"flight-booking/internal/http-server/middleware/mwLogger"
	"flight-booking/internal/lib/logger/sLogger"
	"flight-booking/internal/storage/postgres"
)

func main() {
	cfg := config.MustLoad()

	log := sLogger.SetupLogger(cfg.Env)

	log.Info("starting url-shortener", slog.String("env", cfg.Env))

	router := chi.NewRouter()

	store, err := postgres.New(cfg.Storage)
	if err != nil {
		log.Error("failed to initialize storage", sLogger.Error(err))
		return
	}

	router.Use(middleware.RequestID)
	router.Use(mwLogger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	router.Get("/airports", airports.Get(log, store))
	router.Get("/airports/{city}", airports.Get(log, store))
	router.Get("/airports/{airportID}/inbound", airports.GetInbound(log, store))
	router.Get("/airports/{airportID}/outbound", airports.GetOutbound(log, store))
	router.Get("/cities", cities.Get(log, store))
	router.Get("/routes", routes.Get(log, store))
	router.Post("/bookings", bookings.Create(log, store))
	router.Post("/check-in", checkin.Create(log, store))

	log.Info("starting server", slog.String("address", cfg.HTTPServer.Address))

	srv := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	if err = srv.ListenAndServe(); err != nil {
		log.Error("failed to start server")
	}

	log.Error("server stopped")
}
