package finish

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/http/domainerror"
	"github.com/sonncat/template/internal/model"
)

// Handler обрабатывает завершение поездки
type Handler struct {
	usecase tripUseCase
	timeout time.Duration
}

// New создаёт обработчик завершения поездки
func New(usecase tripUseCase, timeout time.Duration) *Handler {
	return &Handler{
		usecase: usecase,
		timeout: timeout,
	}
}

// Handle завершает поездку и записывает HTTP-ответ
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	trip, err := h.usecase.Execute(ctx, id)
	if err != nil {
		status, problem := domainerror.Problem(err)
		problem.Instance = &r.URL.Path

		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(problem)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(trip))
}

func toAPITrip(trip *model.Trip) api.Trip {
	return api.Trip{
		Id:         trip.ID,
		UserId:     trip.UserID,
		DriverId:   trip.DriverID,
		StartPoint: api.Coordinates{Latitude: trip.StartLatitude, Longitude: trip.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: trip.EndLatitude, Longitude: trip.EndLongitude},
		Price:      trip.Price,
		Status:     api.TripStatus(trip.Status),
		StartedAt:  trip.StartedAt,
		FinishedAt: trip.FinishedAt,
	}
}
