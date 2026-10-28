package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/repository"
	"github.com/sonncat/template/internal/txmanager"
)

type Handler struct {
	pool         *pgxpool.Pool
	txManager    txmanager.TxManager
	trips        *repository.TripsRepository
	queryTimeout time.Duration
}

func NewHandler(
	pool *pgxpool.Pool,
	txManager txmanager.TxManager,
	trips *repository.TripsRepository,
	queryTimeout time.Duration,
) *Handler {
	return &Handler{
		pool:         pool,
		txManager:    txManager,
		trips:        trips,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var body api.TripData
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}

	// Валидация: user_id и driver_id непустые UUID, координаты в диапазонах, price >= 0.
	if body.UserId == (openapi_types.UUID{}) || body.DriverId == (openapi_types.UUID{}) {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "user_id and driver_id are required")
		return
	}
	if body.StartPoint.Latitude < -90 || body.StartPoint.Latitude > 90 ||
		body.EndPoint.Latitude < -90 || body.EndPoint.Latitude > 90 ||
		body.StartPoint.Longitude < -180 || body.StartPoint.Longitude > 180 ||
		body.EndPoint.Longitude < -180 || body.EndPoint.Longitude > 180 {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "coordinates out of range")
		return
	}
	if body.Price < 0 {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "price must be >= 0")
		return
	}

	trip := &repository.Trip{
		ID:             uuid.New(),
		UserID:         uuid.UUID(body.UserId),
		DriverID:       uuid.UUID(body.DriverId),
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

	err := h.txManager.Do(ctx, func(ctx context.Context) error {
		return h.trips.Create(ctx, trip)
	})
	if err != nil {
		if errors.Is(err, domain.ErrDriverBusy) {
			writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver already has an active trip")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	// Перечитываем поездку, чтобы отдать актуальные started_at/created_at.
	created, err := h.trips.GetByID(ctx, trip.ID)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+created.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPITrip(created))
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	t, err := h.trips.GetByID(ctx, uuid.UUID(tripId))
	if err != nil {
		if errors.Is(err, domain.ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip was not found")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(t))
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	var finished *repository.Trip
	err := h.txManager.Do(ctx, func(ctx context.Context) error {
		t, err := h.trips.Finish(ctx, uuid.UUID(tripId))
		if err != nil {
			return err
		}
		finished = t
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTripNotFound):
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip was not found")
		case errors.Is(err, domain.ErrTripCompleted):
			writeProblem(w, r, http.StatusConflict, "trip_completed", "Operation is not allowed for a completed trip")
		default:
			writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(finished))
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	if err := h.pool.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unavailable"}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func toAPITrip(t *repository.Trip) api.Trip {
	status := api.TripStatus(t.Status)
	return api.Trip{
		Id:             openapi_types.UUID(t.ID),
		UserId:         openapi_types.UUID(t.UserID),
		DriverId:       openapi_types.UUID(t.DriverID),
		StartPoint:     api.Coordinates{Latitude: t.StartLatitude, Longitude: t.StartLongitude},
		EndPoint:       api.Coordinates{Latitude: t.EndLatitude, Longitude: t.EndLongitude},
		Price:          t.Price,
		Status:         status,
		StartedAt:      t.StartedAt,
		FinishedAt:     t.FinishedAt,
	}
}