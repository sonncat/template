package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sonncat/template/api"
	"github.com/sonncat/template/internal/config"
	"github.com/sonncat/template/internal/db"
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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DB.ConnectTimeout)
	pool, err := db.NewPool(ctx, cfg.DB)
	cancel()
	if err != nil {
		log.Fatalf("db: %v", err)
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

	api.HandlerWithOptions(h, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"type":"about:blank","title":"Bad Request","status":400,"detail":"invalid path parameters","code":"invalid_request"}`))
		},
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("stopped")
}
