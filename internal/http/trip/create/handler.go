package create

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/http/domainerror"
	"github.com/sonncat/template/internal/model"
)

// Handler обрабатывает создание поездкиы
type Handler struct {
	usecase tripUseCase
	timeout time.Duration
}

// New создаёт обработчик создания поездки
func New(usecase tripUseCase, timeout time.Duration) *Handler {
	return &Handler{
		usecase: usecase,
		timeout: timeout,
	}
}

// Handle разбирает запрос, вызывает usecase и записывает ответ
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	var body api.TripData

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&body); err != nil {
		writeError(w, r, &domain.ValidationError{
			Field:   "body",
			Message: "malformed JSON body or unknown fields",
		})
		return
	}

	trip := &model.Trip{
		UserID:         body.UserId,
		DriverID:       body.DriverId,
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	created, err := h.usecase.Execute(ctx, trip)
	if err != nil {
		writeError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+created.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPITrip(created))
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, problem := domainerror.Problem(err)
	problem.Instance = &r.URL.Path

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
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
