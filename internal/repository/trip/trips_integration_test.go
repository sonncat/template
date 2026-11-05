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

	"github.com/sonncat/template/internal/infra/postgres/transaction"
	"github.com/sonncat/template/internal/model"
	"github.com/sonncat/template/internal/repository/trip"
)

func TestCreateTripTransactionRollback(t *testing.T) {
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

	// Очистка выполняется в обратном порядке регистрации:
	// сначала удаляем данные теста, затем закрываем пул.
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
		StartedAt:      time.Now().UTC(),
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		if _, err := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE id = $1", trip.ID); err != nil {
			t.Errorf("cleanup trip: %v", err)
		}
	})

	expectedErr := errors.New("force transaction rollback")

	err = tx.Do(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, trip); err != nil {
			return err
		}

		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("transaction error = %v, want %v", err, expectedErr)
	}

	var tripsCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM trips WHERE id = $1", trip.ID).Scan(&tripsCount)
	if err != nil {
		t.Fatalf("count trips: %v", err)
	}
	if tripsCount != 0 {
		t.Errorf("trips count = %d, want 0", tripsCount)
	}

	var historyCount int
	err = pool.QueryRow(
		ctx,
		"SELECT count(*) FROM trip_status_history WHERE trip_id = $1",
		trip.ID,
	).Scan(&historyCount)
	if err != nil {
		t.Fatalf("count trip history: %v", err)
	}
	if historyCount != 0 {
		t.Errorf("history count = %d, want 0", historyCount)
	}
}
