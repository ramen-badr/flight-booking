package pricing

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/shopspring/decimal"

	"flight-booking/internal/domain/models"
	"flight-booking/internal/lib/api/response"
	"flight-booking/internal/lib/logger/sLogger"
	"flight-booking/internal/storage"
)

type pricingItem struct {
	FlightID       int              `json:"flightId"`
	ActualPrice    *decimal.Decimal `json:"actualPrice,omitempty"`
	PredictedPrice *decimal.Decimal `json:"predictedPrice,omitempty"`
	Ratio          *decimal.Decimal `json:"ratio,omitempty"`
}

func Get(log *slog.Logger, store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.pricing.Get"

		log = log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		flightIDsParam := strings.TrimSpace(r.URL.Query().Get("flightIds"))
		seatClassParam := strings.TrimSpace(r.URL.Query().Get("class"))
		if flightIDsParam == "" || seatClassParam == "" {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("flightIds and class are required"))
			return
		}

		seatType, err := parseSeatType(seatClassParam)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid booking class"))
			return
		}

		flightIDs, err := parseFlightIDs(flightIDsParam)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid flightIds"))
			return
		}

		pricing, err := store.GetPricing(flightIDs, seatType)
		if err != nil {
			log.Error("failed to get pricing", sLogger.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("internal server error"))
			return
		}

		if len(pricing) == 0 {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("flights not found"))
			return
		}

		items := make([]pricingItem, 0, len(pricing))
		for _, item := range pricing {
			items = append(items, pricingItem{
				FlightID:       item.FlightID,
				ActualPrice:    item.ActualPrice,
				PredictedPrice: item.PredictedPrice,
				Ratio:          item.Ratio,
			})
		}

		log.Info("pricing got")

		render.JSON(w, r, struct {
			response.Response
			Pricing []pricingItem `json:"pricing,omitempty"`
		}{
			Response: response.OK(),
			Pricing:  items,
		})
	}
}

func parseFlightIDs(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	ids := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid flight id")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errors.New("no flight ids")
	}
	return ids, nil
}

func parseSeatType(value string) (models.SeatType, error) {
	switch models.SeatType(value) {
	case models.Economy, models.Comfort, models.Business:
		return models.SeatType(value), nil
	default:
		return "", errors.New("unsupported seat type")
	}
}
