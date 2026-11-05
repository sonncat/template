package container

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/config"
	"github.com/sonncat/template/internal/db"
	"github.com/sonncat/template/internal/http/middleware"
	"github.com/sonncat/template/internal/infra/telemetry"
	"github.com/sonncat/template/internal/repository"
	httptransport "github.com/sonncat/template/internal/transport/http"
	"github.com/sonncat/template/internal/txmanager"
	createtrip "github.com/sonncat/template/internal/usecase/trip/create"
	finishtrip "github.com/sonncat/template/internal/usecase/trip/finish"
	gettrip "github.com/sonncat/template/internal/usecase/trip/get"
)

// Container хранит собранный HTTP-обработчик и ресурсы приложения
type Container struct {
	Handler   http.Handler
	pool      *pgxpool.Pool
	telemetry *telemetry.Providers
}

// New создаёт зависимости приложения и связывает их между собой
func New(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*Container, error) {
	telemetryProviders, err := telemetry.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize telemetry: %w", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, cfg.DB.ConnectTimeout)
	pool, err := db.NewPool(
		connectCtx,
		cfg.DB,
		telemetryProviders.Tracer,
		telemetryProviders.Meter,
	)
	cancel()
	if err != nil {
		_ = telemetryProviders.Shutdown(ctx)
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	txManager := txmanager.NewPgxTxManager(pool)
	tripsRepo := repository.NewTripsRepository(pool)

	tripEvents, err := telemetry.NewTripEvents(
		telemetryProviders.Meter.Meter("trip-service/trips"),
		logger,
	)
	if err != nil {
		pool.Close()
		_ = telemetryProviders.Shutdown(ctx)
		return nil, fmt.Errorf("initialize trip metrics: %w", err)
	}

	createTrip := tripEvents.ObserveCreate(createtrip.New(tripsRepo, txManager))
	getTrip := gettrip.New(tripsRepo)
	finishTrip := tripEvents.ObserveFinish(finishtrip.New(tripsRepo, txManager))

	handler := httptransport.NewHandler(
		pool,
		createTrip,
		getTrip,
		finishTrip,
		cfg.DB.QueryTimeout,
	)

	router := chi.NewRouter()

	activeRequests, err := middleware.ActiveRequests(
		telemetryProviders.Meter.Meter("trip-service/http"),
		router,
	)
	if err != nil {
		pool.Close()
		_ = telemetryProviders.Shutdown(ctx)
		return nil, fmt.Errorf("initialize HTTP metrics: %w", err)
	}

	router.Use(middleware.Logging(logger))
	router.Use(activeRequests)

	// Внутренний Recover позволяет Logging и Metrics увидеть ответ 500
	router.Use(middleware.Recover(logger))

	api.HandlerWithOptions(handler, api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"type":"about:blank","title":"Bad Request","status":400,"detail":"invalid path parameters","code":"invalid_request"}`))
		},
	})

	tracedHandler := otelhttp.NewHandler(
		router,
		"HTTP request",
		otelhttp.WithTracerProvider(telemetryProviders.Tracer),
		otelhttp.WithMeterProvider(telemetryProviders.Meter),
	)

	return &Container{
		Handler:   middleware.Recover(logger)(tracedHandler),
		pool:      pool,
		telemetry: telemetryProviders,
	}, nil
}

// Close закрывает пул базы и отправляет оставшуюся телеметрию
func (c *Container) Close(ctx context.Context) error {
	c.pool.Close()

	if err := c.telemetry.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown telemetry: %w", err)
	}

	return nil
}
