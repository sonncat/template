package position

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/http/domainerror"
	"github.com/sonncat/template/internal/model"
)

type Handler struct {
	save    saveUseCase
	list    listUseCase
	timeout time.Duration
}

func New(save saveUseCase, list listUseCase, timeout time.Duration) *Handler {
	return &Handler{
		save:    save,
		list:    list,
		timeout: timeout,
	}
}

// requestBody использует указатели, чтобы отличить отсутствующее поле
// от явно переданной нулевой координаты.
type requestBody struct {
	Coordinates *struct {
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	} `json:"coordinates"`
	RecordedAt *time.Time `json:"recorded_at"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	var body requestBody

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&body); err != nil {
		writeError(w, r, &domain.ValidationError{
			Field:   "body",
			Message: "malformed JSON body or unknown fields",
		})
		return
	}

	// Проверяем, что после первого объекта нет второго значения JSON.
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, r, &domain.ValidationError{
			Field:   "body",
			Message: "request must contain one JSON object",
		})
		return
	}

	if body.Coordinates == nil ||
		body.Coordinates.Latitude == nil ||
		body.Coordinates.Longitude == nil {
		writeError(w, r, &domain.ValidationError{
			Field:   "coordinates",
			Message: "latitude and longitude are required",
		})
		return
	}

	if body.RecordedAt == nil {
		writeError(w, r, &domain.ValidationError{
			Field:   "recorded_at",
			Message: "is required",
		})
		return
	}

	newPosition := &model.Position{
		TripID:     tripID,
		Latitude:   *body.Coordinates.Latitude,
		Longitude:  *body.Coordinates.Longitude,
		RecordedAt: *body.RecordedAt,
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	saved, err := h.save.Execute(ctx, newPosition)
	if err != nil {
		writeError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPIPosition(saved))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	positions, err := h.list.Execute(ctx, tripID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	result := make([]api.TripPosition, 0, len(positions))
	for i := range positions {
		result = append(result, toAPIPosition(&positions[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.TripPositionsResponse{
		Positions: result,
	})
}

func toAPIPosition(position *model.Position) api.TripPosition {
	return api.TripPosition{
		Id:     position.ID,
		TripId: position.TripID,
		Coordinates: api.Coordinates{
			Latitude:  position.Latitude,
			Longitude: position.Longitude,
		},
		RecordedAt: position.RecordedAt,
	}
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, problem := domainerror.ProblemForRequest(r, err)
	problem.Instance = &r.URL.Path

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
