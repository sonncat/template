package create

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/http/domainerror"
	"github.com/sonncat/template/internal/model"
)

const (
	requiredMessage = "is required"
	bodyField       = "body"
)

// Handler обрабатывает создание поездки
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
	var raw json.RawMessage

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&raw); err != nil {
		writeError(w, r, &domain.ValidationError{
			Field:   bodyField,
			Message: "malformed JSON body",
		})
		return
	}

	// В теле должен быть ровно один JSON-объект
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, r, &domain.ValidationError{
			Field:   bodyField,
			Message: "request must contain one JSON object",
		})
		return
	}

	// Значения разбираются в тип, сгенерированный из OpenAPI
	var body api.TripData
	bodyDecoder := json.NewDecoder(bytes.NewReader(raw))
	bodyDecoder.DisallowUnknownFields()

	if err := bodyDecoder.Decode(&body); err != nil {
		writeError(w, r, &domain.ValidationError{
			Field:   bodyField,
			Message: "malformed JSON body or unknown fields",
		})
		return
	}

	if err := validateRequiredFields(raw); err != nil {
		writeError(w, r, err)
		return
	}

	newTrip := &model.Trip{
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

	created, err := h.usecase.Execute(ctx, newTrip)
	if err != nil {
		writeError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+created.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPITrip(created))
}

// validateRequiredFields отличает отсутствующее поле от переданного нуля
func validateRequiredFields(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return &domain.ValidationError{Field: bodyField, Message: "must be an object"}
	}

	for _, name := range []string{"user_id", "driver_id", "price"} {
		if missingOrNull(fields[name]) {
			return &domain.ValidationError{Field: name, Message: requiredMessage}
		}
	}

	for _, name := range []string{"start_point", "end_point"} {
		if err := validatePointFields(name, fields[name]); err != nil {
			return err
		}
	}

	return nil
}

func validatePointFields(name string, raw json.RawMessage) error {
	if missingOrNull(raw) {
		return &domain.ValidationError{Field: name, Message: requiredMessage}
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return &domain.ValidationError{Field: name, Message: "must be an object"}
	}

	for _, coordinate := range []string{"latitude", "longitude"} {
		if missingOrNull(fields[coordinate]) {
			return &domain.ValidationError{
				Field:   name + "." + coordinate,
				Message: requiredMessage,
			}
		}
	}

	return nil
}

func missingOrNull(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, problem := domainerror.ProblemForRequest(r, err)
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
