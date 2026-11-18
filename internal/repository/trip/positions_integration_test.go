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
	domaintrip "github.com/sonncat/template/internal/domain/trip"
	"github.com/sonncat/template/internal/infra/postgres/transaction"
	"github.com/sonncat/template/internal/model"
	"github.com/sonncat/template/internal/repository/trip"
)

func TestSavePositionUpdatesTimeAndRollsBack(t *testing.T) {
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

	startedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	trip := &model.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.9290,
		EndLongitude:   30.3626,
		Price:          1450,
		Status:         domaintrip.StatusActive,
		StartedAt:      startedAt,
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM trips WHERE id = $1",
			trip.ID,
		); err != nil {
			t.Errorf("cleanup trip: %v", err)
		}
	})

	if err := tx.Do(ctx, func(txCtx context.Context) error {
		return repo.Create(txCtx, trip)
	}); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	freshTime := startedAt.Add(30 * time.Minute)
	oldTime := startedAt.Add(10 * time.Minute)

	save := func(position *model.Position) error {
		return tx.Do(ctx, func(txCtx context.Context) error {
			return repo.SavePosition(txCtx, position)
		})
	}

	fresh := &model.Position{
		TripID:     trip.ID,
		Latitude:   59.93,
		Longitude:  30.33,
		RecordedAt: freshTime,
	}
	if err := save(fresh); err != nil {
		t.Fatalf("save fresh position: %v", err)
	}

	old := &model.Position{
		TripID:     trip.ID,
		Latitude:   59.92,
		Longitude:  30.32,
		RecordedAt: oldTime,
	}
	if err := save(old); err != nil {
		t.Fatalf("save old position: %v", err)
	}

	var lastPositionAt time.Time
	if err := pool.QueryRow(
		ctx,
		"SELECT last_position_at FROM trips WHERE id = $1",
		trip.ID,
	).Scan(&lastPositionAt); err != nil {
		t.Fatalf("read last position time: %v", err)
	}

	if !lastPositionAt.Equal(freshTime) {
		t.Errorf(
			"last_position_at = %v, want latest time %v",
			lastPositionAt,
			freshTime,
		)
	}

	// Эта координата новее предыдущих, но нарушает CHECK в таблице.
	// Обновление поездки должно откатиться вместе с неудачной вставкой.
	invalid := &model.Position{
		TripID:     trip.ID,
		Latitude:   91,
		Longitude:  30.33,
		RecordedAt: freshTime.Add(time.Minute),
	}
	if err := save(invalid); err == nil {
		t.Fatal("saving invalid position succeeded, want database error")
	}

	if err := pool.QueryRow(
		ctx,
		"SELECT last_position_at FROM trips WHERE id = $1",
		trip.ID,
	).Scan(&lastPositionAt); err != nil {
		t.Fatalf("read last position time after rollback: %v", err)
	}

	if !lastPositionAt.Equal(freshTime) {
		t.Errorf(
			"last_position_at after rollback = %v, want %v",
			lastPositionAt,
			freshTime,
		)
	}

	positions, err := repo.ListPositions(ctx, trip.ID)
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}

	if len(positions) != 2 {
		t.Fatalf("positions count = %d, want 2", len(positions))
	}

	if !positions[0].RecordedAt.Equal(oldTime) ||
		!positions[1].RecordedAt.Equal(freshTime) {
		t.Errorf("positions are not ordered by recorded_at: %+v", positions)
	}

	if positions[0].ID == positions[1].ID {
		t.Error("saved positions must have different IDs")
	}

	if _, err := repo.ListPositions(ctx, uuid.New()); !errors.Is(err, domain.ErrTripNotFound) {
		t.Errorf("list missing trip error = %v, want ErrTripNotFound", err)
	}
}
