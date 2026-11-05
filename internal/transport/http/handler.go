package http

import (
	"context"
	"net/http"
	"time"

	"github.com/sonncat/template/api"
	createtransport "github.com/sonncat/template/internal/http/trip/create"
	finishtransport "github.com/sonncat/template/internal/http/trip/finish"
	gettransport "github.com/sonncat/template/internal/http/trip/get"
)

// Handler связывает сгенерированный OpenAPI-интерфейс с обработчиками действий
type Handler struct {
	database     databasePinger
	createTrip   *createtransport.Handler
	getTrip      *gettransport.Handler
	finishTrip   *finishtransport.Handler
	queryTimeout time.Duration
}

// NewHandler создаёт OpenAPI-адаптер и обработчики действий
func NewHandler(
	database databasePinger,
	createTrip createTripUseCase,
	getTrip getTripUseCase,
	finishTrip finishTripUseCase,
	queryTimeout time.Duration,
) *Handler {
	return &Handler{
		database:     database,
		createTrip:   createtransport.New(createTrip, queryTimeout),
		getTrip:      gettransport.New(getTrip, queryTimeout),
		finishTrip:   finishtransport.New(finishTrip, queryTimeout),
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	h.createTrip.Handle(w, r)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.getTrip.Handle(w, r, tripID)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.finishTrip.Handle(w, r, tripID)
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
