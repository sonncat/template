package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sonncat/template/internal/config"
	"github.com/sonncat/template/internal/container"
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

	app, err := container.New(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("initialize application", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.Handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("server listening", "address", cfg.HTTPAddr)
		serverErrors <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	exitCode := 0

	select {
	case <-stop:
		logger.Info("shutdown signal received")
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			exitCode = 1
		}
	}

	signal.Stop(stop)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)

	if err := shutdownHTTP(shutdownCtx, srv); err != nil {
		logger.Error("graceful shutdown failed; server closed", "error", err)
		exitCode = 1
	}

	if err := app.Close(shutdownCtx); err != nil {
		logger.Error("close application", "error", err)
	}

	shutdownCancel()
	logger.Info("server stopped")

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// shutdownHTTP ожидает завершения запросов и принудительно закрывает сервер
// при ошибке или истечении срока ожидания
func shutdownHTTP(ctx context.Context, srv *http.Server) error {
	err := srv.Shutdown(ctx)
	if err == nil {
		return nil
	}

	if closeErr := srv.Close(); closeErr != nil {
		return errors.Join(
			fmt.Errorf("graceful shutdown: %w", err),
			fmt.Errorf("force close server: %w", closeErr),
		)
	}

	return fmt.Errorf("graceful shutdown: %w", err)
}
