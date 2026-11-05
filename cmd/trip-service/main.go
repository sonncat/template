package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
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

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		log.Fatalf("invalid LOG_LEVEL: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})).With("service", "trip-service")

	telemetryProviders, err := telemetry.New(context.Background())
	if err != nil {
		logger.Error("initialize telemetry", "error", err)
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := telemetryProviders.Shutdown(ctx); err != nil {
			logger.Error("shutdown telemetry", "error", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DB.ConnectTimeout)
	pool, err := db.NewPool(ctx, cfg.DB, telemetryProviders.Tracer)
	cancel()
	if err != nil {
		logger.Error("connect to database", "error", err)
		return
	}
	defer pool.Close()

	txManager := txmanager.NewPgxTxManager(pool)
	tripsRepo := repository.NewTripsRepository(pool)

	createTrip := createtrip.New(tripsRepo, txManager)
	getTrip := gettrip.New(tripsRepo)
	finishTrip := finishtrip.New(tripsRepo, txManager)

	h := httptransport.NewHandler(
		pool,
		createTrip,
		getTrip,
		finishTrip,
		cfg.DB.QueryTimeout,
	)

	r := chi.NewRouter()
	r.Use(middleware.Logging(logger))

	api.HandlerWithOptions(h, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"type":"about:blank","title":"Bad Request","status":400,"detail":"invalid path parameters","code":"invalid_request"}`))
		},
	})

	instrumentedHandler := otelhttp.NewHandler(
		r,
		"HTTP request",
		otelhttp.WithTracerProvider(telemetryProviders.Tracer),
		otelhttp.WithMeterProvider(telemetryProviders.Meter),
	)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           instrumentedHandler,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server listening", "address", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown server", "error", err)
	}
	logger.Info("server stopped")
}
