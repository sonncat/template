//go:build integration

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sonncat/template/internal/domain"
	"github.com/sonncat/template/internal/model"
	"github.com/sonncat/template/internal/repository"
	"github.com/sonncat/template/internal/txmanager"
)

func TestFinishTrip(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := repository.NewTripsRepository(pool)
	tx := txmanager.NewPgxTxManager(pool)

	trip := &model.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.9290,
		EndLongitude:   30.3626,
		Price:          1450,
		Status:         "active",
		StartedAt:      time.Now().UTC().Add(-time.Minute),
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		if _, err := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE id = $1", trip.ID); err != nil {
			t.Errorf("cleanup trip: %v", err)
		}
	})

	err = tx.Do(ctx, func(txCtx context.Context) error {
		return repo.Create(txCtx, trip)
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	// PostgreSQL хранит время с точностью до микросекунд
	finishedAt := time.Now().UTC().Truncate(time.Microsecond)

	var finished *model.Trip
	err = tx.Do(ctx, func(txCtx context.Context) error {
		var finishErr error
		finished, finishErr = repo.Finish(txCtx, trip.ID, finishedAt)
		return finishErr
	})
	if err != nil {
		t.Fatalf("finish trip: %v", err)
	}
	if finished.Status != "completed" {
		t.Errorf("status = %q, want completed", finished.Status)
	}
	if finished.FinishedAt == nil {
		t.Fatal("finished_at is nil")
	}
	if !finished.FinishedAt.Equal(finishedAt) {
		t.Errorf("finished_at = %v, want %v", *finished.FinishedAt, finishedAt)
	}

	err = tx.Do(ctx, func(txCtx context.Context) error {
		_, finishErr := repo.Finish(txCtx, trip.ID, finishedAt)
		return finishErr
	})
	if !errors.Is(err, domain.ErrTripCompleted) {
		t.Errorf("second finish error = %v, want ErrTripCompleted", err)
	}
}
