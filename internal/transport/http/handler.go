package http

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sonncat/template/api"
)

type Handler struct {
	pool          *pgxpool.Pool
	queryTimeout  time.Duration
}

func NewHandler(pool *pgxpool.Pool, queryTimeout time.Duration) *Handler {
	return &Handler{
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
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