package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/config"
	"github.com/sonncat/template/internal/gateway/push"
	"github.com/sonncat/template/internal/http/middleware"
	positiontransport "github.com/sonncat/template/internal/http/position"
	"github.com/sonncat/template/internal/infra/postgres"
	"github.com/sonncat/template/internal/infra/postgres/transaction"
	"github.com/sonncat/template/internal/infra/telemetry"
	"github.com/sonncat/template/internal/repository/trip"
	httptransport "github.com/sonncat/template/internal/transport/http"
	listpositions "github.com/sonncat/template/internal/usecase/position/list"
	saveposition "github.com/sonncat/template/internal/usecase/position/save"
	createtrip "github.com/sonncat/template/internal/usecase/trip/create"
	finishtrip "github.com/sonncat/template/internal/usecase/trip/finish"
	gettrip "github.com/sonncat/template/internal/usecase/trip/get"
	"github.com/sonncat/template/internal/worker/positions"
)

// Container хранит собранный HTTP-обработчик и ресурсы приложения
type Container struct {
	Handler   http.Handler
	pool      *pgxpool.Pool
	telemetry *telemetry.Providers

	scannerCancel context.CancelFunc
	scannerDone   chan struct{}
	workers       *positions.WorkerPool
	pushClient    *push.Client
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

	err = telemetry.RegisterActiveTrips(
		telemetryProviders.Meter.Meter("trip-service/trips"),
		tripsRepo,
		cfg.DB.QueryTimeout,
		logger,
	)
	if err != nil {
		_ = telemetryProviders.Shutdown(ctx)
		pool.Close()
		return nil, fmt.Errorf("initialize active trips metric: %w", err)
	}

	tripEvents, err := telemetry.NewTripEvents(
		telemetryProviders.Meter.Meter("trip-service/trips"),
		logger,
	)
	if err != nil {
		_ = telemetryProviders.Shutdown(ctx)
		pool.Close()
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

	scanner := positions.NewScanner(
		tripsRepo,
		cfg.PositionStaleAfter,
		int(cfg.PositionQueueSize),
		logger,
	)

	pushClient := push.New(cfg.PushHTTPURL, cfg.PushRequestTimeout)
	workers := positions.NewWorkerPool(
		scanner,
		pushClient,
		cfg.PushRequestTimeout,
		logger,
	)

	positionHandler := positiontransport.New(
		saveposition.New(tripsRepo, txManager),
		listpositions.New(tripsRepo),
		cfg.DB.QueryTimeout,
	)
	positionMetrics, err := telemetry.NewPositionMetrics(
		telemetryProviders.Meter.Meter("trip-service/positions"),
		scanner.QueueSize,
		workers.ActiveWorkers,
	)
	if err != nil {
		_ = telemetryProviders.Shutdown(ctx)
		pool.Close()
		pushClient.Close()
		return nil, fmt.Errorf("initialize position metrics: %w", err)
	}

	scanner.SetObservability(
		positionMetrics,
		telemetryProviders.Tracer.Tracer("trip-service/positions"),
	)

	positionHandler.SetTaskCanceller(scanner)
	positionHandler.SetMetrics(positionMetrics)
	handler.SetPositionHandler(positionHandler)

	router := chi.NewRouter()

	activeRequests, err := middleware.ActiveRequests(
		telemetryProviders.Meter.Meter("trip-service/http"),
		router,
	)
	if err != nil {
		_ = telemetryProviders.Shutdown(ctx)
		pool.Close()
		return nil, fmt.Errorf("initialize HTTP metrics: %w", err)
	}

	router.Use(middleware.Logging(logger))
	router.Use(activeRequests)
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

	scannerCtx, scannerCancel := context.WithCancel(context.Background())
	scannerDone := make(chan struct{})

	workers.Start(int(cfg.PositionWorkers))

	go func() {
		defer close(scannerDone)

		ticker := time.NewTicker(cfg.PositionCheckInterval)
		defer ticker.Stop()

		scanner.Run(scannerCtx, ticker.C)
	}()

	return &Container{
		Handler:       middleware.Recover(logger)(tracedHandler),
		pool:          pool,
		telemetry:     telemetryProviders,
		scannerCancel: scannerCancel,
		scannerDone:   scannerDone,
		workers:       workers,
		pushClient:    pushClient,
	}, nil
}

// Close останавливает телеметрию и закрывает пул в пределах срока контекста
func (c *Container) Close(ctx context.Context) error {
	var result error

	c.scannerCancel()

	select {
	case <-c.scannerDone:
		if err := c.workers.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("stop position workers: %w", err))
		}

	case <-ctx.Done():
		// Очередь не закрываем, пока сканер ещё может в неё записывать
		c.workers.Cancel()
		result = errors.Join(result, fmt.Errorf("stop position scanner: %w", ctx.Err()))
	}

	c.pushClient.Close()

	if err := c.telemetry.Shutdown(ctx); err != nil {
		result = errors.Join(result, fmt.Errorf("shutdown telemetry: %w", err))
	}

	poolClosed := make(chan struct{})

	go func() {
		c.pool.Close()
		close(poolClosed)
	}()

	select {
	case <-poolClosed:
		return result
	case <-ctx.Done():
		return errors.Join(result, fmt.Errorf("close database pool: %w", ctx.Err()))
	}
}
