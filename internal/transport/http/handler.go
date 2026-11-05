package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/model"
)

type Handler struct {
	database     databasePinger
	createTrip   createTripUseCase
	getTrip      getTripUseCase
	finishTrip   finishTripUseCase
	queryTimeout time.Duration
}

func NewHandler(
	database databasePinger,
	createTrip createTripUseCase,
	getTrip getTripUseCase,
	finishTrip finishTripUseCase,
	queryTimeout time.Duration,
) *Handler {
	return &Handler{
		database:     database,
		createTrip:   createTrip,
		getTrip:      getTrip,
		finishTrip:   finishTrip,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	var body api.TripData

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&body); err != nil {
		writeValidationError(w, r, "body", "malformed JSON body or unknown fields")
		return
	}

	if body.UserId == uuid.Nil {
		writeValidationError(w, r, "user_id", "is required")
		return
	}

	if body.DriverId == uuid.Nil {
		writeValidationError(w, r, "driver_id", "is required")
		return
	}

	if body.Price <= 0 {
		writeValidationError(w, r, "price", "is required and must be greater than zero")
		return
	}

	if body.StartPoint.Latitude == 0 && body.StartPoint.Longitude == 0 {
		writeValidationError(w, r, "start_point", "is required")
		return
	}

	if body.EndPoint.Latitude == 0 && body.EndPoint.Longitude == 0 {
		writeValidationError(w, r, "end_point", "is required")
		return
	}

	if body.StartPoint.Latitude < -90 || body.StartPoint.Latitude > 90 ||
		body.StartPoint.Longitude < -180 || body.StartPoint.Longitude > 180 {
		writeValidationError(w, r, "start_point", "coordinates are out of range")
		return
	}

	if body.EndPoint.Latitude < -90 || body.EndPoint.Latitude > 90 ||
		body.EndPoint.Longitude < -180 || body.EndPoint.Longitude > 180 {
		writeValidationError(w, r, "end_point", "coordinates are out of range")
		return
	}

	trip := &model.Trip{
		ID:             uuid.New(),
		UserID:         body.UserId,
		DriverID:       body.DriverId,
		StartLatitude:  body.StartPoint.Latitude,
		StartLongitude: body.StartPoint.Longitude,
		EndLatitude:    body.EndPoint.Latitude,
		EndLongitude:   body.EndPoint.Longitude,
		Price:          body.Price,
		Status:         "active",
		StartedAt:      time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	created, err := h.createTrip.Execute(ctx, trip)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+created.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPITrip(created))
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	trip, err := h.getTrip.Execute(ctx, tripID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(trip))
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	finished, err := h.finishTrip.Execute(ctx, tripID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(finished))
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")

	if err := h.database.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func toAPITrip(t *model.Trip) api.Trip {
	return api.Trip{
		Id:         t.ID,
		UserId:     t.UserID,
		DriverId:   t.DriverID,
		StartPoint: api.Coordinates{Latitude: t.StartLatitude, Longitude: t.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: t.EndLatitude, Longitude: t.EndLongitude},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}
